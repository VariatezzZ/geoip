package ipinfo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/VariatezzZ/geoip/internal/config"
)

func TestFindChecksum(t *testing.T) {
	data := []byte("database")
	sum := sha256.Sum256(data)
	checksums := []byte(fmt.Sprintf("%s  ipinfo-lite.mmdb\n", hex.EncodeToString(sum[:])))

	value, found, err := findChecksum(checksums, "ipinfo-lite.mmdb")
	if err != nil {
		t.Fatalf("findChecksum returned an error: %v", err)
	}
	if !found {
		t.Fatal("findChecksum did not find the checksum")
	}
	if value != hex.EncodeToString(sum[:]) {
		t.Fatalf("unexpected checksum value: %s", value)
	}
	if err := verifyChecksum(data, checksums, "ipinfo-lite.mmdb"); err != nil {
		t.Fatalf("verifyChecksum returned an error: %v", err)
	}
}

func TestVerifyChecksumRejectsMismatch(t *testing.T) {
	checksums := []byte(strings.Repeat("0", 64) + "  ipinfo-lite.mmdb\n")
	if err := verifyChecksum([]byte("database"), checksums, "ipinfo-lite.mmdb"); err == nil {
		t.Fatal("verifyChecksum accepted a mismatch")
	}
}

func TestAssetName(t *testing.T) {
	name, err := assetName("https://example.com/releases/latest/download/ipinfo-lite.mmdb?download=1")
	if err != nil {
		t.Fatalf("assetName returned an error: %v", err)
	}
	if name != "ipinfo-lite.mmdb" {
		t.Fatalf("got %s, want ipinfo-lite.mmdb", name)
	}
}

func TestValidateBytesRejectsInvalidData(t *testing.T) {
	if err := ValidateBytes([]byte("not an mmdb")); err == nil {
		t.Fatal("ValidateBytes accepted invalid data")
	}
}

func TestLoadPrefixesRejectsMissingFile(t *testing.T) {
	if _, err := LoadPrefixes("missing.mmdb", "CN", false, true); err == nil {
		t.Fatal("LoadPrefixes accepted a missing file")
	}
}

type failingGetter struct{}

func (failingGetter) Get(context.Context, string) ([]byte, error) {
	return nil, errors.New("unavailable")
}

func TestFetchReportsAllMirrorFailures(t *testing.T) {
	cfg := config.IPInfoConfig{
		Mirrors: []config.Mirror{
			{Name: "primary", URL: "https://example.com/ipinfo-lite.mmdb", Checksums: "https://example.com/checksums.txt"},
			{Name: "secondary", URL: "https://example.com/ipinfo_lite.mmdb"},
		},
	}
	if _, err := Fetch(context.Background(), cfg, failingGetter{}); err == nil {
		t.Fatal("Fetch succeeded with failing mirrors")
	}
}

func TestLoadPrefixesRejectsInvalidCountry(t *testing.T) {
	if _, err := LoadPrefixes("missing.mmdb", "CHN", true, true); err == nil {
		t.Fatal("LoadPrefixes accepted an invalid country code")
	}
}
