package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"sort"
	"strings"

	"github.com/VariatezzZ/geoip/internal/atomicfile"
	"github.com/VariatezzZ/geoip/internal/config"
	"github.com/VariatezzZ/geoip/internal/ipinfo"
	"github.com/VariatezzZ/geoip/internal/ipset"
	"github.com/VariatezzZ/geoip/internal/textparse"
	"github.com/VariatezzZ/geoip/internal/xraydat"
)

type Getter interface {
	Get(context.Context, string) ([]byte, error)
}

type Report struct {
	Categories map[string]int
	Outputs    []Output
}

type Output struct {
	Path   string
	SHA256 string
	Size   int
}

func Run(ctx context.Context, cfg *config.Config, getter Getter) (Report, error) {
	report := Report{Categories: make(map[string]int, 2)}
	if cfg == nil {
		return report, errors.New("configuration is nil")
	}
	if err := cfg.Validate(); err != nil {
		return report, fmt.Errorf("validate configuration: %w", err)
	}

	entries := make([]xraydat.Entry, 0, len(cfg.Categories))
	for _, category := range cfg.Categories {
		prefixes, err := collectCategory(ctx, category, getter)
		if err != nil {
			return report, fmt.Errorf("build category %s: %w", category.Name, err)
		}

		aggregated, err := ipset.Aggregate(prefixes)
		if err != nil {
			return report, fmt.Errorf("aggregate category %s: %w", category.Name, err)
		}
		if len(aggregated) == 0 {
			return report, fmt.Errorf("category %s has no prefixes", category.Name)
		}
		if category.Name == config.CategoryCN {
			if !ipset.HasIPv4(aggregated) {
				return report, errors.New("category cn has no IPv4 prefixes")
			}
			if !ipset.HasIPv6(aggregated) {
				return report, errors.New("category cn has no IPv6 prefixes")
			}
		}

		code := strings.ToUpper(category.Name)
		report.Categories[code] = len(aggregated)
		entries = append(entries, xraydat.Entry{
			Code:     code,
			Prefixes: aggregated,
		})
	}

	if len(report.Categories) != 2 {
		return report, fmt.Errorf("expected two categories, got %d", len(report.Categories))
	}
	if _, ok := report.Categories["CN"]; !ok {
		return report, errors.New("category CN is missing")
	}
	if _, ok := report.Categories["PRIVATE"]; !ok {
		return report, errors.New("category PRIVATE is missing")
	}

	encoded, err := xraydat.Encode(entries)
	if err != nil {
		return report, fmt.Errorf("encode DAT: %w", err)
	}
	if err := validateEncoded(encoded); err != nil {
		return report, fmt.Errorf("validate DAT: %w", err)
	}

	for _, filename := range cfg.Outputs {
		if err := atomicfile.Write(filename, encoded, 0o644); err != nil {
			return report, fmt.Errorf("write output %s: %w", filename, err)
		}
	}

	outputs, err := verifyOutputs(cfg.Outputs, encoded)
	if err != nil {
		return report, err
	}
	report.Outputs = outputs
	return report, nil
}

func collectCategory(ctx context.Context, category config.Category, getter Getter) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, 65536)

	for index, source := range category.Sources {
		collected, err := collectSource(ctx, source, getter)
		if err != nil {
			return nil, fmt.Errorf("source %d (%s): %w", index+1, source.Type, err)
		}
		if len(collected) == 0 {
			return nil, fmt.Errorf("source %d (%s) produced no prefixes", index+1, source.Type)
		}
		prefixes = append(prefixes, collected...)
	}
	return prefixes, nil
}

func collectSource(ctx context.Context, source config.Source, getter Getter) ([]netip.Prefix, error) {
	switch source.Type {
	case config.SourceIPInfo:
		prefixes, err := ipinfo.LoadPrefixes(
			source.File,
			source.Country,
			source.AllowsIPv4(),
			source.AllowsIPv6(),
		)
		if err != nil {
			return nil, err
		}
		return prefixes, nil
	case config.SourceText:
		data, err := readText(ctx, source, getter)
		if err != nil {
			return nil, err
		}
		prefixes, err := textparse.Parse(data)
		if err != nil {
			return nil, err
		}
		return ipset.FilterFamily(prefixes, source.AllowsIPv4(), source.AllowsIPv6()), nil
	default:
		return nil, fmt.Errorf("unsupported source type %q", source.Type)
	}
}

func readText(ctx context.Context, source config.Source, getter Getter) ([]byte, error) {
	if source.File != "" {
		data, err := os.ReadFile(source.File)
		if err != nil {
			return nil, fmt.Errorf("read text source %s: %w", source.File, err)
		}
		return data, nil
	}
	if getter == nil {
		return nil, errors.New("download client is nil")
	}
	data, err := getter.Get(ctx, source.URL)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func validateEncoded(data []byte) error {
	entries, err := xraydat.Decode(data)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return fmt.Errorf("expected two entries, got %d", len(entries))
	}

	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Code < entries[right].Code
	})
	if entries[0].Code != "CN" || entries[1].Code != "PRIVATE" {
		return fmt.Errorf("unexpected entry codes %q and %q", entries[0].Code, entries[1].Code)
	}
	for _, entry := range entries {
		if len(entry.Prefixes) == 0 {
			return fmt.Errorf("entry %s has no prefixes", entry.Code)
		}
	}
	return nil
}

func verifyOutputs(filenames []string, encoded []byte) ([]Output, error) {
	outputs := make([]Output, 0, len(filenames))
	var first []byte

	for index, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read output %s: %w", filename, err)
		}
		if !bytes.Equal(data, encoded) {
			return nil, fmt.Errorf("output %s differs from the encoded DAT", filename)
		}
		if index == 0 {
			first = data
		} else if !bytes.Equal(data, first) {
			return nil, fmt.Errorf("output %s is not identical to %s", filename, filenames[0])
		}
		if err := validateEncoded(data); err != nil {
			return nil, fmt.Errorf("validate output %s: %w", filename, err)
		}

		sum := sha256.Sum256(data)
		outputs = append(outputs, Output{
			Path:   filename,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   len(data),
		})
	}
	return outputs, nil
}
