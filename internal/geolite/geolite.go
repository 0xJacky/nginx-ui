package geolite

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/oschwald/geoip2-golang/v2"
	"github.com/uozi-tech/cosy/geoip"
)

type IPLocation struct {
	RegionCode string `json:"region_code"`
	Province   string `json:"province"`
	City       string `json:"city"`
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
	// Load database from file (memory-mapped for efficiency)
	dbPath := GetDBPath()

	if !DBExists() {
		return fmt.Errorf("GeoLite2 database not found at %s. Please download it first", dbPath)
	}

	if err := s.loadFromFile(dbPath); err != nil {
		return fmt.Errorf("failed to load GeoLite2 database: %v", err)
	}

	return nil
}

func (s *Service) loadFromFile(path string) error {
	// Open database file with memory mapping (more efficient than loading into memory)
	cityDB, err := geoip2.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open GeoLite2 database: %v", err)
	}

	s.cityDB = cityDB
	return nil
}

func (s *Service) Search(ipStr string) (*IPLocation, error) {
	if s.cityDB == nil {
		return nil, fmt.Errorf("no databases loaded")
	}

	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	loc := &IPLocation{}

	// Use cosy geoip for country code
	loc.RegionCode = geoip.ParseIP(ipStr)

	// Use city database for detailed information
	if record, err := s.cityDB.City(ip); err == nil {
		// Override country code from city database if cosy didn't provide it
		if loc.RegionCode == "" {
			loc.RegionCode = record.Country.ISOCode
		}

		if len(record.Subdivisions) > 0 {
			loc.Province = record.Subdivisions[0].Names.English
		}

		loc.City = record.City.Names.English

		// Get Chinese names for Chinese regions
		if loc.RegionCode == "CN" || loc.RegionCode == "HK" ||
			loc.RegionCode == "MO" || loc.RegionCode == "TW" {
			if len(record.Subdivisions) > 0 {
				if cnRegion := record.Subdivisions[0].Names.SimplifiedChinese; cnRegion != "" {
					loc.Province = cnRegion
				}
			} else {
				// If it's a Chinese IP but has no province, mark it as "其它"
				loc.Province = "其它"
			}
			if cnCity := record.City.Names.SimplifiedChinese; cnCity != "" {
				loc.City = cnCity
			}

			loc.RegionCode = "CN"
		}

		return loc, nil
	}

	// If city database lookup fails, return minimal info with country code
	if loc.RegionCode != "" {
		return loc, nil
	}

	return nil, fmt.Errorf("no location data found for IP: %s", ipStr)
}

func (s *Service) SearchWithISO(ipStr string) (*IPLocation, error) {
	// This method specifically returns English names and ISO codes
	if s.cityDB == nil {
		return nil, fmt.Errorf("no databases loaded")
	}

	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	loc := &IPLocation{}

	// Use cosy geoip for country code
	loc.RegionCode = geoip.ParseIP(ipStr)

	// Use city database for detailed information
	if record, err := s.cityDB.City(ip); err == nil {
		// Override country code from city database if cosy didn't provide it
		if loc.RegionCode == "" {
			loc.RegionCode = record.Country.ISOCode
		}

		if len(record.Subdivisions) > 0 {
			loc.RegionCode = record.Subdivisions[0].Names.English
		}

		loc.RegionCode = record.City.Names.English

		return loc, nil
	}

	// If city database lookup fails, return minimal info with country code
	if loc.RegionCode != "" {
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
	return loc != nil && (loc.RegionCode == "CN" ||
		loc.RegionCode == "HK" ||
		loc.RegionCode == "MO" ||
		loc.RegionCode == "TW")
}

func IsChineseRegion(regionCode string) bool {
	chineseRegionCodes := []string{"CN", "HK", "MO", "TW"}
	for _, region := range chineseRegionCodes {
		if regionCode == region {
			return true
		}
	}
	return false
}
