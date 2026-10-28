package dbl

import (
	//"fmt"
	"github.com/espebra/filebin2/internal/ds"
	"testing"
	//"time"
	"net"
)

func TestGetClientByIP(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	ip := "1.2.3.4"
	client := &ds.Client{}
	client.IP = ip
	_, err = dao.Client().RecordActivity(client)
	if err != nil {
		t.Error(err)
	}
	if client.IP != ip {
		t.Errorf("Was expecting client ip %s, got %s instead.", ip, client.IP)
	}

	dbClient, found, err := dao.Client().GetByIP(net.ParseIP(ip))
	if err != nil {
		t.Error(err)
	}
	if found == false {
		t.Errorf("Expected found to be true as the client exists.")
	}
	if dbClient.IP != ip {
		t.Errorf("Was expecting client ip %s, got %s instead.", ip, dbClient.IP)
	}
	if dbClient.Requests != 1 {
		t.Errorf("Was expecting number of requests to be 1, got %d\n", dbClient.Requests)
	}

	_, err = dao.Client().RecordActivity(&dbClient)
	if err != nil {
		t.Error(err)
	}
	if dbClient.Requests != 2 {
		t.Errorf("Was expecting the number of requests to be 2, not %d\n", dbClient.Requests)
	}

	dbClient2, _, err := dao.Client().GetByRemoteAddr(ip)
	if err != nil {
		t.Error(err)
	}
	_, err = dao.Client().RecordActivity(&dbClient2)
	if err != nil {
		t.Error(err)
	}
	if dbClient2.Requests != 3 {
		t.Errorf("Was expecting number of requests to be 3, got %d\n", dbClient2.Requests)
	}
}

func TestClientsGetAll(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	ips := []string{"192.168.0.1", "192.168.0.2", "10.0.0.10"}
	for _, ip := range ips {
		client := &ds.Client{}
		client.IP = ip
		_, err = dao.Client().RecordActivity(client)
		if err != nil {
			t.Error(err)
		}
	}

	clients, err := dao.Client().GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(clients) != len(ips) {
		t.Errorf("Was expecting %d clients, got %d\n", len(ips), len(clients))
	}
}

func TestClientBanUnban(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	ip := "5.6.7.8"
	client := &ds.Client{}
	client.IP = ip
	_, err = dao.Client().RecordActivity(client)
	if err != nil {
		t.Error(err)
	}

	err = dao.Client().Ban([]string{ip}, "admin")
	if err != nil {
		t.Error(err)
	}
	dbClient, _, err := dao.Client().GetByIP(net.ParseIP(ip))
	if err != nil {
		t.Error(err)
	}
	if !dbClient.IsBanned() {
		t.Errorf("Was expecting client %s to be banned\n", ip)
	}

	err = dao.Client().Unban(ip)
	if err != nil {
		t.Error(err)
	}
	dbClient, _, err = dao.Client().GetByIP(net.ParseIP(ip))
	if err != nil {
		t.Error(err)
	}
	if dbClient.IsBanned() {
		t.Errorf("Was expecting client %s to not be banned\n", ip)
	}

	// Unbanning an unknown IP is a no-op
	err = dao.Client().Unban("9.9.9.9")
	if err != nil {
		t.Error(err)
	}
}

func TestClientBanWithoutRecord(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	// An existing client keeps its details when banned
	known := &ds.Client{IP: "10.0.0.1", ASN: 64500, Country: "Norway", City: "Oslo"}
	if _, err := dao.Client().RecordActivity(known); err != nil {
		t.Fatal(err)
	}

	// Ban the known client together with clients that have no record
	ips := []string{"10.0.0.1", "10.0.0.2", "2001:db8::1"}
	if err := dao.Client().Ban(ips, "192.0.2.1"); err != nil {
		t.Fatalf("Was expecting the ban to succeed, got %s", err)
	}

	for _, ip := range ips {
		client, found, err := dao.Client().GetByIP(net.ParseIP(ip))
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Errorf("Was expecting a client record for %s", ip)
		}
		if !client.IsBanned() {
			t.Errorf("Was expecting client %s to be banned", ip)
		}
		if client.BannedBy != "192.0.2.1" {
			t.Errorf("Was expecting client %s to be banned by 192.0.2.1, got %q", ip, client.BannedBy)
		}
	}

	client, _, err := dao.Client().GetByIP(net.ParseIP("10.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	if client.ASN != 64500 || client.Country != "Norway" || client.City != "Oslo" || client.Requests != 1 {
		t.Errorf("Was expecting the details of the known client to be kept, got %+v", client)
	}

	client, _, err = dao.Client().GetByIP(net.ParseIP("10.0.0.2"))
	if err != nil {
		t.Fatal(err)
	}
	if client.Requests != 0 {
		t.Errorf("Was expecting a new client record to have 0 requests, got %d", client.Requests)
	}

	// The lurker keeps banned clients without recent activity
	if _, err := dao.Client().Cleanup(0); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := dao.Client().GetByIP(net.ParseIP("10.0.0.2")); !found {
		t.Errorf("Was expecting the banned client to be kept by the cleanup")
	}

	// Banning the same IP address twice in one call works
	if err := dao.Client().Ban([]string{"10.0.0.3", "10.0.0.3"}, "192.0.2.1"); err != nil {
		t.Errorf("Was expecting a ban with duplicate IP addresses to succeed, got %s", err)
	}
}

// Requests from banned clients are reported and not counted.
func TestRecordActivityOfBannedClient(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tearDown(dao) }()

	ip := "10.0.0.3"
	client := &ds.Client{IP: ip}
	if banned, err := dao.Client().RecordActivity(client); err != nil || banned {
		t.Fatalf("Was expecting a new client not to be banned: banned=%t err=%v", banned, err)
	}
	if err := dao.Client().Ban([]string{ip}, "192.0.2.1"); err != nil {
		t.Fatal(err)
	}

	banned, err := dao.Client().RecordActivity(&ds.Client{IP: ip})
	if err != nil {
		t.Fatal(err)
	}
	if !banned {
		t.Error("Was expecting the client to be reported as banned")
	}
	dbClient, _, err := dao.Client().GetByIP(net.ParseIP(ip))
	if err != nil {
		t.Fatal(err)
	}
	if dbClient.Requests != 1 {
		t.Errorf("Was expecting the request from the banned client not to be counted, got %d requests", dbClient.Requests)
	}

	if err := dao.Client().Unban(ip); err != nil {
		t.Fatal(err)
	}
	if banned, err := dao.Client().RecordActivity(&ds.Client{IP: ip}); err != nil || banned {
		t.Fatalf("Was expecting an unbanned client not to be banned: banned=%t err=%v", banned, err)
	}
}
