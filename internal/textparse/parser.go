package textparse

import (
	"bufio"
	"bytes"
	"fmt"
	"net/netip"
	"strings"

	"github.com/VariatezzZ/geoip/internal/ipset"
)

func Parse(data []byte) ([]netip.Prefix, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	result := make([]netip.Prefix, 0, 1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 1 {
			return nil, fmt.Errorf("line %d: expected one CIDR, got %q", lineNumber, line)
		}

		prefix, err := parseValue(fields[0])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		result = append(result, prefix)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read text data: %w", err)
	}
	return result, nil
}

func parseValue(value string) (netip.Prefix, error) {
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("invalid CIDR %q: %w", value, err)
		}
		normalized, err := ipset.NormalizePrefix(prefix)
		if err != nil {
			return netip.Prefix{}, err
		}
		return normalized, nil
	}

	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid IP address %q: %w", value, err)
	}
	bits := 128
	if address.Is4() {
		bits = 32
	}
	normalized, err := ipset.NormalizePrefix(netip.PrefixFrom(address, bits))
	if err != nil {
		return netip.Prefix{}, err
	}
	return normalized, nil
}
