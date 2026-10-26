package textparse

import (
	"testing"
)

func TestParse(t *testing.T) {
	data := []byte("\xef\xbb\xbf# comment\r\n1.2.3.0/24 # inline\r\n\r\n8.8.8.8\r\n2001:db8::/32\r\n::ffff:192.0.2.0/120\r\n")

	prefixes, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if len(prefixes) != 4 {
		t.Fatalf("got %d prefixes, want 4: %v", len(prefixes), prefixes)
	}

	expected := []string{
		"1.2.3.0/24",
		"8.8.8.8/32",
		"2001:db8::/32",
		"192.0.2.0/24",
	}
	for index, value := range expected {
		if prefixes[index].String() != value {
			t.Fatalf("prefix %d: got %s, want %s", index, prefixes[index], value)
		}
	}
}

func TestParseRejectsInvalidLine(t *testing.T) {
	if _, err := Parse([]byte("1.2.3.0/24\nnot-a-prefix\n")); err == nil {
		t.Fatal("Parse accepted an invalid line")
	}
}

func TestParseRejectsExtraFields(t *testing.T) {
	if _, err := Parse([]byte("1.2.3.0/24 trailing\n")); err == nil {
		t.Fatal("Parse accepted extra fields")
	}
}
