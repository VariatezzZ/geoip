package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolvesRelativePaths(t *testing.T) {
	directory := t.TempDir()
	content := `
ipinfo:
  output: ./ipinfo/ipinfo_lite.mmdb
  mirrors:
    - name: primary
      url: https://example.com/ipinfo-lite.mmdb
      checksums: https://example.com/checksums.txt
categories:
  - name: CN
    sources:
      - type: text
        file: ./data/cn.txt
        ip_types: [ipv4]
  - name: private
    sources:
      - type: text
        file: ./data/private.txt
outputs:
  - ./output/geoip.dat
  - ./output/geoip-only-cn-private.dat
`
	filename := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filename)
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}
	if cfg.Categories[0].Name != "cn" {
		t.Fatalf("category name was not normalized: %s", cfg.Categories[0].Name)
	}
	if !filepath.IsAbs(cfg.Outputs[0]) {
		t.Fatalf("output path is not absolute: %s", cfg.Outputs[0])
	}
	if !filepath.IsAbs(cfg.Categories[0].Sources[0].File) {
		t.Fatalf("source path is not absolute: %s", cfg.Categories[0].Sources[0].File)
	}
	if !strings.HasSuffix(cfg.Outputs[1], "geoip-only-cn-private.dat") {
		t.Fatalf("unexpected output path: %s", cfg.Outputs[1])
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	directory := t.TempDir()
	content := `
unknown: true
ipinfo:
  output: ./ipinfo.mmdb
  mirrors:
    - name: primary
      url: https://example.com/ipinfo-lite.mmdb
      checksums: https://example.com/checksums.txt
categories:
  - name: cn
    sources:
      - type: text
        file: ./cn.txt
  - name: private
    sources:
      - type: text
        file: ./private.txt
outputs:
  - ./geoip.dat
  - ./geoip-only-cn-private.dat
`
	filename := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filename); err == nil {
		t.Fatal("Load accepted an unknown field")
	}
}

func TestLoadRejectsDuplicateOutputs(t *testing.T) {
	directory := t.TempDir()
	content := `
ipinfo:
  output: ./ipinfo.mmdb
  mirrors:
    - name: primary
      url: https://example.com/ipinfo-lite.mmdb
      checksums: https://example.com/checksums.txt
categories:
  - name: cn
    sources:
      - type: text
        file: ./cn.txt
  - name: private
    sources:
      - type: text
        file: ./private.txt
outputs:
  - ./geoip.dat
  - ./geoip.dat
`
	filename := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filename); err == nil {
		t.Fatal("Load accepted duplicate output paths")
	}
}
