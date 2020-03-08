package dbl

import (
	"errors"
	"fmt"
	"github.com/espebra/filebin2/ds"
	"testing"
)

func TestGetFileById(t *testing.T) {
	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	// Create file
	file := &ds.File{}
	file.Filename = "testfile.txt"
	file.BinId = bin.Id // Foreign key
	file.Size = 1
	file.Checksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	if file.Id == 0 {
		t.Error(errors.New("Expected id > 0"))
	}

	dbFile, err := dao.File().GetById(file.Id)

	if err != nil {
		t.Error(err)
	}

	if dbFile.Filename != "testfile.txt" {
		t.Errorf("Was expecting name testfile.txt, got %s instead.", dbFile.Filename)
	}

	if dbFile.Size != 1 {
		t.Errorf("Was expecting size 1, got %d instead.", dbFile.Size)
	}

	if dbFile.Checksum != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Was expecting checksum e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, got %s instead.", dbFile.Checksum)
	}
}

func TestInsertDuplicatedFile(t *testing.T) {
	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	file := &ds.File{}
	file.Filename = "testfile.txt"
	file.BinId = bin.Id
	file.Size = 1
	file.Checksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	_, err = dao.File().GetById(file.Id)

	if err != nil {
		t.Error(err)
	}

	err = dao.File().Insert(file)

	if err == nil {
		t.Errorf("Was expecting an error here, cannot insert the same file name twice.")
	}
}

func TestGetAllFiles(t *testing.T) {
	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	count := 50

	for i := 0; i < count; i++ {
		file := &ds.File{}
		file.Filename = fmt.Sprintf("File-%d", i)
		file.BinId = bin.Id
		file.Size = 1
		file.Checksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		err = dao.File().Insert(file)

		if err != nil {
			t.Error(err)
			break
		}
	}

	files, err := dao.File().GetAll()

	if err != nil {
		t.Error(err)
	}

	if len(files) != count {
		t.Errorf("Was expecting to find %d files, got %d instead.", count, len(files))
	}
}

func TestDeleteFile(t *testing.T) {
	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	file := &ds.File{}
	file.Filename = "testfile.txt"
	file.BinId = bin.Id
	file.Size = 1
	file.Checksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	dbFile, err := dao.File().GetById(file.Id)

	if err != nil {
		t.Error(err)
	}

	err = dao.File().Delete(&dbFile)

	if err != nil {
		t.Error(err)
	}

	_, err = dao.File().GetById(file.Id)

	if err == nil {
		t.Errorf("Was expecting an error here, the file was deleted earlier.")
	}
}

func TestUpdateFile(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	file := &ds.File{}
	file.BinId = bin.Id
	file.Filename = "testfile.txt"
	file.Size = 1
	file.Checksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	err = dao.File().Insert(file)
	if err != nil {
		t.Error(err)
	}

	dbFile, err := dao.File().GetById(file.Id)
	if err != nil {
		t.Error(err)
	}

	dbFile.Size = 2
	dbFile.Checksum = "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b"
	err = dao.File().Update(&dbFile)
	if err != nil {
		t.Error(err)
	}

	updatedFile, err := dao.File().GetById(file.Id)
	if err != nil {
		t.Error(err)
	}
	if updatedFile.Size != 2 {
		t.Errorf("Was expecting the updated file size 2, got %d instead.", updatedFile.Size)
	}
	if updatedFile.Checksum != "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b" {
		t.Errorf("Was expecting the updated file checksum ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b, got %s instead.", updatedFile.Checksum)
	}
}

func TestUpdateNonExistingFile(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	file := &ds.File{}
	file.Id = 5
	file.Filename = "foo"
	err = dao.File().Update(file)
	if err == nil {
		t.Errorf("Was expecting an error here, file %v does not exist.", file)
	}
}

func TestDeleteNonExistingFile(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	file := &ds.File{}
	file.Id = 10
	file.Filename = "foo"
	err = dao.File().Delete(file)
	if err == nil {
		t.Errorf("Was expecting an error here, file %v does not exist.", file)
	}
}

func TestGetFilesByBinId(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Bid = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	// Create files
	file1 := &ds.File{}
	file1.Filename = "file1.txt"
	file1.BinId = bin.Id // Foreign key
	file1.Size = 1
	file1.Checksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	err = dao.File().Insert(file1)
	if err != nil {
		t.Error(err)
	}

	file2 := &ds.File{}
	file2.Filename = "file2.txt"
	file2.BinId = bin.Id // Foreign key
	file2.Size = 2
	file2.Checksum = "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b"
	err = dao.File().Insert(file2)
	if err != nil {
		t.Error(err)
	}

	files, err := dao.File().GetByBinId(bin.Id)
	if err != nil {
		t.Error(err)
	}
	if len(files) != 2 {
		t.Errorf("Was expecting two files, got %d instead.", len(files))
	}

	files, err = dao.File().GetByBinId(-1)
	if err != nil {
		t.Error("Did not expect an error even though we asked for a bin id that does not exist")
	}
	if len(files) != 0 {
		t.Errorf("Was expecting zero files matching the non existent bin id, got %d instead.", len(files))
	}
}