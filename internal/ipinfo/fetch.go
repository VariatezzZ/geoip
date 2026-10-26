package ipinfo

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/VariatezzZ/geoip/internal/atomicfile"
	"github.com/VariatezzZ/geoip/internal/config"
)

type Getter interface {
	Get(context.Context, string) ([]byte, error)
}

type Result struct {
	Mirror string
	Data   []byte
}

func Fetch(ctx context.Context, cfg config.IPInfoConfig, getter Getter) (Result, error) {
	if getter == nil {
		return Result{}, errors.New("download client is nil")
	}
	if len(cfg.Mirrors) == 0 {
		return Result{}, errors.New("no IPinfo mirrors configured")
	}

	failures := make([]error, 0, len(cfg.Mirrors))
	for _, mirror := range cfg.Mirrors {
		data, err := fetchMirror(ctx, mirror, getter)
		if err != nil {
			failures = append(failures, fmt.Errorf("mirror %s: %w", mirror.Name, err))
			continue
		}
		return Result{Mirror: mirror.Name, Data: data}, nil
	}

	return Result{}, fmt.Errorf("all IPinfo mirrors failed: %w", errors.Join(failures...))
}

func Save(filename string, data []byte) error {
	if err := atomicfile.Write(filename, data, 0o644); err != nil {
		return fmt.Errorf("save IPinfo database: %w", err)
	}
	return nil
}

func fetchMirror(ctx context.Context, mirror config.Mirror, getter Getter) ([]byte, error) {
	var checksums []byte
	if mirror.Checksums != "" {
		data, err := getter.Get(ctx, mirror.Checksums)
		if err != nil {
			return nil, fmt.Errorf("download checksums: %w", err)
		}
		checksums = data
	}

	data, err := getter.Get(ctx, mirror.URL)
	if err != nil {
		return nil, fmt.Errorf("download database: %w", err)
	}

	if len(checksums) > 0 {
		asset, err := assetName(mirror.URL)
		if err != nil {
			return nil, err
		}
		if err := verifyChecksum(data, checksums, asset); err != nil {
			return nil, err
		}
	}
	if err := ValidateBytes(data); err != nil {
		return nil, err
	}
	return data, nil
}

func assetName(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse mirror URL: %w", err)
	}
	name := path.Base(parsed.Path)
	if name == "" || name == "." || name == "/" {
		return "", errors.New("mirror URL does not contain an asset name")
	}
	return name, nil
}

func verifyChecksum(data []byte, checksums []byte, asset string) error {
	expected, found, err := findChecksum(checksums, asset)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("checksum for %s is missing", asset)
	}

	actual := sha256.Sum256(data)
	actualText := hex.EncodeToString(actual[:])
	if !strings.EqualFold(actualText, expected) {
		return fmt.Errorf("checksum mismatch for %s", asset)
	}
	return nil
}

func findChecksum(checksums []byte, asset string) (string, bool, error) {
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	scanner.Buffer(make([]byte, 16*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		name = strings.TrimPrefix(name, "./")
		if path.Base(name) != asset {
			continue
		}

		value := strings.ToLower(fields[0])
		if len(value) != sha256.Size*2 {
			return "", false, fmt.Errorf("checksum for %s is malformed", asset)
		}
		if _, err := hex.DecodeString(value); err != nil {
			return "", false, fmt.Errorf("checksum for %s is malformed: %w", asset, err)
		}
		return value, true, nil
	}
	if err := scanner.Err(); err != nil {
		return "", false, fmt.Errorf("read checksums: %w", err)
	}
	return "", false, nil
}
