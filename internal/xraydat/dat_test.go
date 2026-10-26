package xraydat

import (
	"net/netip"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	entries := []Entry{
		{
			Code: "PRIVATE",
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.0/8"),
				netip.MustParsePrefix("fc00::/7"),
			},
		},
		{
			Code: "CN",
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("2001:db8::/32"),
				netip.MustParsePrefix("192.0.2.0/24"),
			},
		},
	}

	data, err := Encode(entries)
	if err != nil {
		t.Fatalf("Encode returned an error: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode returned an error: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("got %d entries, want 2", len(decoded))
	}
	if decoded[0].Code != "CN" || decoded[1].Code != "PRIVATE" {
		t.Fatalf("unexpected entry order: %s, %s", decoded[0].Code, decoded[1].Code)
	}
	if len(decoded[0].Prefixes) != 2 || len(decoded[1].Prefixes) != 2 {
		t.Fatalf("unexpected prefix counts: %d, %d", len(decoded[0].Prefixes), len(decoded[1].Prefixes))
	}
}

func TestEncodeRejectsLowercaseCode(t *testing.T) {
	_, err := Encode([]Entry{{
		Code:     "cn",
		Prefixes: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
	}})
	if err == nil {
		t.Fatal("Encode accepted a lowercase code")
	}
}

func TestDecodeRejectsEmptyData(t *testing.T) {
	entries, err := Decode(nil)
	if err != nil {
		t.Fatalf("Decode returned an error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}
