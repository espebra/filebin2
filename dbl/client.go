package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	//"net/http"
	"time"

	"github.com/espebra/filebin2/ds"

	"github.com/dustin/go-humanize"
)

type ClientDao struct {
	db *sql.DB
}

func (c *ClientDao) GetByIP(ip net.IP) (client ds.Client, found bool, err error) {
	sqlStatement := "SELECT ip, asn, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client WHERE ip = $1 LIMIT 1"
	err = c.db.QueryRow(sqlStatement, ip.String()).Scan(&client.IP, &client.ASN, &client.Network, &client.City, &client.Country, &client.Continent, &client.Proxy, &client.Requests, &client.FirstActiveAt, &client.LastActiveAt, &client.BannedAt, &client.BannedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			client.IP = ip.String()
			//client.Requests = 0
			//now := time.Now().UTC()
			//client.FirstActiveAt = now
			//client.LastActiveAt = now
			return client, false, nil
		}
		return client, false, err
	}
	// https://github.com/lib/pq/issues/329
	client.FirstActiveAt = client.FirstActiveAt.UTC()
	client.LastActiveAt = client.LastActiveAt.UTC()
	client.FirstActiveAtRelative = humanize.Time(client.FirstActiveAt)
	client.LastActiveAtRelative = humanize.Time(client.LastActiveAt)
	if client.IsBanned() {
		client.BannedAt.Time = client.BannedAt.Time.UTC()
		client.BannedAtRelative = humanize.Time(client.BannedAt.Time)
	}
	return client, true, nil
}

func (c *ClientDao) GetByRemoteAddr(remoteAddr string) (client ds.Client, found bool, err error) {
	// Parse the client IP address
	host, _, err := net.SplitHostPort(remoteAddr)
	var ip net.IP
	if err == nil {
		ip = net.ParseIP(host)
	} else {
		ip = net.ParseIP(remoteAddr)
	}
	if ip == nil {
		return client, false, errors.New(fmt.Sprintf("Unable to parse remote addr %s", remoteAddr))
	}

	// Look up the client by IP
	client, found, err = c.GetByIP(ip)
	return client, found, err
}

func (c *ClientDao) Update(client *ds.Client) (err error) {
	now := time.Now().UTC()
	sqlStatement := "INSERT INTO client (ip, asn, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, '') ON CONFLICT(ip) DO UPDATE SET asn=$11, network=$12, city=$13, country=$14, continent=$15, proxy=$16, requests=client.requests+1, last_active_at=$17 RETURNING ip, requests"
	if err := c.db.QueryRow(sqlStatement, client.IP, client.ASN, client.Network, client.City, client.Country, client.Continent, client.Proxy, 1, now, now, client.ASN, client.Network, client.City, client.Country, client.Continent, client.Proxy, now).Scan(&client.IP, &client.Requests); err != nil {
		return err
	}
	return err
}

func (c *ClientDao) GetAll() (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client ORDER BY last_active_at DESC"
	clients, err = c.clientQuery(sqlStatement)
	return clients, err
}

func (c *ClientDao) GetByLastActiveAt(limit int) (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client ORDER BY last_active_at DESC LIMIT $1"
	clients, err = c.clientQuery(sqlStatement, limit)
	return clients, err
}

func (c *ClientDao) GetByRequests(limit int) (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client ORDER BY requests DESC LIMIT $1"
	clients, err = c.clientQuery(sqlStatement, limit)
	return clients, err
}

func (c *ClientDao) GetByBannedAt(limit int) (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client WHERE banned_at > $1 ORDER BY banned_at DESC LIMIT $2"
	clients, err = c.clientQuery(sqlStatement, time.Unix(0, 0), limit)
	return clients, err
}

func (c *ClientDao) clientQuery(sqlStatement string, params ...interface{}) (clients []ds.Client, err error) {
	rows, err := c.db.Query(sqlStatement, params...)
	if err != nil {
		return clients, err
	}
	defer rows.Close()
	for rows.Next() {
		var client ds.Client
		err = rows.Scan(&client.IP, &client.ASN, &client.Network, &client.City, &client.Country, &client.Continent, &client.Proxy, &client.Requests, &client.FirstActiveAt, &client.LastActiveAt, &client.BannedAt, &client.BannedBy)
		if err != nil {
			return clients, err
		}
		// https://github.com/lib/pq/issues/329
		client.FirstActiveAt = client.FirstActiveAt.UTC()
		client.LastActiveAt = client.LastActiveAt.UTC()
		client.FirstActiveAtRelative = humanize.Time(client.FirstActiveAt)
		client.LastActiveAtRelative = humanize.Time(client.LastActiveAt)
		if client.IsBanned() {
			client.BannedAt.Time = client.BannedAt.Time.UTC()
			client.BannedAtRelative = humanize.Time(client.BannedAt.Time)
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func (c *ClientDao) Ban(IPsToBan []string, banByRemoteAddr string) (err error) {
	// Loop over the IP addresses that will be banned
	now := time.Now().UTC()
	sqlStatement := "UPDATE client SET banned_at=$1, banned_by=$2 WHERE ip=$3 RETURNING ip"
	var ret string
	for _, ipToBan := range IPsToBan {
		if err := c.db.QueryRow(sqlStatement, now, banByRemoteAddr, ipToBan).Scan(&ret); err != nil {
			return err
		}
	}
	return err
}
