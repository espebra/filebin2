package dbl

import (
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/espebra/filebin2/internal/ds"

	"github.com/dustin/go-humanize"
)

type ClientDao struct {
	db      *sql.DB
	metrics DBMetricsObserver
}

func (c *ClientDao) GetByIP(ip net.IP) (client ds.Client, found bool, err error) {
	sqlStatement := "SELECT ip, asn, asn_organization, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client WHERE ip = $1 LIMIT 1"
	t0 := time.Now()
	err = c.db.QueryRow(sqlStatement, ip.String()).Scan(&client.IP, &client.ASN, &client.ASNOrganization, &client.Network, &client.City, &client.Country, &client.Continent, &client.Proxy, &client.Requests, &client.FirstActiveAt, &client.LastActiveAt, &client.BannedAt, &client.BannedBy)
	observeQuery(c.metrics, "client_get_by_ip", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			client.IP = ip.String()
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
		return client, false, fmt.Errorf("unable to parse remote addr %s", remoteAddr)
	}

	// Look up the client by IP
	client, found, err = c.GetByIP(ip)
	return client, found, err
}

func (c *ClientDao) Update(client *ds.Client) (err error) {
	now := time.Now().UTC()
	sqlStatement := "INSERT INTO client (ip, asn, asn_organization, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, '') ON CONFLICT(ip) DO UPDATE SET asn=$12, asn_organization=$13, network=$14, city=$15, country=$16, continent=$17, proxy=$18, requests=client.requests+1, last_active_at=$19 RETURNING ip, requests"
	t0 := time.Now()
	err = c.db.QueryRow(sqlStatement, client.IP, client.ASN, client.ASNOrganization, client.Network, client.City, client.Country, client.Continent, client.Proxy, 1, now, now, client.ASN, client.ASNOrganization, client.Network, client.City, client.Country, client.Continent, client.Proxy, now).Scan(&client.IP, &client.Requests)
	observeQuery(c.metrics, "client_update", t0, err)
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientDao) GetAll() (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, asn_organization, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client ORDER BY last_active_at DESC"
	clients, err = c.clientQuery(sqlStatement)
	return clients, err
}

func (c *ClientDao) GetByLastActiveAt(limit int) (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, asn_organization, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client ORDER BY last_active_at DESC LIMIT $1"
	clients, err = c.clientQuery(sqlStatement, limit)
	return clients, err
}

func (c *ClientDao) GetByRequests(limit int) (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, asn_organization, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client ORDER BY requests DESC LIMIT $1"
	clients, err = c.clientQuery(sqlStatement, limit)
	return clients, err
}

func (c *ClientDao) GetByBannedAt(limit int) (clients []ds.Client, err error) {
	sqlStatement := "SELECT ip, asn, asn_organization, network, city, country, continent, proxy, requests, first_active_at, last_active_at, banned_at, banned_by FROM client WHERE banned_at > $1 ORDER BY banned_at DESC LIMIT $2"
	clients, err = c.clientQuery(sqlStatement, time.Unix(0, 0), limit)
	return clients, err
}

func (c *ClientDao) clientQuery(sqlStatement string, params ...interface{}) (clients []ds.Client, err error) {
	t0 := time.Now()
	rows, err := c.db.Query(sqlStatement, params...)
	observeQuery(c.metrics, "client_query", t0, err)
	if err != nil {
		return clients, err
	}
	defer rows.Close()
	for rows.Next() {
		var client ds.Client
		err = rows.Scan(&client.IP, &client.ASN, &client.ASNOrganization, &client.Network, &client.City, &client.Country, &client.Continent, &client.Proxy, &client.Requests, &client.FirstActiveAt, &client.LastActiveAt, &client.BannedAt, &client.BannedBy)
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
	if err = rows.Err(); err != nil {
		return clients, err
	}
	return clients, nil
}

func (c *ClientDao) Ban(IPsToBan []string, banByRemoteAddr string) (err error) {
	// Loop over the IP addresses that will be banned
	now := time.Now().UTC()
	sqlStatement := "UPDATE client SET banned_at=$1, banned_by=$2 WHERE ip=$3 RETURNING ip"
	var ret string
	for _, ipToBan := range IPsToBan {
		t0 := time.Now()
		err = c.db.QueryRow(sqlStatement, now, banByRemoteAddr, ipToBan).Scan(&ret)
		observeQuery(c.metrics, "client_ban", t0, err)
		if err != nil {
			return err
		}
	}
	return err
}

func (c *ClientDao) Cleanup(days uint64) (count int64, err error) {
	sqlStatement := "DELETE FROM client WHERE last_active_at < CURRENT_DATE - ($1 || ' days')::interval AND banned_at IS NULL"
	t0 := time.Now()
	res, err := c.db.Exec(sqlStatement, days)
	observeQuery(c.metrics, "client_cleanup", t0, err)
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
			c.ip, c.asn, c.asn_organization, c.network, c.city, c.country, c.continent, c.proxy,
			c.requests, c.first_active_at, c.last_active_at, c.banned_at, c.banned_by,
			COALESCE(COUNT(f.id), 0) as files_uploaded,
			COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
		FROM client c
		LEFT JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
		LEFT JOIN file_content fc ON fc.sha256 = f.sha256
		GROUP BY c.ip
		ORDER BY files_uploaded DESC, bytes_uploaded DESC
		LIMIT $1`
	t0 := time.Now()
	rows, err := c.db.Query(sqlStatement, limit)
	observeQuery(c.metrics, "client_get_by_files_uploaded", t0, err)
	if err != nil {
		return clients, err
	}
	defer rows.Close()
	for rows.Next() {
		var client ds.Client
		err = rows.Scan(
			&client.IP, &client.ASN, &client.ASNOrganization, &client.Network, &client.City, &client.Country,
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
	if err = rows.Err(); err != nil {
		return clients, err
	}
	return clients, nil
}

func (c *ClientDao) GetByBytesUploaded(limit int) (clients []ds.Client, err error) {
	sqlStatement := `
		SELECT
			c.ip, c.asn, c.asn_organization, c.network, c.city, c.country, c.continent, c.proxy,
			c.requests, c.first_active_at, c.last_active_at, c.banned_at, c.banned_by,
			COALESCE(COUNT(f.id), 0) as files_uploaded,
			COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
		FROM client c
		LEFT JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
		LEFT JOIN file_content fc ON fc.sha256 = f.sha256
		GROUP BY c.ip
		ORDER BY bytes_uploaded DESC, files_uploaded DESC
		LIMIT $1`
	t0 := time.Now()
	rows, err := c.db.Query(sqlStatement, limit)
	observeQuery(c.metrics, "client_get_by_bytes_uploaded", t0, err)
	if err != nil {
		return clients, err
	}
	defer rows.Close()
	for rows.Next() {
		var client ds.Client
		err = rows.Scan(
			&client.IP, &client.ASN, &client.ASNOrganization, &client.Network, &client.City, &client.Country,
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
	if err = rows.Err(); err != nil {
		return clients, err
	}
	return clients, nil
}

func (c *ClientDao) GetByCountry(limit int) (countries []ds.ClientByCountry, err error) {
	sqlStatement := `
		WITH client_stats AS (
			SELECT
				country,
				COUNT(DISTINCT ip) as client_count,
				SUM(requests) as requests,
				MIN(first_active_at) as first_active_at,
				MAX(last_active_at) as last_active_at
			FROM client
			WHERE country != ''
			GROUP BY country
		),
		file_stats AS (
			SELECT
				c.country,
				COUNT(f.id) as files_uploaded,
				COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
			FROM client c
			JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
			JOIN file_content fc ON fc.sha256 = f.sha256
			WHERE c.country != ''
			GROUP BY c.country
		)
		SELECT
			cs.country,
			cs.client_count,
			cs.requests,
			COALESCE(fs.files_uploaded, 0) as files_uploaded,
			COALESCE(fs.bytes_uploaded, 0) as bytes_uploaded,
			cs.first_active_at,
			cs.last_active_at
		FROM client_stats cs
		LEFT JOIN file_stats fs ON fs.country = cs.country
		ORDER BY cs.requests DESC, COALESCE(fs.files_uploaded, 0) DESC
		LIMIT $1`
	t0 := time.Now()
	rows, err := c.db.Query(sqlStatement, limit)
	observeQuery(c.metrics, "client_get_by_country", t0, err)
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
	if err = rows.Err(); err != nil {
		return countries, err
	}
	return countries, nil
}

func (c *ClientDao) GetByNetwork(limit int) (networks []ds.ClientByNetwork, err error) {
	sqlStatement := `
		WITH client_stats AS (
			SELECT
				network,
				COUNT(DISTINCT ip) as client_count,
				SUM(requests) as requests,
				MIN(first_active_at) as first_active_at,
				MAX(last_active_at) as last_active_at
			FROM client
			WHERE network != ''
			GROUP BY network
		),
		file_stats AS (
			SELECT
				c.network,
				COUNT(f.id) as files_uploaded,
				COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
			FROM client c
			JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
			JOIN file_content fc ON fc.sha256 = f.sha256
			WHERE c.network != ''
			GROUP BY c.network
		)
		SELECT
			cs.network,
			cs.client_count,
			cs.requests,
			COALESCE(fs.files_uploaded, 0) as files_uploaded,
			COALESCE(fs.bytes_uploaded, 0) as bytes_uploaded,
			cs.first_active_at,
			cs.last_active_at
		FROM client_stats cs
		LEFT JOIN file_stats fs ON fs.network = cs.network
		ORDER BY cs.requests DESC, COALESCE(fs.files_uploaded, 0) DESC
		LIMIT $1`
	t0 := time.Now()
	rows, err := c.db.Query(sqlStatement, limit)
	observeQuery(c.metrics, "client_get_by_network", t0, err)
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
	if err = rows.Err(); err != nil {
		return networks, err
	}
	return networks, nil
}

func (c *ClientDao) GetASNWithStats(limit int) (asns []ds.AutonomousSystem, err error) {
	sqlStatement := `
		WITH client_stats AS (
			SELECT
				asn,
				MAX(asn_organization) as asn_organization,
				COUNT(DISTINCT ip) as client_count,
				SUM(requests) as requests,
				MIN(first_active_at) as first_active_at,
				MAX(last_active_at) as last_active_at
			FROM client
			WHERE asn > 0
			GROUP BY asn
		),
		file_stats AS (
			SELECT
				c.asn,
				COUNT(f.id) as files_uploaded,
				COALESCE(SUM(fc.bytes), 0) as bytes_uploaded
			FROM client c
			JOIN file f ON f.ip = c.ip AND f.deleted_at IS NULL
			JOIN file_content fc ON fc.sha256 = f.sha256
			WHERE c.asn > 0
			GROUP BY c.asn
		)
		SELECT
			cs.asn,
			cs.asn_organization,
			cs.client_count,
			cs.requests,
			COALESCE(fs.files_uploaded, 0) as files_uploaded,
			COALESCE(fs.bytes_uploaded, 0) as bytes_uploaded,
			cs.first_active_at,
			cs.last_active_at
		FROM client_stats cs
		LEFT JOIN file_stats fs ON fs.asn = cs.asn
		ORDER BY cs.requests DESC, COALESCE(fs.files_uploaded, 0) DESC
		LIMIT $1`
	t0 := time.Now()
	rows, err := c.db.Query(sqlStatement, limit)
	observeQuery(c.metrics, "client_get_asn_with_stats", t0, err)
	if err != nil {
		return asns, err
	}
	defer rows.Close()
	for rows.Next() {
		var asn ds.AutonomousSystem
		err = rows.Scan(
			&asn.ASN, &asn.Organization, &asn.ClientCount, &asn.Requests,
			&asn.FilesUploaded, &asn.BytesUploaded,
			&asn.FirstActiveAt, &asn.LastActiveAt,
		)
		if err != nil {
			return asns, err
		}
		asn.FirstActiveAt = asn.FirstActiveAt.UTC()
		asn.LastActiveAt = asn.LastActiveAt.UTC()
		asn.FirstActiveAtRelative = humanize.Time(asn.FirstActiveAt)
		asn.LastActiveAtRelative = humanize.Time(asn.LastActiveAt)
		asn.BytesUploadedReadable = humanize.Bytes(asn.BytesUploaded)
		asns = append(asns, asn)
	}
	if err = rows.Err(); err != nil {
		return asns, err
	}
	return asns, nil
}
