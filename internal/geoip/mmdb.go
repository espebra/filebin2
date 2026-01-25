package geoip

import (
	"fmt"
	"net"

	"github.com/espebra/filebin2/internal/ds"

	"github.com/oschwald/maxminddb-golang"
)

type DAO struct {
	as   *maxminddb.Reader
	city *maxminddb.Reader
}

type record struct {
	Network struct {
		Network string `maxminddb:"network"`
	} `maxminddb:"network"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Country struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Continent struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"continent"`
	Traits struct {
		IsAnonymousProxy bool `maxminddb:"is_anonymous_proxy"`
	} `maxminddb:"traits"`
}

type asnRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

func Init(asnPath string, cityPath string) (DAO, error) {
	var dao DAO

	// GeoIP is optional - if paths are not provided, return empty DAO
	if asnPath == "" || cityPath == "" {
		fmt.Println("GeoIP databases not configured, skipping")
		return dao, nil
	}

	fmt.Printf("Loading mmdb (ASN): %s\n", asnPath)
	asn, err := maxminddb.Open(asnPath)
	if err != nil {
		return dao, err
	}

	fmt.Printf("Loading mmdb (City): %s\n", cityPath)
	city, err := maxminddb.Open(cityPath)
	if err != nil {
		return dao, err
	}
	dao = DAO{as: asn, city: city}
	return dao, nil
}

func (dao DAO) Close() error {
	if dao.as != nil {
		if err := dao.as.Close(); err != nil {
			return err
		}
	}
	if dao.city != nil {
		if err := dao.city.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (dao DAO) Enabled() bool {
	return dao.as != nil && dao.city != nil
}

func (dao DAO) LookupASN(ip net.IP, client *ds.Client) error {
	var r asnRecord
	_, found, err := dao.as.LookupNetwork(ip, &r)
	if err != nil {
		return err
	}
	if found {
		client.ASN = int(r.AutonomousSystemNumber)
		client.ASNOrganization = r.AutonomousSystemOrganization
	}
	return nil
}

func (dao DAO) LookupCity(ip net.IP, client *ds.Client) error {
	var r record
	network, found, err := dao.city.LookupNetwork(ip, &r)
	if err != nil {
		return err
	}

	// Parsed IP/network only
	client.IP = ip.String()
	n := *network
	client.Network = n.String()

	// MMDB lookup result, if any
	if found {
		client.City = r.City.Names["en"]
		client.Country = r.Country.Names["en"]
		client.Continent = r.Continent.Names["en"]
		client.Proxy = r.Traits.IsAnonymousProxy
	}
	return nil
}

func (dao DAO) Lookup(remoteAddr string, client *ds.Client) (err error) {
	// Skip lookup if geoip is not enabled
	if !dao.Enabled() {
		return nil
	}

	// Parse the client IP address
	host, _, err := net.SplitHostPort(remoteAddr)
	var ip net.IP
	if err == nil {
		ip = net.ParseIP(host)
	} else {
		ip = net.ParseIP(remoteAddr)
	}
	if ip == nil {
		return fmt.Errorf("unable to parse remote addr %s", remoteAddr)
	}

	if err := dao.LookupASN(ip, client); err != nil {
		//fmt.Printf("ASN lookup error: %s\n", err.Error())
		return err
	}

	if err := dao.LookupCity(ip, client); err != nil {
		//fmt.Printf("City lookup error: %s\n", err.Error())
		return err
	}
	return nil
}
