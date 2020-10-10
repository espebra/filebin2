package dbl

import (
	//"fmt"
	"github.com/espebra/filebin2/ds"
	"testing"
	//"time"
)

func TestGetByBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin_id := "1234567890"
	bin := &ds.Bin{}
	bin.Id = bin_id
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	tr := &ds.Transaction{}
	tr.Bin = bin_id
	tr.Method = "GET"
	tr.Path = "/foo/bar"
	tr.IP = "1.2.3.4"
	tr.Trace = "trace"
	err = dao.Transaction().Insert(tr)
	if err != nil {
		t.Error(err)
	}
	if tr.Id == 0 {
		t.Errorf("Was expecting bin id != 0, got %d.\n", tr.Id)
	}

	if tr.Trace != "trace" {
		t.Errorf("Trace was unexpected: %s\n", tr.Trace)
	}

	tr.Trace = "trace2"
	err = dao.Transaction().Update(tr)
	if err != nil {
		t.Error(err)
	}

	if tr.Trace != "trace2" {
		t.Errorf("Trace was unexpected: %s\n", tr.Trace)
	}

	trs, err := dao.Transaction().GetByBin(tr.Bin)
	if err != nil {
		t.Error(err)
	}
	if len(trs) != 1 {
		t.Errorf("Was expecting one transaction, got %d.", len(trs))
	}
}
