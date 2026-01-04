package dbl

import (
	"fmt"
	"github.com/espebra/filebin2/ds"
	"testing"
	"time"
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

	dbBin, found, err := dao.Bin().GetByID(id)
	if err != nil {
		t.Error(err)
	}
	if found == false {
		t.Errorf("Expected found to be true as the bin exists.")
	}
	if dbBin.Id != id {
		t.Errorf("Was expecting bin id %s, got %s instead.", id, dbBin.Id)
	}
	if dbBin.Files != 0 {
		t.Errorf("Was expecting number of files to be 0, got %d\n", dbBin.Files)
	}

	err = dao.Bin().RegisterDownload(bin)
	if err != nil {
		t.Error(err)
	}
	if bin.Downloads != 1 {
		t.Errorf("Was expecting the number of downloads to be 1, not %d\n", bin.Downloads)
	}

	if bin.Bytes != 0 {
		t.Errorf("Was expecting bytes to be 0, not %d\n", bin.Bytes)
	}

	if bin.Files != 0 {
		t.Errorf("Was expecting number of files to be 0, not %d\n", bin.Files)
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

func TestBinTooLong(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Id = "1234567890123456789012345678901234567890123456789012345678901"
	err = dao.Bin().Insert(bin)
	if err == nil {
		t.Errorf("Was expecting an error here, the bin id is too long.")
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
		bin.Id = fmt.Sprintf("somebin-%d", i)
		bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
		if err := dao.Bin().Insert(bin); err != nil {
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

	dbBin, _, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}

	err = dao.Bin().Delete(&dbBin)
	if err != nil {
		t.Error(err)
	}

	_, found, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Errorf("Did not expect an error even though the bin was deleted earlier: %s\n", err.Error())
	}
	if found == true {
		t.Errorf("Expected found to be false as the bin was deleted earlier.")
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
	bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	dbBin, _, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}

	err = dao.Bin().Update(&dbBin)
	if err != nil {
		t.Error(err)
	}

	//updatedBin, err := dao.Bin().GetByID(bin.Id)
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

func TestInvalidBinInput(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	bin := &ds.Bin{}
	bin.Id = "12345"
	err = dao.Bin().Insert(bin)
	if err == nil {
		t.Error("Expected an error since bin is too short")
	}

	bin.Id = "..."
	err = dao.Bin().Insert(bin)
	if err == nil {
		t.Error("Expected an error since bin is invalid")
	}

	bin.Id = "%&/()"
	err = dao.Bin().Insert(bin)
	if err == nil {
		t.Error("Expected an error since bin contains invalid characters")
	}
}

func TestFileCount(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	type testcase struct {
		Bin   string
		Files uint64
		Bytes uint64
	}

	testcases := []testcase{
		{
			Bin:   "firstbin",
			Files: 10,
			Bytes: 1,
		}, {
			Bin:   "secondbin",
			Files: 20,
			Bytes: 2,
		}, {
			Bin:   "thirdbin",
			Files: 30,
			Bytes: 100,
		},
	}

	// Create file_content record for the default SHA256 used by test files
	defaultSHA256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	defaultContent := &ds.FileContent{
		SHA256:    defaultSHA256,
		Bytes:     0,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(defaultContent)
	if err != nil {
		t.Error(err)
	}

	for _, tc := range testcases {
		bin := &ds.Bin{}
		bin.Id = tc.Bin
		bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
		err = dao.Bin().Insert(bin)
		if err != nil {
			t.Error(err)
		}

		for i := 0; i < int(tc.Files); i++ {
			// Create some files
			file := &ds.File{}
			file.Bin = bin.Id // Foreign key
			file.Filename = fmt.Sprintf("testfile-%d", i)
			file.Bytes = tc.Bytes
			file.SHA256 = defaultSHA256 // Set SHA256 to satisfy foreign key constraint
			err = dao.File().Insert(file)
			if err != nil {
				t.Error(err)
			}
		}

		dbBin, found, err := dao.Bin().GetByID(bin.Id)
		if err != nil {
			t.Error(err)
		}
		if found == false {
			t.Errorf("Expected found to be true as the bin exists.")
		}
		if dbBin.Files != tc.Files {
			t.Errorf("Was expecting number of files in bin %s to be %d, got %d instead.\n", bin.Id, tc.Files, dbBin.Files)
		}
		if dbBin.Bytes != (tc.Bytes * tc.Files) {
			t.Errorf("Was expecting %d bytes in total in bin %s, got %d instead.\n", (tc.Bytes * tc.Files), bin.Id, dbBin.Bytes)
		}
	}

	dbBins, err := dao.Bin().GetAll()
	if err != nil {
		t.Error(err)
	}

	if len(dbBins) != len(testcases) {
		t.Errorf("Was expecting %d bins, got %d.\n", len(testcases), len(dbBins))
	}

	for _, bin := range dbBins {
		for _, tc := range testcases {
			if bin.Id == tc.Bin {
				if bin.Files != tc.Files {
					t.Errorf("Was expecting %d files in bin %s, got %d.\n", tc.Files, bin.Id, bin.Files)
				}
				if bin.Bytes != (tc.Files * tc.Bytes) {
					t.Errorf("Was expecting %d bytes in bin %s, got %d.\n", (tc.Files * tc.Bytes), bin.Id, bin.Bytes)
				}
			}
		}
	}
}
