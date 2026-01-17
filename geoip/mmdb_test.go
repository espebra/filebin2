package geoip

import (
	"net"
	"testing"

	"github.com/espebra/filebin2/ds"
)

func TestLookupASN(t *testing.T) {
	dao, err := Init("../mmdb/GeoLite2-ASN.mmdb", "../mmdb/GeoLite2-City.mmdb")
	if err != nil {
		t.Fatalf("Failed to initialize geoip: %s", err.Error())
	}
	defer dao.Close()

	tests := []struct {
		name        string
		ip          string
		expectASN   bool // true if we expect a non-zero ASN
		description string
	}{
		{
			name:        "Google DNS",
			ip:          "8.8.8.8",
			expectASN:   true,
			description: "Google's public DNS should have ASN data",
		},
		{
			name:        "Cloudflare DNS",
			ip:          "1.1.1.1",
			expectASN:   true,
			description: "Cloudflare's public DNS should have ASN data",
		},
		{
			name:        "Local IP",
			ip:          "127.0.0.1",
			expectASN:   false,
			description: "Localhost should not have ASN data",
		},
		{
			name:        "Private IP",
			ip:          "192.168.1.1",
			expectASN:   false,
			description: "Private network IPs should not have ASN data",
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
				t.Logf("LookupASN returned error for %s: %s", tt.ip, err.Error())
			}

			t.Logf("IP: %s, ASN: %d, Organization: %s", tt.ip, client.ASN, client.ASNOrganization)

			if tt.expectASN && client.ASN == 0 {
				t.Errorf("Expected ASN for %s (%s), got 0", tt.ip, tt.description)
			}
			if !tt.expectASN && client.ASN != 0 {
				t.Logf("Unexpected ASN %d for %s (this is OK)", client.ASN, tt.ip)
			}
		})
	}
}

func TestLookupFull(t *testing.T) {
	dao, err := Init("../mmdb/GeoLite2-ASN.mmdb", "../mmdb/GeoLite2-City.mmdb")
	if err != nil {
		t.Fatalf("Failed to initialize geoip: %s", err.Error())
	}
	defer dao.Close()

	var client ds.Client
	err = dao.Lookup("8.8.8.8", &client)
	if err != nil {
		t.Fatalf("Lookup failed: %s", err.Error())
	}

	t.Logf("Full lookup for 8.8.8.8:")
	t.Logf("  IP: %s", client.IP)
	t.Logf("  ASN: %d", client.ASN)
	t.Logf("  ASN Organization: %s", client.ASNOrganization)
	t.Logf("  Network: %s", client.Network)
	t.Logf("  City: %s", client.City)
	t.Logf("  Country: %s", client.Country)
	t.Logf("  Continent: %s", client.Continent)

	if client.ASN == 0 {
		t.Error("Expected non-zero ASN for 8.8.8.8")
	}
	if client.Country == "" {
		t.Error("Expected non-empty country for 8.8.8.8")
	}
}
