package ipset

import (
	"errors"
	"fmt"
	"net/netip"

	"go4.org/netipx"
)

func NormalizePrefix(prefix netip.Prefix) (netip.Prefix, error) {
	if !prefix.IsValid() {
		return netip.Prefix{}, errors.New("invalid prefix")
	}

	address := prefix.Addr()
	if address.Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("prefix %s contains a zone", prefix)
	}
	if address.Is4In6() {
		if prefix.Bits() < 96 {
			return netip.Prefix{}, fmt.Errorf("prefix %s cannot be converted to IPv4", prefix)
		}
		prefix = netip.PrefixFrom(address.Unmap(), prefix.Bits()-96)
		address = prefix.Addr()
	}

	if address.Is4() {
		if prefix.Bits() < 0 || prefix.Bits() > 32 {
			return netip.Prefix{}, fmt.Errorf("prefix %s has an invalid IPv4 length", prefix)
		}
	} else if address.Is6() {
		if prefix.Bits() < 0 || prefix.Bits() > 128 {
			return netip.Prefix{}, fmt.Errorf("prefix %s has an invalid IPv6 length", prefix)
		}
	} else {
		return netip.Prefix{}, fmt.Errorf("prefix %s has an unsupported address family", prefix)
	}

	return prefix.Masked(), nil
}

func Aggregate(prefixes []netip.Prefix) ([]netip.Prefix, error) {
	var builder netipx.IPSetBuilder
	for index, prefix := range prefixes {
		normalized, err := NormalizePrefix(prefix)
		if err != nil {
			return nil, fmt.Errorf("prefix %d: %w", index+1, err)
		}
		builder.AddPrefix(normalized)
	}

	set, err := builder.IPSet()
	if err != nil {
		return nil, fmt.Errorf("build IP set: %w", err)
	}
	return set.Prefixes(), nil
}

func FilterFamily(prefixes []netip.Prefix, ipv4 bool, ipv6 bool) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(prefixes))
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() && ipv4 {
			result = append(result, prefix)
			continue
		}
		if prefix.Addr().Is6() && ipv6 {
			result = append(result, prefix)
		}
	}
	return result
}

func HasIPv4(prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() {
			return true
		}
	}
	return false
}

func HasIPv6(prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Addr().Is6() {
			return true
		}
	}
	return false
}
