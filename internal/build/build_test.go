package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/VariatezzZ/geoip/internal/config"
	"github.com/VariatezzZ/geoip/internal/xraydat"
)

func TestRunWritesIdenticalOutputs(t *testing.T) {
	cfg := loadFixture(t, true)
	report, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if report.Categories["CN"] == 0 || report.Categories["PRIVATE"] == 0 {
		t.Fatalf("unexpected category counts: %v", report.Categories)
	}
	if len(report.Outputs) != 2 {
		t.Fatalf("got %d outputs, want 2", len(report.Outputs))
	}
	if report.Outputs[0].SHA256 != report.Outputs[1].SHA256 {
		t.Fatalf("output checksums differ: %s and %s", report.Outputs[0].SHA256, report.Outputs[1].SHA256)
	}

	first, err := os.ReadFile(cfg.Outputs[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(cfg.Outputs[1])
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("output files are not byte identical")
	}

	entries, err := xraydat.Decode(first)
	if err != nil {
		t.Fatalf("Decode returned an error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Code != "CN" || entries[1].Code != "PRIVATE" {
		t.Fatalf("unexpected categories: %s and %s", entries[0].Code, entries[1].Code)
	}
	if len(entries[0].Prefixes) == 0 || len(entries[1].Prefixes) == 0 {
		t.Fatal("an output category is empty")
	}
}

func TestRunRejectsCNWithoutIPv6(t *testing.T) {
	cfg := loadFixture(t, false)
	if _, err := Run(context.Background(), cfg, nil); err == nil {
		t.Fatal("Run accepted CN without IPv6")
	}
}

func loadFixture(t *testing.T, includeIPv6 bool) *config.Config {
	t.Helper()
	directory := t.TempDir()

	writeFixtureFile(t, filepath.Join(directory, "cn4.txt"), "1.1.1.0/24\n")
	writeFixtureFile(t, filepath.Join(directory, "cn6.txt"), "2001:db8::/32\n")
	writeFixtureFile(t, filepath.Join(directory, "private.txt"), "10.0.0.0/8\nfc00::/7\n")

	cnIPv6Source := ""
	if includeIPv6 {
		cnIPv6Source = "      - type: text\n        file: ./cn6.txt\n        ip_types: [ipv6]\n"
	}

	content := `ipinfo:
  output: ./ipinfo.mmdb
  mirrors:
    - name: primary
      url: https://example.com/ipinfo-lite.mmdb
      checksums: https://example.com/checksums.txt
categories:
  - name: cn
    sources:
      - type: text
        file: ./cn4.txt
        ip_types: [ipv4]
` + cnIPv6Source + `  - name: private
    sources:
      - type: text
        file: ./private.txt
outputs:
  - ./output/geoip.dat
  - ./output/geoip-only-cn-private.dat
`
	filename := filepath.Join(directory, "config.yaml")
	writeFixtureFile(t, filename, content)

	cfg, err := config.Load(filename)
	if err != nil {
		t.Fatalf("load fixture configuration: %v", err)
	}
	return cfg
}

func writeFixtureFile(t *testing.T, filename string, content string) {
	t.Helper()
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
