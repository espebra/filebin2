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
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	// Create file
	file := &ds.File{}
	file.Filename = "testfile.txt"
	file.Bin = bin.Id // Foreign key
	file.Bytes = 1
	file.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	if file.Id == 0 {
		t.Error(errors.New("Expected id > 0"))
	}

	dbFile, found, err := dao.File().GetById(file.Id)
	if err != nil {
		t.Error(err)
	}

	if found == false {
		t.Errorf("Expected found to be true")
	}

	if dbFile.Filename != "testfile.txt" {
		t.Errorf("Was expecting name testfile.txt, got %s instead.", dbFile.Filename)
	}

	if dbFile.Bytes != 1 {
		t.Errorf("Was expecting bytes 1, got %d instead.", dbFile.Bytes)
	}

	if dbFile.SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Was expecting checksum e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, got %s instead.", dbFile.SHA256)
	}
	if dbFile.IP != "N/A" {
		t.Errorf("Was expecting default value for IP, got %s instead.", dbFile.IP)
	}
	if dbFile.Trace != "N/A" {
		t.Errorf("Was expecting default value for trace, got %s instead.", dbFile.Trace)
	}

	dbBin, found, err := dao.Bin().GetById(dbFile.Bin)
	if err != nil {
		t.Error(err)
	}
	if found == false {
		t.Errorf("Expected found to be true")
	}
	if dbBin.Bytes != dbFile.Bytes {
		t.Errorf("Expecting the same size in bin (%d) and file (%d)", dbBin.Bytes, dbFile.Bytes)
	}
	if dbBin.BytesReadable == "" {
		t.Errorf("Expected to get human readable bytes")
	}
}

func TestGetFileByName(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	// Create file
	file := &ds.File{}
	file.Filename = "testfile.txt"
	file.Bin = bin.Id // Foreign key
	file.Bytes = 1
	file.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	file.IP = "127.0.0.1"
	file.Trace = "some trace"
	err = dao.File().Insert(file)
	if err != nil {
		t.Error(err)
	}

	if file.Id == 0 {
		t.Error(errors.New("Expected id > 0"))
	}

	dbFile, found, err := dao.File().GetByName(bin.Id, file.Filename)
	if err != nil {
		t.Error(err)
	}

	if found == false {
		t.Errorf("Expected found to be true")
	}
	if dbFile.Filename != "testfile.txt" {
		t.Errorf("Was expecting name testfile.txt, got %s instead.", dbFile.Filename)
	}
	if dbFile.Bytes != 1 {
		t.Errorf("Was expecting bytes 1, got %d instead.", dbFile.Bytes)
	}
	if dbFile.SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Was expecting checksum e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, got %s instead.", dbFile.SHA256)
	}
	if dbFile.IP != "127.0.0.1" {
		t.Errorf("Was expecting IP 127.0.0.1, got %s instead.", dbFile.IP)
	}
	if dbFile.Trace != "some trace" {
		t.Errorf("Was expecting trace 'some trace', got %s instead.", dbFile.Trace)
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
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	file := &ds.File{}
	file.Filename = "testfile.txt"
	file.Bin = bin.Id
	file.Bytes = 1
	file.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	_, _, err = dao.File().GetById(file.Id)

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
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	count := 50

	for i := 0; i < count; i++ {
		file := &ds.File{}
		file.Filename = fmt.Sprintf("File-%d", i)
		file.Bin = bin.Id
		file.Bytes = 1
		file.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		err = dao.File().Insert(file)

		if err != nil {
			t.Error(err)
			break
		}
	}

	files, err := dao.File().GetAll(0)
	if err != nil {
		t.Error(err)
	}

	if len(files) != count {
		t.Errorf("Was expecting to find %d files, got %d instead.", count, len(files))
	}

	dbBin, _, err := dao.Bin().GetById(bin.Id)
	if err != nil {
		t.Error(err)
	}
	if dbBin.Bytes != 50 {
		t.Errorf("Was expecting to get 50 bytes, got %d.", dbBin.Bytes)
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
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	file := &ds.File{}
	file.Filename = "testfile.txt"
	file.Bin = bin.Id
	file.Bytes = 1
	file.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	dbFile, _, err := dao.File().GetById(file.Id)

	if err != nil {
		t.Error(err)
	}

	err = dao.File().Delete(&dbFile)

	if err != nil {
		t.Error(err)
	}

	_, found, err := dao.File().GetById(file.Id)
	if err != nil {
		t.Errorf("Did not expect an error even though the file was deleted: " + err.Error())
	}
	if found == true {
		t.Errorf("expected found to be false as the file was deleted earlier.")
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
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	file := &ds.File{}
	file.Bin = bin.Id
	file.Filename = "testfile.txt"
	file.Bytes = 1
	file.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	file.IP = "127.0.0.1"
	file.Trace = "first trace"
	err = dao.File().Insert(file)
	if err != nil {
		t.Error(err)
	}

	dbFile, _, err := dao.File().GetById(file.Id)
	if err != nil {
		t.Error(err)
	}

	dbFile.Bytes = 2
	dbFile.SHA256 = "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b"
	dbFile.IP = "127.0.0.2"
	dbFile.Trace = "second trace"
	err = dao.File().Update(&dbFile)
	if err != nil {
		t.Error(err)
	}

	updatedFile, _, err := dao.File().GetById(file.Id)
	if err != nil {
		t.Error(err)
	}
	if updatedFile.Bytes != 2 {
		t.Errorf("Was expecting the updated file bytes 2, got %d instead.", updatedFile.Bytes)
	}
	if updatedFile.SHA256 != "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b" {
		t.Errorf("Was expecting the updated file checksum ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b, got %s instead.", updatedFile.SHA256)
	}
	if updatedFile.IP != "127.0.0.2" {
		t.Errorf("Was expecting the updated IP 127.0.0.2, got %s instead.", updatedFile.IP)
	}
	if updatedFile.Trace != "second trace" {
		t.Errorf("Was expecting the updated trace 'second trace', got %s instead.", updatedFile.Trace)
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

func TestGetFilesByBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	// Create files
	file1 := &ds.File{}
	file1.Filename = "file1.txt"
	file1.Bin = bin.Id // Foreign key
	file1.Bytes = 1
	file1.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	err = dao.File().Insert(file1)
	if err != nil {
		t.Error(err)
	}

	file2 := &ds.File{}
	file2.Filename = "file2.txt"
	file2.Bin = bin.Id // Foreign key
	file2.Bytes = 2
	file2.SHA256 = "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b"
	err = dao.File().Insert(file2)
	if err != nil {
		t.Error(err)
	}

	files, err := dao.File().GetByBin(bin.Id, 0)
	if err != nil {
		t.Error(err)
	}
	if len(files) != 2 {
		t.Errorf("Was expecting two files, got %d instead.", len(files))
	}

	files, err = dao.File().GetByBin("-1", 0)
	if err != nil {
		t.Error("Did not expect an error even though we asked for a bin id that does not exist")
	}
	if len(files) != 0 {
		t.Errorf("Was expecting zero files matching the non existent bin id, got %d instead.", len(files))
	}

	err = dao.File().RegisterDownload(file1)
	if err != nil {
		t.Error(err)
	}
	if file1.Downloads != 1 {
		t.Errorf("Was expecting the number of downloads to be 1, not %d\n", file1.Downloads)
	}
}

func TestInvalidFileInput(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Create bin first
	bin := &ds.Bin{}
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	file := &ds.File{}
	file.Bin = bin.Id
	file.Filename = ""
	file.Bytes = 1
	file.SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	err = dao.File().Insert(file)
	if err == nil {
		t.Error("Expected an error since filename is not set")
	}
}
