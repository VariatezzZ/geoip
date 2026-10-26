package ipinfo

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

type record struct {
	CountryCode string `maxminddb:"country_code"`
}

func LoadPrefixes(filename string, country string, ipv4 bool, ipv6 bool) ([]netip.Prefix, error) {
	if filename == "" {
		return nil, errors.New("IPinfo database path is empty")
	}
	if len(country) != 2 {
		return nil, fmt.Errorf("invalid country code %q", country)
	}
	if !ipv4 && !ipv6 {
		return nil, errors.New("no IP type selected")
	}

	database, err := maxminddb.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open IPinfo database %s: %w", filename, err)
	}
	defer database.Close()

	country = strings.ToUpper(country)
	result := make([]netip.Prefix, 0, 4096)

	for network := range database.Networks() {
		if err := network.Err(); err != nil {
			return nil, fmt.Errorf("iterate IPinfo database: %w", err)
		}

		prefix := network.Prefix()
		if !prefix.IsValid() {
			return nil, fmt.Errorf("IPinfo database returned an invalid network")
		}
		if prefix.Addr().Is4() && !ipv4 {
			continue
		}
		if prefix.Addr().Is6() && !ipv6 {
			continue
		}

		var location record
		if err := network.Decode(&location); err != nil {
			return nil, fmt.Errorf("decode IPinfo network %s: %w", prefix, err)
		}
		if !strings.EqualFold(strings.TrimSpace(location.CountryCode), country) {
			continue
		}
		result = append(result, prefix.Masked())
	}

	return result, nil
}

func ValidateBytes(data []byte) error {
	if len(data) == 0 {
		return errors.New("MMDB data is empty")
	}
	database, err := maxminddb.OpenBytes(data)
	if err != nil {
		return fmt.Errorf("invalid MMDB data: %w", err)
	}
	return database.Close()
}
