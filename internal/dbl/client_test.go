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
	defer tearDown(dao)

	ip := "1.2.3.4"
	client := &ds.Client{}
	client.IP = ip
	err = dao.Client().Update(client)
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

	err = dao.Client().Update(&dbClient)
	if err != nil {
		t.Error(err)
	}
	if dbClient.Requests != 2 {
		t.Errorf("Was expecting the number of requests to be 2, not %d\n", dbClient.Requests)
	}

	dbClient2, found, err := dao.Client().GetByRemoteAddr(ip)
	if err != nil {
		t.Error(err)
	}
	err = dao.Client().Update(&dbClient2)
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
	defer tearDown(dao)

	ips := []string{"192.168.0.1", "192.168.0.2", "10.0.0.10"}
	for _, ip := range ips {
		client := &ds.Client{}
		client.IP = ip
		err = dao.Client().Update(client)
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
