package geoip

import (
	"net"
	"testing"

	"github.com/espebra/filebin2/ds"
)

const (
	testASNPath  = "testdata/test-asn.mmdb"
	testCityPath = "testdata/test-city.mmdb"
)

func TestLookupASN(t *testing.T) {
	dao, err := Init(testASNPath, testCityPath)
	if err != nil {
		t.Fatalf("Failed to initialize geoip: %s", err)
	}
	defer dao.Close()

	tests := []struct {
		name         string
		ip           string
		expectASN    int
		expectOrg    string
		expectNoData bool
	}{
		{
			name:      "Google DNS",
			ip:        "8.8.8.8",
			expectASN: 15169,
			expectOrg: "GOOGLE",
		},
		{
			name:      "Cloudflare DNS",
			ip:        "1.1.1.1",
			expectASN: 13335,
			expectOrg: "CLOUDFLARENET",
		},
		{
			name:         "Local IP - no data",
			ip:           "127.0.0.1",
			expectNoData: true,
		},
		{
			name:         "Private IP - no data",
			ip:           "192.168.1.1",
			expectNoData: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}

			var client ds.Client
			err := dao.LookupASN(ip, &client)
			if err != nil {
				t.Errorf("LookupASN returned error for %s: %s", tt.ip, err)
				return
			}

			if tt.expectNoData {
				if client.ASN != 0 {
					t.Errorf("Expected no ASN data for %s, got ASN=%d", tt.ip, client.ASN)
				}
				return
			}

			if client.ASN != tt.expectASN {
				t.Errorf("Expected ASN %d for %s, got %d", tt.expectASN, tt.ip, client.ASN)
			}
			if client.ASNOrganization != tt.expectOrg {
				t.Errorf("Expected organization %q for %s, got %q", tt.expectOrg, tt.ip, client.ASNOrganization)
			}
		})
	}
}

func TestLookupCity(t *testing.T) {
	dao, err := Init(testASNPath, testCityPath)
	if err != nil {
		t.Fatalf("Failed to initialize geoip: %s", err)
	}
	defer dao.Close()

	tests := []struct {
		name            string
		ip              string
		expectCity      string
		expectCountry   string
		expectContinent string
		expectProxy     bool
		expectNoData    bool
	}{
		{
			name:            "Google DNS",
			ip:              "8.8.8.8",
			expectCity:      "Mountain View",
			expectCountry:   "United States",
			expectContinent: "North America",
			expectProxy:     false,
		},
		{
			name:            "Cloudflare DNS",
			ip:              "1.1.1.1",
			expectCity:      "Sydney",
			expectCountry:   "Australia",
			expectContinent: "Oceania",
			expectProxy:     false,
		},
		{
			name:         "Local IP - no data",
			ip:           "127.0.0.1",
			expectNoData: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}

			var client ds.Client
			err := dao.LookupCity(ip, &client)
			if err != nil {
				t.Errorf("LookupCity returned error for %s: %s", tt.ip, err)
				return
			}

			if tt.expectNoData {
				if client.City != "" || client.Country != "" {
					t.Errorf("Expected no city data for %s, got City=%q Country=%q", tt.ip, client.City, client.Country)
				}
				return
			}

			if client.City != tt.expectCity {
				t.Errorf("Expected city %q for %s, got %q", tt.expectCity, tt.ip, client.City)
			}
			if client.Country != tt.expectCountry {
				t.Errorf("Expected country %q for %s, got %q", tt.expectCountry, tt.ip, client.Country)
			}
			if client.Continent != tt.expectContinent {
				t.Errorf("Expected continent %q for %s, got %q", tt.expectContinent, tt.ip, client.Continent)
			}
			if client.Proxy != tt.expectProxy {
				t.Errorf("Expected proxy=%v for %s, got %v", tt.expectProxy, tt.ip, client.Proxy)
			}
		})
	}
}

func TestLookupFull(t *testing.T) {
	dao, err := Init(testASNPath, testCityPath)
	if err != nil {
		t.Fatalf("Failed to initialize geoip: %s", err)
	}
	defer dao.Close()

	var client ds.Client
	err = dao.Lookup("8.8.8.8", &client)
	if err != nil {
		t.Fatalf("Lookup failed: %s", err)
	}

	// Verify ASN data
	if client.ASN != 15169 {
		t.Errorf("Expected ASN 15169, got %d", client.ASN)
	}
	if client.ASNOrganization != "GOOGLE" {
		t.Errorf("Expected organization GOOGLE, got %q", client.ASNOrganization)
	}

	// Verify City data
	if client.IP != "8.8.8.8" {
		t.Errorf("Expected IP 8.8.8.8, got %q", client.IP)
	}
	if client.City != "Mountain View" {
		t.Errorf("Expected city Mountain View, got %q", client.City)
	}
	if client.Country != "United States" {
		t.Errorf("Expected country United States, got %q", client.Country)
	}
	if client.Continent != "North America" {
		t.Errorf("Expected continent North America, got %q", client.Continent)
	}
}

func TestLookupWithPort(t *testing.T) {
	dao, err := Init(testASNPath, testCityPath)
	if err != nil {
		t.Fatalf("Failed to initialize geoip: %s", err)
	}
	defer dao.Close()

	// Test with IP:port format (common in RemoteAddr)
	var client ds.Client
	err = dao.Lookup("8.8.8.8:12345", &client)
	if err != nil {
		t.Fatalf("Lookup with port failed: %s", err)
	}

	if client.ASN != 15169 {
		t.Errorf("Expected ASN 15169, got %d", client.ASN)
	}
	if client.Country != "United States" {
		t.Errorf("Expected country United States, got %q", client.Country)
	}
}

func TestInitWithInvalidPaths(t *testing.T) {
	_, err := Init("/nonexistent/asn.mmdb", "/nonexistent/city.mmdb")
	if err == nil {
		t.Error("Expected error when initializing with invalid paths")
	}
}
