package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
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

func (c *ClientDao) Cleanup(days uint64) (count int64, err error) {
	sqlStatement := fmt.Sprintf("DELETE FROM client WHERE last_active_at < CURRENT_DATE - CAST('%d days' AS interval) AND banned_at IS NULL", days)
	res, err := c.db.Exec(sqlStatement)
	if err != nil {
		return 0, err
	}
	count, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (c *ClientDao) GetByFilesUploaded(limit int) (clients []ds.Client, err error) {
	sqlStatement := `
		SELECT
			c.ip, c.asn, c.network, c.city, c.country, c.continent, c.proxy,
			c.requests, c.first_active_at, c.last_active_at, c.banned_at, c.banned_by,
			COALESCE(COUNT(f.id), 0) as files_uploaded,
			COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
		FROM client c
		LEFT JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
		LEFT JOIN file_content fc ON fc.sha256 = f.sha256
		GROUP BY c.ip
		ORDER BY files_uploaded DESC, bytes_uploaded DESC
		LIMIT $1`
	rows, err := c.db.Query(sqlStatement, limit)
	if err != nil {
		return clients, err
	}
	defer rows.Close()
	for rows.Next() {
		var client ds.Client
		err = rows.Scan(
			&client.IP, &client.ASN, &client.Network, &client.City, &client.Country,
			&client.Continent, &client.Proxy, &client.Requests, &client.FirstActiveAt,
			&client.LastActiveAt, &client.BannedAt, &client.BannedBy,
			&client.FilesUploaded, &client.BytesUploaded,
		)
		if err != nil {
			return clients, err
		}
		client.FirstActiveAt = client.FirstActiveAt.UTC()
		client.LastActiveAt = client.LastActiveAt.UTC()
		client.FirstActiveAtRelative = humanize.Time(client.FirstActiveAt)
		client.LastActiveAtRelative = humanize.Time(client.LastActiveAt)
		client.BytesUploadedReadable = humanize.Bytes(client.BytesUploaded)
		if client.IsBanned() {
			client.BannedAt.Time = client.BannedAt.Time.UTC()
			client.BannedAtRelative = humanize.Time(client.BannedAt.Time)
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func (c *ClientDao) GetByBytesUploaded(limit int) (clients []ds.Client, err error) {
	sqlStatement := `
		SELECT
			c.ip, c.asn, c.network, c.city, c.country, c.continent, c.proxy,
			c.requests, c.first_active_at, c.last_active_at, c.banned_at, c.banned_by,
			COALESCE(COUNT(f.id), 0) as files_uploaded,
			COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
		FROM client c
		LEFT JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
		LEFT JOIN file_content fc ON fc.sha256 = f.sha256
		GROUP BY c.ip
		ORDER BY bytes_uploaded DESC, files_uploaded DESC
		LIMIT $1`
	rows, err := c.db.Query(sqlStatement, limit)
	if err != nil {
		return clients, err
	}
	defer rows.Close()
	for rows.Next() {
		var client ds.Client
		err = rows.Scan(
			&client.IP, &client.ASN, &client.Network, &client.City, &client.Country,
			&client.Continent, &client.Proxy, &client.Requests, &client.FirstActiveAt,
			&client.LastActiveAt, &client.BannedAt, &client.BannedBy,
			&client.FilesUploaded, &client.BytesUploaded,
		)
		if err != nil {
			return clients, err
		}
		client.FirstActiveAt = client.FirstActiveAt.UTC()
		client.LastActiveAt = client.LastActiveAt.UTC()
		client.FirstActiveAtRelative = humanize.Time(client.FirstActiveAt)
		client.LastActiveAtRelative = humanize.Time(client.LastActiveAt)
		client.BytesUploadedReadable = humanize.Bytes(client.BytesUploaded)
		if client.IsBanned() {
			client.BannedAt.Time = client.BannedAt.Time.UTC()
			client.BannedAtRelative = humanize.Time(client.BannedAt.Time)
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func (c *ClientDao) GetByCountry(limit int) (countries []ds.ClientByCountry, err error) {
	sqlStatement := `
		SELECT
			c.country,
			COUNT(DISTINCT c.ip) as client_count,
			SUM(c.requests) as requests,
			COALESCE(COUNT(f.id), 0) as files_uploaded,
			COALESCE(SUM(fc.bytes), 0) as bytes_uploaded,
			MIN(c.first_active_at) as first_active_at,
			MAX(c.last_active_at) as last_active_at
		FROM client c
		LEFT JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
		LEFT JOIN file_content fc ON fc.sha256 = f.sha256
		WHERE c.country != ''
		GROUP BY c.country
		ORDER BY requests DESC, files_uploaded DESC
		LIMIT $1`
	rows, err := c.db.Query(sqlStatement, limit)
	if err != nil {
		return countries, err
	}
	defer rows.Close()
	for rows.Next() {
		var country ds.ClientByCountry
		err = rows.Scan(
			&country.Country, &country.ClientCount, &country.Requests,
			&country.FilesUploaded, &country.BytesUploaded,
			&country.FirstActiveAt, &country.LastActiveAt,
		)
		if err != nil {
			return countries, err
		}
		country.FirstActiveAt = country.FirstActiveAt.UTC()
		country.LastActiveAt = country.LastActiveAt.UTC()
		country.FirstActiveAtRelative = humanize.Time(country.FirstActiveAt)
		country.LastActiveAtRelative = humanize.Time(country.LastActiveAt)
		country.BytesUploadedReadable = humanize.Bytes(country.BytesUploaded)
		countries = append(countries, country)
	}
	return countries, nil
}

func (c *ClientDao) GetByNetwork(limit int) (networks []ds.ClientByNetwork, err error) {
	sqlStatement := `
		SELECT
			c.network,
			COUNT(DISTINCT c.ip) as client_count,
			SUM(c.requests) as requests,
			COALESCE(COUNT(f.id), 0) as files_uploaded,
			COALESCE(SUM(fc.bytes), 0) as bytes_uploaded,
			MIN(c.first_active_at) as first_active_at,
			MAX(c.last_active_at) as last_active_at
		FROM client c
		LEFT JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
		LEFT JOIN file_content fc ON fc.sha256 = f.sha256
		WHERE c.network != ''
		GROUP BY c.network
		ORDER BY requests DESC, files_uploaded DESC
		LIMIT $1`
	rows, err := c.db.Query(sqlStatement, limit)
	if err != nil {
		return networks, err
	}
	defer rows.Close()
	for rows.Next() {
		var network ds.ClientByNetwork
		err = rows.Scan(
			&network.Network, &network.ClientCount, &network.Requests,
			&network.FilesUploaded, &network.BytesUploaded,
			&network.FirstActiveAt, &network.LastActiveAt,
		)
		if err != nil {
			return networks, err
		}
		network.FirstActiveAt = network.FirstActiveAt.UTC()
		network.LastActiveAt = network.LastActiveAt.UTC()
		network.FirstActiveAtRelative = humanize.Time(network.FirstActiveAt)
		network.LastActiveAtRelative = humanize.Time(network.LastActiveAt)
		network.BytesUploadedReadable = humanize.Bytes(network.BytesUploaded)
		networks = append(networks, network)
	}
	return networks, nil
}

func (c *ClientDao) GetASNWithStats(limit int) (asns []ds.AutonomousSystem, err error) {
	sqlStatement := `
		SELECT
			a.asn, a.organization, a.requests,
			a.first_active_at, a.last_active_at, a.banned_at,
			COUNT(DISTINCT cl.ip) as client_count,
			COALESCE(COUNT(f.id), 0) as files_uploaded,
			COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
		FROM autonomous_system a
		LEFT JOIN client cl ON cl.asn = a.asn
		LEFT JOIN file f ON f.ip = cl.ip AND f.deleted_at IS NULL
		LEFT JOIN file_content fc ON fc.sha256 = f.sha256
		GROUP BY a.asn, a.organization, a.requests, a.first_active_at, a.last_active_at, a.banned_at
		ORDER BY a.requests DESC, files_uploaded DESC
		LIMIT $1`
	rows, err := c.db.Query(sqlStatement, limit)
	if err != nil {
		return asns, err
	}
	defer rows.Close()
	for rows.Next() {
		var asn ds.AutonomousSystem
		err = rows.Scan(
			&asn.ASN, &asn.Organization, &asn.Requests,
			&asn.FirstActiveAt, &asn.LastActiveAt, &asn.BannedAt,
			&asn.ClientCount, &asn.FilesUploaded, &asn.BytesUploaded,
		)
		if err != nil {
			return asns, err
		}
		asn.FirstActiveAt = asn.FirstActiveAt.UTC()
		asn.LastActiveAt = asn.LastActiveAt.UTC()
		asn.FirstActiveAtRelative = humanize.Time(asn.FirstActiveAt)
		asn.LastActiveAtRelative = humanize.Time(asn.LastActiveAt)
		asn.BytesUploadedReadable = humanize.Bytes(asn.BytesUploaded)
		if asn.IsBanned() {
			asn.BannedAt.Time = asn.BannedAt.Time.UTC()
			asn.BannedAtRelative = humanize.Time(asn.BannedAt.Time)
		}
		asns = append(asns, asn)
	}
	return asns, nil
}
