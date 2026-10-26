package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	CategoryCN      = "cn"
	CategoryPrivate = "private"

	SourceIPInfo = "ipinfo"
	SourceText   = "text"
)

type Config struct {
	IPInfo     IPInfoConfig `yaml:"ipinfo"`
	Categories []Category   `yaml:"categories"`
	Outputs    []string     `yaml:"outputs"`
}

type IPInfoConfig struct {
	Output  string   `yaml:"output"`
	Mirrors []Mirror `yaml:"mirrors"`
}

type Mirror struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	Checksums string `yaml:"checksums"`
}

type Category struct {
	Name    string   `yaml:"name"`
	Sources []Source `yaml:"sources"`
}

type Source struct {
	Type    string   `yaml:"type"`
	File    string   `yaml:"file"`
	URL     string   `yaml:"url"`
	Country string   `yaml:"country"`
	IPTypes []string `yaml:"ip_types"`
}

func (s Source) AllowsIPv4() bool {
	if len(s.IPTypes) == 0 {
		return true
	}
	for _, value := range s.IPTypes {
		if value == "ipv4" {
			return true
		}
	}
	return false
}

func (s Source) AllowsIPv6() bool {
	if len(s.IPTypes) == 0 {
		return true
	}
	for _, value := range s.IPTypes {
		if value == "ipv6" {
			return true
		}
	}
	return false
}

func Load(filename string) (*Config, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, errors.New("configuration path is empty")
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open configuration %s: %w", filename, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("configuration %s is empty", filename)
		}
		return nil, fmt.Errorf("decode configuration %s: %w", filename, err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("configuration %s contains multiple YAML documents", filename)
		}
		return nil, fmt.Errorf("decode configuration %s: %w", filename, err)
	}

	if err := cfg.normalize(filename); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate configuration %s: %w", filename, err)
	}
	return &cfg, nil
}

func (c *Config) normalize(filename string) error {
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("resolve configuration path %s: %w", filename, err)
	}
	base := filepath.Dir(absolute)

	c.IPInfo.Output = resolvePath(base, c.IPInfo.Output)
	for index := range c.Outputs {
		c.Outputs[index] = resolvePath(base, c.Outputs[index])
	}

	for categoryIndex := range c.Categories {
		category := &c.Categories[categoryIndex]
		category.Name = strings.ToLower(strings.TrimSpace(category.Name))

		for sourceIndex := range category.Sources {
			source := &category.Sources[sourceIndex]
			source.Type = strings.ToLower(strings.TrimSpace(source.Type))
			source.File = resolvePath(base, source.File)
			source.URL = strings.TrimSpace(source.URL)
			source.Country = strings.ToUpper(strings.TrimSpace(source.Country))

			types, err := normalizeIPTypes(source.IPTypes)
			if err != nil {
				return fmt.Errorf("category %s source %d: %w", category.Name, sourceIndex+1, err)
			}
			source.IPTypes = types
		}
	}

	for index := range c.IPInfo.Mirrors {
		mirror := &c.IPInfo.Mirrors[index]
		mirror.Name = strings.TrimSpace(mirror.Name)
		mirror.URL = strings.TrimSpace(mirror.URL)
		mirror.Checksums = strings.TrimSpace(mirror.Checksums)
	}
	return nil
}

func resolvePath(base string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}

func normalizeIPTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "ipv4", "ipv6":
		default:
			return nil, fmt.Errorf("unsupported ip type %q", value)
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result, nil
}

func (c *Config) Validate() error {
	if c.IPInfo.Output == "" {
		return errors.New("ipinfo.output is required")
	}
	if len(c.IPInfo.Mirrors) == 0 {
		return errors.New("at least one ipinfo mirror is required")
	}

	mirrorNames := make(map[string]bool, len(c.IPInfo.Mirrors))
	for index, mirror := range c.IPInfo.Mirrors {
		if mirror.Name == "" {
			return fmt.Errorf("ipinfo mirror %d: name is required", index+1)
		}
		if mirrorNames[mirror.Name] {
			return fmt.Errorf("ipinfo mirror %d: duplicate name %q", index+1, mirror.Name)
		}
		mirrorNames[mirror.Name] = true

		if err := validateHTTPURL(mirror.URL); err != nil {
			return fmt.Errorf("ipinfo mirror %s: %w", mirror.Name, err)
		}
		if mirror.Checksums != "" {
			if err := validateHTTPURL(mirror.Checksums); err != nil {
				return fmt.Errorf("ipinfo mirror %s checksums: %w", mirror.Name, err)
			}
		}
	}
	if c.IPInfo.Mirrors[0].Checksums == "" {
		return errors.New("the primary ipinfo mirror must provide a checksums URL")
	}

	if len(c.Categories) != 2 {
		return fmt.Errorf("exactly two categories are required, got %d", len(c.Categories))
	}

	categoryNames := make(map[string]bool, len(c.Categories))
	for index, category := range c.Categories {
		switch category.Name {
		case CategoryCN, CategoryPrivate:
		default:
			return fmt.Errorf("category %d: unsupported name %q", index+1, category.Name)
		}
		if categoryNames[category.Name] {
			return fmt.Errorf("category %d: duplicate name %q", index+1, category.Name)
		}
		categoryNames[category.Name] = true

		if len(category.Sources) == 0 {
			return fmt.Errorf("category %s: at least one source is required", category.Name)
		}

		for sourceIndex, source := range category.Sources {
			if err := validateSource(source); err != nil {
				return fmt.Errorf("category %s source %d: %w", category.Name, sourceIndex+1, err)
			}
		}
	}
	if !categoryNames[CategoryCN] {
		return fmt.Errorf("category %s is required", CategoryCN)
	}
	if !categoryNames[CategoryPrivate] {
		return fmt.Errorf("category %s is required", CategoryPrivate)
	}

	if len(c.Outputs) != 2 {
		return fmt.Errorf("exactly two outputs are required, got %d", len(c.Outputs))
	}
	outputNames := make(map[string]bool, len(c.Outputs))
	for index, output := range c.Outputs {
		if output == "" {
			return fmt.Errorf("output %d: path is required", index+1)
		}
		cleaned := filepath.Clean(output)
		if outputNames[cleaned] {
			return fmt.Errorf("output %d: duplicate path %s", index+1, cleaned)
		}
		outputNames[cleaned] = true
	}
	return nil
}

func validateSource(source Source) error {
	if source.File != "" && source.URL != "" {
		return errors.New("only one of file or url may be set")
	}
	if source.File == "" && source.URL == "" {
		return errors.New("one of file or url is required")
	}
	if source.URL != "" {
		if err := validateHTTPURL(source.URL); err != nil {
			return err
		}
	}

	switch source.Type {
	case SourceText:
		if source.Country != "" {
			return errors.New("country is only valid for ipinfo sources")
		}
		if source.URL != "" && source.File != "" {
			return errors.New("text source must use either file or url")
		}
	case SourceIPInfo:
		if source.File == "" {
			return errors.New("ipinfo source must use a local file")
		}
		if len(source.Country) != 2 {
			return fmt.Errorf("ipinfo source requires a two letter country code, got %q", source.Country)
		}
		for _, character := range source.Country {
			if character < 'A' || character > 'Z' {
				return fmt.Errorf("ipinfo source has invalid country code %q", source.Country)
			}
		}
		if !source.AllowsIPv4() && !source.AllowsIPv6() {
			return errors.New("ipinfo source does not select any IP type")
		}
	default:
		return fmt.Errorf("unsupported source type %q", source.Type)
	}
	return nil
}

func validateHTTPURL(value string) error {
	if value == "" {
		return errors.New("URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return errors.New("URL host is required")
	}
	return nil
}
