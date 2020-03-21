package dbl

import (
	//"errors"
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

	id := "1234567890"
	bin := &ds.Bin{}
	bin.Id = id
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}
	if bin.Id != id {
		t.Errorf("Was expecting bin id %s, got %s instead.", id, bin.Id)
	}

	dbBin, err := dao.Bin().GetById(id)
	if err != nil {
		t.Error(err)
	}
	if dbBin.Id != id {
		t.Errorf("Was expecting bin id %s, got %s instead.", id, dbBin.Id)
	}

	err = dao.Bin().RegisterDownload(bin)
	if err != nil {
		t.Error(err)
	}
	if bin.Downloads != 1 {
		t.Errorf("Was expecting the number of downloads to be 1, not %d\n", bin.Downloads)
	}
}

func TestInsertDuplicatedBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Id = "1234567890"
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
		bin.Id = fmt.Sprintf("bid-%d", i)
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
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	dbBin, err := dao.Bin().GetById(bin.Id)
	if err != nil {
		t.Error(err)
	}

	err = dao.Bin().Delete(&dbBin)
	if err != nil {
		t.Error(err)
	}

	_, err = dao.Bin().GetById(bin.Id)
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
	bin.Id = "1234567890"
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
	bin.Id = "1234567890"
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
	bin.Id = "1234567890"
	err = dao.Bin().Delete(bin)
	if err == nil {
		t.Errorf("Was expecting an error here, bin %v does not exist.", bin)
	}
}
