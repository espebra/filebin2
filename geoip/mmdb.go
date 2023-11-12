package geoip

import (
	"fmt"
	"github.com/espebra/filebin2/ds"
	"github.com/oschwald/maxminddb-golang"
	"net"
)

type DAO struct {
	db *maxminddb.Reader
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

func Init(path string) (DAO, error) {
	var dao DAO
	db, err := maxminddb.Open(path)
	if err != nil {
		return dao, err
	}
	dao = DAO{db: db}
	fmt.Printf("Loading mmdb: %s\n", path)
	return dao, nil
}

func (dao DAO) Close() error {
	return dao.db.Close()
}

func (dao DAO) Lookup(ip net.IP) (client ds.Client, err error) {
	var r record
	network, found, err := dao.db.LookupNetwork(ip, &r)
	if err != nil {
		return client, err
	}
	if found {
		client.IP = ip
		client.Network = network.String()
		client.City = r.City.Names["en"]
		client.Country = r.Country.Names["en"]
		client.Continent = r.Continent.Names["en"]
		client.Proxy = r.Traits.IsAnonymousProxy
	}
	//fmt.Printf("The client is: %v\n", client)
	return client, nil
}
