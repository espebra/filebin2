package dbl

import (
	//"fmt"
	"github.com/espebra/filebin2/internal/ds"
	"testing"
	"time"
)

func TestGetByBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	binID := "1234567890"
	bin := &ds.Bin{}
	bin.Id = binID
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	tr := &ds.Transaction{}
	tr.BinId = binID
	tr.Method = "GET"
	tr.Path = "/foo/bar"
	tr.IP = "1.2.3.4"
	tr.Headers = "headers"
	err = dao.Transaction().Insert(tr)
	if err != nil {
		t.Error(err)
	}
	if tr.Id == 0 {
		t.Errorf("Was expecting bin id != 0, got %d.\n", tr.Id)
	}

	if tr.Headers != "headers" {
		t.Errorf("Headers was unexpected: %s\n", tr.Headers)
	}

	tr.Headers = "headers2"
	err = dao.Transaction().Update(tr)
	if err != nil {
		t.Error(err)
	}

	if tr.Headers != "headers2" {
		t.Errorf("Headers was unexpected: %s\n", tr.Headers)
	}

	trs, err := dao.Transaction().GetByBin(tr.BinId)
	if err != nil {
		t.Error(err)
	}
	if len(trs) != 1 {
		t.Errorf("Was expecting one transaction, got %d.", len(trs))
	}
}

func TestGetTransfersByBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tearDown(dao) }()

	binID := "transfersbin"
	bin := &ds.Bin{}
	bin.Id = binID
	if _, err := dao.Bin().Insert(bin); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	transactions := []ds.Transaction{
		{Method: "POST", Path: "/transfersbin/a.txt", Filename: "a.txt"},
		{Method: "GET", Path: "/transfersbin/a.txt", Filename: "a.txt"},
		{Method: "GET", Path: "/archive/transfersbin/zip"},
		{Method: "PUT", Path: "/transfersbin"},
		{Method: "DELETE", Path: "/transfersbin/a.txt", Filename: "a.txt"},
		{Method: "POST", Path: "/admin/bin/transfersbin/ban"},
	}
	for i := range transactions {
		tr := &transactions[i]
		tr.BinId = binID
		tr.IP = "192.0.2.10"
		tr.Headers = "GET / HTTP/1.1\r\nUser-Agent: test\r\n\r\n"
		tr.Timestamp = now.Add(time.Duration(i) * time.Second)
		tr.CompletedAt = tr.Timestamp.Add(250 * time.Millisecond)
		if err := dao.Transaction().Insert(tr); err != nil {
			t.Fatal(err)
		}
	}

	transfers, err := dao.Transaction().GetTransfersByBin(binID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"upload", "download", "archive"}
	if len(transfers) != len(want) {
		t.Fatalf("Expected %d transfers, got %d: %+v", len(want), len(transfers), transfers)
	}
	for i, w := range want {
		if transfers[i].Type != w {
			t.Errorf("Expected transfer %d to be a %s, got %s", i, w, transfers[i].Type)
		}
		if transfers[i].Duration != 250*time.Millisecond {
			t.Errorf("Expected transfer %d to take 250ms, got %s", i, transfers[i].Duration)
		}
	}

	// Transfers from clients that are not in the client table are included
	if transfers[0].Client.Country != "" || transfers[0].Client.ASN != 0 {
		t.Errorf("Expected empty client information, got %+v", transfers[0].Client)
	}
}
