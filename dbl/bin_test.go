package dbl

import (
	"errors"
	"fmt"
	"github.com/espebra/filebin2/ds"
	"testing"
)

func TestGetBinById(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bid := "1234567890"
	bin := &ds.Bin{}
	bin.Bid = bid
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}
	if bin.Id == 0 {
		t.Error(errors.New("Expected id > 0"))
	}

	dbBin, err := dao.Bin().GetByBid(bid)
	if err != nil {
		t.Error(err)
	}
	if dbBin.Bid != bid {
		t.Errorf("Was expecting bid %s, got %s instead.", bid, dbBin.Bid)
	}
}

func TestInsertDuplicatedBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	err = dao.Bin().Insert(bin)
	if err == nil {
		t.Errorf("Was expecting an error here, cannot insert the same bid twice.")
	}
}

func TestGetAllBins(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	count := 50
	for i := 0; i < count; i++ {
		bin := &ds.Bin{}
		bin.Bid = fmt.Sprintf("bid-%d", i)
		err = dao.Bin().Insert(bin)
		if err != nil {
			t.Error(err)
			break
		}
	}

	bins, err := dao.Bin().GetAll()
	if err != nil {
		t.Error(err)
	}

	if len(bins) != count {
		t.Errorf("Was expecting to find %d bins, got %d instead.", count, len(bins))
	}
}

func TestDeleteBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	dbBin, err := dao.Bin().GetByBid(bin.Bid)
	if err != nil {
		t.Error(err)
	}

	err = dao.Bin().Delete(&dbBin)
	if err != nil {
		t.Error(err)
	}

	_, err = dao.Bin().GetByBid(bin.Bid)
	if err == nil {
		t.Errorf("Was expecting an error here, the bin was deleted earlier.")
	}
}

func TestUpdateBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	dbBin, err := dao.Bin().GetById(bin.Id)
	if err != nil {
		t.Error(err)
	}

	err = dao.Bin().Update(&dbBin)
	if err != nil {
		t.Error(err)
	}

	//updatedBin, err := dao.Bin().GetById(bin.Id)
	//if err != nil {
	//	t.Error(err)
	//}
	//if updatedBin.Foo != "bar" {
	//	t.Errorf("Was expecting the updated bin hostname %s, got %s instead.", "bar", updatedBin.Foo)
	//}
}

func TestUpdateNonExistingBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Id = 5
	bin.Bid = "1234567890"
	err = dao.Bin().Update(bin)
	if err == nil {
		t.Errorf("Was expecting an error here, bin %v does not exist.", bin)
	}
}

func TestDeleteNonExistingBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Id = 10
	bin.Bid = "1234567890"
	err = dao.Bin().Delete(bin)
	if err == nil {
		t.Errorf("Was expecting an error here, bin %v does not exist.", bin)
	}
}
