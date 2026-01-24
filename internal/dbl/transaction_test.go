package dbl

import (
	//"fmt"
	"github.com/espebra/filebin2/internal/ds"
	"testing"
	//"time"
)

func TestGetByBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	binID := "1234567890"
	bin := &ds.Bin{}
	bin.Id = binID
	err = dao.Bin().Insert(bin)
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
