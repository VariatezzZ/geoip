package xraydat

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/VariatezzZ/geoip/internal/ipset"
	"google.golang.org/protobuf/encoding/protowire"
)

type Entry struct {
	Code     string
	Prefixes []netip.Prefix
}

func Encode(entries []Entry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, errors.New("no entries to encode")
	}

	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	slices.SortFunc(sorted, func(left Entry, right Entry) int {
		return strings.Compare(left.Code, right.Code)
	})

	seen := make(map[string]bool, len(sorted))
	var output []byte
	for _, entry := range sorted {
		if err := validateCode(entry.Code); err != nil {
			return nil, err
		}
		if seen[entry.Code] {
			return nil, fmt.Errorf("duplicate entry code %q", entry.Code)
		}
		seen[entry.Code] = true
		if len(entry.Prefixes) == 0 {
			return nil, fmt.Errorf("entry %s has no prefixes", entry.Code)
		}

		message, err := encodeGeoIP(entry)
		if err != nil {
			return nil, err
		}
		output = protowire.AppendTag(output, 1, protowire.BytesType)
		output = protowire.AppendBytes(output, message)
	}
	return output, nil
}

func validateCode(code string) error {
	if code == "" {
		return errors.New("entry code is empty")
	}
	for _, character := range code {
		if character < 'A' || character > 'Z' {
			return fmt.Errorf("entry code %q must contain only uppercase letters", code)
		}
	}
	return nil
}

func encodeGeoIP(entry Entry) ([]byte, error) {
	prefixes := make([]netip.Prefix, 0, len(entry.Prefixes))
	seen := make(map[netip.Prefix]bool, len(entry.Prefixes))
	for index, prefix := range entry.Prefixes {
		normalized, err := ipset.NormalizePrefix(prefix)
		if err != nil {
			return nil, fmt.Errorf("entry %s prefix %d: %w", entry.Code, index+1, err)
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		prefixes = append(prefixes, normalized)
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("entry %s has no prefixes", entry.Code)
	}

	slices.SortFunc(prefixes, comparePrefix)

	var output []byte
	output = protowire.AppendTag(output, 1, protowire.BytesType)
	output = protowire.AppendString(output, entry.Code)
	for _, prefix := range prefixes {
		message := encodeCIDR(prefix)
		output = protowire.AppendTag(output, 2, protowire.BytesType)
		output = protowire.AppendBytes(output, message)
	}
	return output, nil
}

func encodeCIDR(prefix netip.Prefix) []byte {
	var output []byte
	output = protowire.AppendTag(output, 1, protowire.BytesType)
	output = protowire.AppendBytes(output, prefix.Addr().AsSlice())
	if prefix.Bits() != 0 {
		output = protowire.AppendTag(output, 2, protowire.VarintType)
		output = protowire.AppendVarint(output, uint64(prefix.Bits()))
	}
	return output
}

func comparePrefix(left netip.Prefix, right netip.Prefix) int {
	if comparison := left.Addr().Compare(right.Addr()); comparison != 0 {
		return comparison
	}
	return left.Bits() - right.Bits()
}

func Decode(data []byte) ([]Entry, error) {
	entries := make([]Entry, 0, 2)

	for len(data) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(data)
		if tagLength < 0 {
			return nil, fmt.Errorf("decode DAT tag: %w", protowire.ParseError(tagLength))
		}
		data = data[tagLength:]

		if number == 1 && wireType == protowire.BytesType {
			value, length := protowire.ConsumeBytes(data)
			if length < 0 {
				return nil, fmt.Errorf("decode DAT entry: %w", protowire.ParseError(length))
			}
			entry, err := decodeGeoIP(value)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entry)
			data = data[length:]
			continue
		}

		length := protowire.ConsumeFieldValue(number, wireType, data)
		if length < 0 {
			return nil, fmt.Errorf("skip DAT field: %w", protowire.ParseError(length))
		}
		data = data[length:]
	}

	return entries, nil
}

func decodeGeoIP(data []byte) (Entry, error) {
	var entry Entry

	for len(data) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(data)
		if tagLength < 0 {
			return Entry{}, fmt.Errorf("decode GeoIP tag: %w", protowire.ParseError(tagLength))
		}
		data = data[tagLength:]

		switch {
		case number == 1 && wireType == protowire.BytesType:
			value, length := protowire.ConsumeBytes(data)
			if length < 0 {
				return Entry{}, fmt.Errorf("decode country code: %w", protowire.ParseError(length))
			}
			entry.Code = string(value)
			data = data[length:]
		case number == 2 && wireType == protowire.BytesType:
			value, length := protowire.ConsumeBytes(data)
			if length < 0 {
				return Entry{}, fmt.Errorf("decode CIDR: %w", protowire.ParseError(length))
			}
			prefix, err := decodeCIDR(value)
			if err != nil {
				return Entry{}, err
			}
			entry.Prefixes = append(entry.Prefixes, prefix)
			data = data[length:]
		default:
			length := protowire.ConsumeFieldValue(number, wireType, data)
			if length < 0 {
				return Entry{}, fmt.Errorf("skip GeoIP field: %w", protowire.ParseError(length))
			}
			data = data[length:]
		}
	}

	if entry.Code == "" {
		return Entry{}, errors.New("GeoIP entry has no country code")
	}
	return entry, nil
}

func decodeCIDR(data []byte) (netip.Prefix, error) {
	var (
		addressBytes []byte
		bits         uint64
	)

	for len(data) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(data)
		if tagLength < 0 {
			return netip.Prefix{}, fmt.Errorf("decode CIDR tag: %w", protowire.ParseError(tagLength))
		}
		data = data[tagLength:]

		switch {
		case number == 1 && wireType == protowire.BytesType:
			value, length := protowire.ConsumeBytes(data)
			if length < 0 {
				return netip.Prefix{}, fmt.Errorf("decode CIDR address: %w", protowire.ParseError(length))
			}
			addressBytes = append(addressBytes[:0], value...)
			data = data[length:]
		case number == 2 && wireType == protowire.VarintType:
			value, length := protowire.ConsumeVarint(data)
			if length < 0 {
				return netip.Prefix{}, fmt.Errorf("decode CIDR prefix: %w", protowire.ParseError(length))
			}
			bits = value
			data = data[length:]
		default:
			length := protowire.ConsumeFieldValue(number, wireType, data)
			if length < 0 {
				return netip.Prefix{}, fmt.Errorf("skip CIDR field: %w", protowire.ParseError(length))
			}
			data = data[length:]
		}
	}

	var address netip.Addr
	switch len(addressBytes) {
	case 4:
		var value [4]byte
		copy(value[:], addressBytes)
		address = netip.AddrFrom4(value)
	case 16:
		var value [16]byte
		copy(value[:], addressBytes)
		address = netip.AddrFrom16(value)
	default:
		return netip.Prefix{}, fmt.Errorf("CIDR address has %d bytes, expected 4 or 16", len(addressBytes))
	}

	maximum := 32
	if address.Is6() {
		maximum = 128
	}
	if bits > uint64(maximum) {
		return netip.Prefix{}, fmt.Errorf("CIDR prefix length %d exceeds %d", bits, maximum)
	}

	prefix := netip.PrefixFrom(address, int(bits))
	if prefix != prefix.Masked() {
		return netip.Prefix{}, fmt.Errorf("CIDR %s contains host bits", prefix)
	}
	return prefix, nil
}
