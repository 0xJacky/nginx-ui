package geolite

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
	"github.com/ulikunitz/xz"
	"github.com/uozi-tech/cosy/geoip"
)

//go:embed GeoLite2-City.mmdb.xz
var embeddedCityDB []byte

type IPLocation struct {
	CountryCode string `json:"country_code"`
	Region      string `json:"region"`
	RegionCode  string `json:"region_code"`
	City        string `json:"city"`
}

type Service struct {
	cityDB *geoip2.Reader
}

var (
	instance *Service
	once     sync.Once
	initErr  error
)

func GetService() (*Service, error) {
	once.Do(func() {
		instance = &Service{}
		initErr = instance.init()
	})
	return instance, initErr
}

func (s *Service) init() error {
	// Load embedded compressed database
	if len(embeddedCityDB) > 0 {
		if err := s.loadEmbeddedCityDB(); err != nil {
			return fmt.Errorf("failed to load embedded city database: %v", err)
		}
		return nil
	}

	return fmt.Errorf("no embedded GeoLite2 City database available")
}

func (s *Service) loadEmbeddedCityDB() error {
	// Decompress the embedded database
	xzReader, err := xz.NewReader(bytes.NewReader(embeddedCityDB))
	if err != nil {
		return fmt.Errorf("failed to create xz reader: %v", err)
	}

	decompressedData, err := io.ReadAll(xzReader)
	if err != nil {
		return fmt.Errorf("failed to decompress database: %v", err)
	}

	// Create geoip2 reader from decompressed data
	cityDB, err := geoip2.FromBytes(decompressedData)
	if err != nil {
		return fmt.Errorf("failed to create geoip2 reader: %v", err)
	}

	s.cityDB = cityDB
	return nil
}

func (s *Service) Search(ipStr string) (*IPLocation, error) {
	if s.cityDB == nil {
		return nil, fmt.Errorf("no databases loaded")
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	loc := &IPLocation{}

	// Use cosy geoip for country code
	loc.CountryCode = geoip.ParseIP(ipStr)

	// Use city database for detailed information
	if record, err := s.cityDB.City(ip); err == nil {
		// Override country code from city database if cosy didn't provide it
		if loc.CountryCode == "" {
			loc.CountryCode = record.Country.IsoCode
		}

		if len(record.Subdivisions) > 0 {
			loc.Region = record.Subdivisions[0].Names["en"]
			loc.RegionCode = record.Subdivisions[0].
				IsoCode
		}

		loc.City = record.City.Names["en"]

		// Get Chinese names for Chinese regions
		if loc.CountryCode == "CN" || loc.CountryCode == "HK" ||
			loc.CountryCode == "MO" || loc.CountryCode == "TW" {
			if len(record.Subdivisions) > 0 {
				if cnRegion := record.Subdivisions[0].Names["zh-CN"]; cnRegion != "" {
					loc.Region = cnRegion
				}
			}
			if cnCity := record.City.Names["zh-CN"]; cnCity != "" {
				loc.City = cnCity
			}
		}

		return loc, nil
	}

	// If city database lookup fails, return minimal info with country code
	if loc.CountryCode != "" {
		return loc, nil
	}

	return nil, fmt.Errorf("no location data found for IP: %s", ipStr)
}

func (s *Service) SearchWithISO(ipStr string) (*IPLocation, error) {
	// This method specifically returns English names and ISO codes
	if s.cityDB == nil {
		return nil, fmt.Errorf("no databases loaded")
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	loc := &IPLocation{}

	// Use cosy geoip for country code
	loc.CountryCode = geoip.ParseIP(ipStr)

	// Use city database for detailed information
	if record, err := s.cityDB.City(ip); err == nil {
		// Override country code from city database if cosy didn't provide it
		if loc.CountryCode == "" {
			loc.CountryCode = record.Country.IsoCode
		}

		if len(record.Subdivisions) > 0 {
			loc.Region = record.Subdivisions[0].Names["en"]
			loc.RegionCode = record.Subdivisions[0].IsoCode
		}

		loc.City = record.City.Names["en"]

		return loc, nil
	}

	// If city database lookup fails, return minimal info with country code
	if loc.CountryCode != "" {
		return loc, nil
	}

	return nil, fmt.Errorf("no location data found for IP: %s", ipStr)
}

func (s *Service) Close() {
	if s.cityDB != nil {
		s.cityDB.Close()
		s.cityDB = nil
	}
}

func IsChineseIP(loc *IPLocation) bool {
	return loc != nil && (loc.CountryCode == "CN" ||
		loc.CountryCode == "HK" ||
		loc.CountryCode == "MO" ||
		loc.CountryCode == "TW")
}

func IsChineseRegion(code string) bool {
	chineseRegions := []string{"CN", "HK", "MO", "TW"}
	for _, region := range chineseRegions {
		if code == region {
			return true
		}
	}
	return false
}
