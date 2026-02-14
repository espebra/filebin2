package dbl

import (
	"errors"
	"fmt"
	"github.com/espebra/filebin2/internal/ds"
	"testing"
	"time"
)

// ensureFileContent creates a file_content record if it doesn't exist
// This is required because of the foreign key constraint from file.sha256 to file_content.sha256
func ensureFileContent(dao DAO, file *ds.File) error {
	// Check if file_content already exists
	_, err := dao.FileContent().GetBySHA256(file.SHA256)
	if err == nil {
		// Already exists, nothing to do
		return nil
	}

	// Create new file_content record
	content := &ds.FileContent{
		SHA256:    file.SHA256,
		Bytes:     file.Bytes,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	return dao.FileContent().InsertOrIncrement(content)
}

func TestUpsert(t *testing.T) {
	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer func() { _ = tearDown(dao) }()

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

	// Ensure file_content exists (required by foreign key constraint)
	err = ensureFileContent(dao, file)
	if err != nil {
		t.Error(err)
	}

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	if file.Id == 0 {
		t.Error(errors.New("Expected id > 0"))
	}
}

func TestGetFileById(t *testing.T) {
	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer func() { _ = tearDown(dao) }()

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

	// Ensure file_content exists (required by foreign key constraint)
	err = ensureFileContent(dao, file)
	if err != nil {
		t.Error(err)
	}

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	if file.Id == 0 {
		t.Error(errors.New("Expected id > 0"))
	}

	dbFile, found, err := dao.File().GetByID(file.Id)
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
	if dbFile.Headers != "N/A" {
		t.Errorf("Was expecting default value for headers, got %s instead.", dbFile.Headers)
	}

	dbBin, found, err := dao.Bin().GetByID(dbFile.Bin)
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
	defer func() { _ = tearDown(dao) }()

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
	file.Headers = "some headers"

	// Ensure file_content exists (required by foreign key constraint)
	err = ensureFileContent(dao, file)
	if err != nil {
		t.Error(err)
	}

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
	if dbFile.Headers != "some headers" {
		t.Errorf("Was expecting headers 'some headers', got %s instead.", dbFile.Headers)
	}
}

func TestInsertDuplicatedFile(t *testing.T) {
	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer func() { _ = tearDown(dao) }()

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

	// Ensure file_content exists (required by foreign key constraint)
	err = ensureFileContent(dao, file)
	if err != nil {
		t.Error(err)
	}

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	_, _, err = dao.File().GetByID(file.Id)

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

	defer func() { _ = tearDown(dao) }()

	// Create bin first
	bin := &ds.Bin{}
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	count := 50

	// Create file_content record for the files
	sha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	content := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     1,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	// Initialize ref count to 1
	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < count; i++ {
		file := &ds.File{}
		file.Filename = fmt.Sprintf("File-%d", i)
		file.Bin = bin.Id
		file.Bytes = 1
		file.SHA256 = sha256
		err = dao.File().Insert(file)

		if err != nil {
			t.Error(err)
			break
		}

		// Increment ref count for each additional file (except the first)
		if i > 0 {
			err = dao.FileContent().InsertOrIncrement(content)
			if err != nil {
				t.Error(err)
				break
			}
		}
	}

	files, err := dao.File().GetAll(true)
	if err != nil {
		t.Error(err)
	}

	if len(files) != count {
		t.Errorf("Was expecting to find %d files, got %d instead.", count, len(files))
	}

	dbBin, _, err := dao.Bin().GetByID(bin.Id)
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

	defer func() { _ = tearDown(dao) }()

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

	// Ensure file_content exists (required by foreign key constraint)
	err = ensureFileContent(dao, file)
	if err != nil {
		t.Error(err)
	}

	err = dao.File().Insert(file)

	if err != nil {
		t.Error(err)
	}

	dbFile, _, err := dao.File().GetByID(file.Id)

	if err != nil {
		t.Error(err)
	}

	err = dao.File().Delete(&dbFile)

	if err != nil {
		t.Error(err)
	}

	_, found, err := dao.File().GetByID(file.Id)
	if err != nil {
		t.Errorf("Did not expect an error even though the file was deleted: %s\n", err.Error())
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
	defer func() { _ = tearDown(dao) }()

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
	file.Headers = "first headers"

	// Ensure file_content exists (required by foreign key constraint)
	err = ensureFileContent(dao, file)
	if err != nil {
		t.Error(err)
	}

	err = dao.File().Insert(file)
	if err != nil {
		t.Error(err)
	}

	dbFile, _, err := dao.File().GetByID(file.Id)
	if err != nil {
		t.Error(err)
	}

	dbFile.Bytes = 2
	dbFile.SHA256 = "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b"
	dbFile.IP = "127.0.0.2"
	dbFile.Headers = "second headers"

	// Ensure new SHA256 exists in file_content before updating
	err = ensureFileContent(dao, &dbFile)
	if err != nil {
		t.Error(err)
	}

	err = dao.File().Update(&dbFile)
	if err != nil {
		t.Error(err)
	}

	updatedFile, _, err := dao.File().GetByID(file.Id)
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
	if updatedFile.Headers != "second headers" {
		t.Errorf("Was expecting the updated headers 'second headers', got %s instead.", updatedFile.Headers)
	}
}

func TestUpdateNonExistingFile(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

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
	defer func() { _ = tearDown(dao) }()

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
	defer func() { _ = tearDown(dao) }()

	// Create bin first
	bin := &ds.Bin{}
	bin.Id = "1234567890"
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	// Create file_content records first
	content1 := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     1,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(content1)
	if err != nil {
		t.Error(err)
	}

	content2 := &ds.FileContent{
		SHA256:    "ff0350c8a7fea1087c5300e9ae922a7ab453648b1c156d5c58437d9f4565244b",
		Bytes:     2,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(content2)
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

	files, err := dao.File().GetByBin(bin.Id, true)
	if err != nil {
		t.Error(err)
	}
	if len(files) != 2 {
		t.Errorf("Was expecting two files, got %d instead.", len(files))
	}

	files, err = dao.File().GetByBin("-1", true)
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

func TestIsAvailableForDownload(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	sha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Create file_content record
	content := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Error(err)
	}

	// Test 1: File should be available (all conditions met)
	bin1 := &ds.Bin{Id: "availablebin1", ExpiredAt: time.Now().UTC().Add(time.Hour * 24)}
	err = dao.Bin().Insert(bin1)
	if err != nil {
		t.Error(err)
	}
	file1 := &ds.File{Filename: "test1.txt", Bin: bin1.Id, Bytes: 100, SHA256: sha256}
	err = dao.File().Insert(file1)
	if err != nil {
		t.Error(err)
	}
	available, err := dao.File().IsAvailableForDownload(file1.Id)
	if err != nil {
		t.Errorf("Failed to check availability: %s", err)
	}
	if !available {
		t.Error("File should be available when all conditions are met")
	}

	// Test 2: File not available when file is deleted
	bin2 := &ds.Bin{Id: "availablebin2", ExpiredAt: time.Now().UTC().Add(time.Hour * 24)}
	err = dao.Bin().Insert(bin2)
	if err != nil {
		t.Error(err)
	}
	file2 := &ds.File{Filename: "test2.txt", Bin: bin2.Id, Bytes: 100, SHA256: sha256}
	err = dao.File().Insert(file2)
	if err != nil {
		t.Error(err)
	}
	file2.DeletedAt.Time = time.Now().UTC()
	file2.DeletedAt.Valid = true
	err = dao.File().Update(file2)
	if err != nil {
		t.Error(err)
	}
	available, err = dao.File().IsAvailableForDownload(file2.Id)
	if err != nil {
		t.Errorf("Failed to check availability: %s", err)
	}
	if available {
		t.Error("File should not be available when file is deleted")
	}

	// Test 3: File not available when bin is expired
	bin3 := &ds.Bin{Id: "expiredbin123", ExpiredAt: time.Now().UTC().Add(-time.Hour)}
	err = dao.Bin().Insert(bin3)
	if err != nil {
		t.Error(err)
	}
	file3 := &ds.File{Filename: "test3.txt", Bin: bin3.Id, Bytes: 100, SHA256: sha256}
	err = dao.File().Insert(file3)
	if err != nil {
		t.Error(err)
	}
	available, err = dao.File().IsAvailableForDownload(file3.Id)
	if err != nil {
		t.Errorf("Failed to check availability: %s", err)
	}
	if available {
		t.Error("File should not be available when bin is expired")
	}

	// Test 4: File not available when bin is deleted
	bin4 := &ds.Bin{Id: "deletedbin123", ExpiredAt: time.Now().UTC().Add(time.Hour * 24)}
	err = dao.Bin().Insert(bin4)
	if err != nil {
		t.Error(err)
	}
	bin4.DeletedAt.Time = time.Now().UTC()
	bin4.DeletedAt.Valid = true
	err = dao.Bin().Update(bin4)
	if err != nil {
		t.Error(err)
	}
	file4 := &ds.File{Filename: "test4.txt", Bin: bin4.Id, Bytes: 100, SHA256: sha256}
	err = dao.File().Insert(file4)
	if err != nil {
		t.Error(err)
	}
	available, err = dao.File().IsAvailableForDownload(file4.Id)
	if err != nil {
		t.Errorf("Failed to check availability: %s", err)
	}
	if available {
		t.Error("File should not be available when bin is deleted")
	}

	// Test 5: File not available when content is not in storage
	sha256_2 := "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3"
	content2 := &ds.FileContent{
		SHA256:    sha256_2,
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: false,
	}
	err = dao.FileContent().InsertOrIncrement(content2)
	if err != nil {
		t.Error(err)
	}
	bin5 := &ds.Bin{Id: "availablebin5", ExpiredAt: time.Now().UTC().Add(time.Hour * 24)}
	err = dao.Bin().Insert(bin5)
	if err != nil {
		t.Error(err)
	}
	file5 := &ds.File{Filename: "test5.txt", Bin: bin5.Id, Bytes: 100, SHA256: sha256_2}
	err = dao.File().Insert(file5)
	if err != nil {
		t.Error(err)
	}
	available, err = dao.File().IsAvailableForDownload(file5.Id)
	if err != nil {
		t.Errorf("Failed to check availability: %s", err)
	}
	if available {
		t.Error("File should not be available when content is not in storage")
	}

	// Test 6: Non-existent file ID should return false
	available, err = dao.File().IsAvailableForDownload(999999)
	if err != nil {
		t.Errorf("Failed to check availability: %s", err)
	}
	if available {
		t.Error("Non-existent file should not be available")
	}
}

func TestInvalidFileInput(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

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

	// Ensure file_content exists (required by foreign key constraint)
	_ = ensureFileContent(dao, file) // ignore error since we expect Insert to fail anyway

	err = dao.File().Insert(file)
	if err == nil {
		t.Error("Expected an error since filename is not set")
	}
}

func TestUpsertWiderCharacterSet(t *testing.T) {
	type TestCase struct {
		InputFilename    string
		Valid            bool
		ModifiedFilename string
	}

	tests := []TestCase{
		{
			InputFilename:    "a",
			Valid:            true,
			ModifiedFilename: "a",
		}, {
			InputFilename:    "1",
			Valid:            true,
			ModifiedFilename: "1",
		}, {
			InputFilename:    "雨中.txt",
			Valid:            true,
			ModifiedFilename: "雨中.txt",
		}, {
			InputFilename:    ".",
			Valid:            true,
			ModifiedFilename: "_",
		}, {
			InputFilename:    "",
			Valid:            false,
			ModifiedFilename: "",
		}, {
			InputFilename:    "foo\bbar",
			Valid:            true,
			ModifiedFilename: "foo_bar",
		}, {
			InputFilename:    "   ",
			Valid:            false,
			ModifiedFilename: "",
		}, {
			InputFilename:    "    test    ",
			Valid:            true,
			ModifiedFilename: "test",
		}, {
			InputFilename:    "    ../foo/bar/baz.zip    ",
			Valid:            true,
			ModifiedFilename: "baz.zip",
		}, {
			InputFilename:    "℃!",
			Valid:            true,
			ModifiedFilename: "__",
		}, {
			InputFilename:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Valid:            true,
			ModifiedFilename: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}, {
			InputFilename:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Valid:            true,
			ModifiedFilename: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}, {
			InputFilename:    " a b ( ) [ ] ",
			Valid:            true,
			ModifiedFilename: "a b ( ) [ ]",
		}, {
			InputFilename:    "a    b c  d",
			Valid:            true,
			ModifiedFilename: "a b c d",
		},
	}

	dao, err := tearUp()

	if err != nil {
		t.Error(err)
	}

	defer func() { _ = tearDown(dao) }()

	// Create bin first
	bin := &ds.Bin{}
	bin.Id = "sometestbin"
	err = dao.Bin().Insert(bin)

	if err != nil {
		t.Error(err)
	}

	// Create file_content record for the default SHA256 used by all test files
	// Since file.SHA256 is not explicitly set, it will use the default empty value
	// We need to set it to a valid hash for the foreign key constraint
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

	for i, test := range tests {
		file := &ds.File{}
		file.Filename = test.InputFilename
		file.Bin = bin.Id
		file.SHA256 = defaultSHA256 // Set SHA256 to satisfy foreign key constraint
		err = dao.File().Insert(file)

		if !test.Valid {
			if err == nil {
				t.Errorf("Expected error and invalid filename, got %q\n", file.Filename)
			}
		}

		if test.Valid {
			if err != nil {
				t.Errorf("Got %s, but did not expect error here", err.Error())
			}

			if file.Id == 0 {
				t.Error(errors.New("Expected id > 0"))
			}

			if file.Filename != test.ModifiedFilename {
				t.Errorf("Test case %d: Unexpected filename. Got %q, expected %q\n", i, file.Filename, test.ModifiedFilename)
			}
		}
	}
}
