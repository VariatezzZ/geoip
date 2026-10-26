package ipset

import (
	"net/netip"
	"testing"
)

func TestAggregateMergesAndDeduplicates(t *testing.T) {
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/25"),
		netip.MustParsePrefix("10.0.0.128/25"),
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("2001:db8::/32"),
	}

	result, err := Aggregate(prefixes)
	if err != nil {
		t.Fatalf("Aggregate returned an error: %v", err)
	}

	expected := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("2001:db8::/32"),
	}
	if len(result) != len(expected) {
		t.Fatalf("got %d prefixes, want %d: %v", len(result), len(expected), result)
	}
	for index := range expected {
		if result[index] != expected[index] {
			t.Fatalf("prefix %d: got %s, want %s", index, result[index], expected[index])
		}
	}
}

func TestNormalizePrefixConvertsMappedIPv4(t *testing.T) {
	prefix := netip.MustParsePrefix("::ffff:192.0.2.0/120")
	result, err := NormalizePrefix(prefix)
	if err != nil {
		t.Fatalf("NormalizePrefix returned an error: %v", err)
	}
	if result.String() != "192.0.2.0/24" {
		t.Fatalf("got %s, want 192.0.2.0/24", result)
	}
}

func TestFilterFamily(t *testing.T) {
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8::/32"),
	}
	result := FilterFamily(prefixes, true, false)
	if len(result) != 1 || !result[0].Addr().Is4() {
		t.Fatalf("unexpected IPv4 filter result: %v", result)
	}
	result = FilterFamily(prefixes, false, true)
	if len(result) != 1 || !result[0].Addr().Is6() {
		t.Fatalf("unexpected IPv6 filter result: %v", result)
	}
}
