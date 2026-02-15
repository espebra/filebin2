package dbl

import (
	"fmt"
	"github.com/espebra/filebin2/internal/ds"
	"testing"
	"time"
)

func TestFileContentInsertOrIncrement(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	// First insert should create the record
	content := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}

	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Errorf("Failed to insert file content: %s", err)
	}

	// Verify the record exists
	dbContent, err := dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get file content after insert: %s", err)
	}

	if dbContent.SHA256 != content.SHA256 {
		t.Errorf("Expected SHA256 %s, got %s", content.SHA256, dbContent.SHA256)
	}

	// Subsequent calls should update last_referenced_at without error
	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Errorf("Failed to update file content: %s", err)
	}

	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Errorf("Failed to update file content: %s", err)
	}

	// Verify the record still exists
	dbContent, err = dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	if !dbContent.InStorage {
		t.Errorf("Expected content to be in storage")
	}
}

func TestFileContentGetPendingDelete(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	// Create a bin for testing
	bin := &ds.Bin{
		Id:        "testbin789",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	_, _ = dao.Bin().Insert(bin)

	// Create content1 with no file references (should be pending delete)
	content1 := &ds.FileContent{
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(content1)
	// No file records created, so COUNT(*) will be 0

	// Create content2 with one non-deleted file (should NOT be pending delete)
	content2 := &ds.FileContent{
		SHA256:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Bytes:     200,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(content2)
	file2 := &ds.File{
		Filename: "file2.txt",
		Bin:      bin.Id,
		Bytes:    200,
		SHA256:   content2.SHA256,
	}
	_ = dao.File().Insert(file2)

	// Create content3 with files but all marked as deleted (should be pending delete)
	content3 := &ds.FileContent{
		SHA256:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Bytes:     300,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(content3)
	file3 := &ds.File{
		Filename: "file3.txt",
		Bin:      bin.Id,
		Bytes:    300,
		SHA256:   content3.SHA256,
	}
	_ = dao.File().Insert(file3)
	// Mark as deleted
	_ = file3.DeletedAt.Scan(time.Now().UTC())
	_ = dao.File().Update(file3)

	// Create content4 with in_storage=false (should NOT be pending delete)
	content4 := &ds.FileContent{
		SHA256:    "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		Bytes:     400,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: false,
	}
	_ = dao.FileContent().InsertOrIncrement(content4)

	// Get pending deletes
	pending, err := dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Errorf("Failed to get pending deletes: %s", err)
	}

	// Should find content1 and content3 (both have no non-deleted file references)
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending deletes, got %d", len(pending))
	}

	// Verify we got the right content
	foundContent1 := false
	foundContent3 := false
	for _, p := range pending {
		if p.SHA256 == content1.SHA256 {
			foundContent1 = true
		}
		if p.SHA256 == content3.SHA256 {
			foundContent3 = true
		}
	}

	if !foundContent1 {
		t.Errorf("Expected to find content1 (%s) in pending deletes", content1.SHA256)
	}
	if !foundContent3 {
		t.Errorf("Expected to find content3 (%s) in pending deletes", content3.SHA256)
	}
}

func TestFileContentGetPendingDeleteWithExpiredBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	// Create a bin that is NOT expired
	activeBin := &ds.Bin{
		Id:        "activebin",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	_, _ = dao.Bin().Insert(activeBin)

	// Create an expired bin
	expiredBin := &ds.Bin{
		Id:        "expiredbin",
		ExpiredAt: time.Now().UTC().Add(-time.Hour), // Expired 1 hour ago
	}
	_, _ = dao.Bin().Insert(expiredBin)

	// Create content with file in active bin (should NOT be pending delete)
	activeContent := &ds.FileContent{
		SHA256:    "1111111111111111111111111111111111111111111111111111111111111111",
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(activeContent)
	activeFile := &ds.File{
		Filename: "activefile.txt",
		Bin:      activeBin.Id,
		Bytes:    100,
		SHA256:   activeContent.SHA256,
	}
	_ = dao.File().Insert(activeFile)

	// Create content with file in expired bin (should be pending delete)
	expiredContent := &ds.FileContent{
		SHA256:    "2222222222222222222222222222222222222222222222222222222222222222",
		Bytes:     200,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(expiredContent)
	expiredFile := &ds.File{
		Filename: "expiredfile.txt",
		Bin:      expiredBin.Id,
		Bytes:    200,
		SHA256:   expiredContent.SHA256,
	}
	_ = dao.File().Insert(expiredFile)

	// Get pending deletes
	pending, err := dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Errorf("Failed to get pending deletes: %s", err)
	}

	// Should only find expiredContent (file in expired bin)
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending delete, got %d", len(pending))
	}

	if len(pending) > 0 && pending[0].SHA256 != expiredContent.SHA256 {
		t.Errorf("Expected pending delete to be %s, got %s", expiredContent.SHA256, pending[0].SHA256)
	}
}

func TestFileContentGetAll(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	// Create multiple content records
	content1 := &ds.FileContent{
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(content1)

	content2 := &ds.FileContent{
		SHA256:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Bytes:     200,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(content2)

	all, err := dao.FileContent().GetAll()
	if err != nil {
		t.Errorf("Failed to get all file contents: %s", err)
	}

	if len(all) != 2 {
		t.Errorf("Expected 2 file contents, got %d", len(all))
	}
}

func TestFileContentUpdate(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	// Create content
	content := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	_ = dao.FileContent().InsertOrIncrement(content)

	// Get and modify
	dbContent, err := dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	dbContent.InStorage = false
	err = dao.FileContent().Update(dbContent)
	if err != nil {
		t.Errorf("Failed to update file content: %s", err)
	}

	// Verify update
	updatedContent, err := dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get updated file content: %s", err)
	}

	if updatedContent.InStorage != false {
		t.Errorf("Expected in_storage to be false, got %v", updatedContent.InStorage)
	}
}

func TestFileCountBySHA256(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	sha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Create bin
	bin := &ds.Bin{
		Id:        "testbin123",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	// Count should be 0 initially
	count, err := dao.File().CountBySHA256(sha256)
	if err != nil {
		t.Errorf("Failed to count files: %s", err)
	}

	if count != 0 {
		t.Errorf("Expected count to be 0, got %d", count)
	}

	// Create file_content record (required by foreign key constraint)
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

	// Create 3 files with same SHA256
	for i := 0; i < 3; i++ {
		file := &ds.File{
			Filename: fmt.Sprintf("file%d.txt", i),
			Bin:      bin.Id,
			Bytes:    100,
			SHA256:   sha256,
		}
		err = dao.File().Insert(file)
		if err != nil {
			t.Error(err)
		}
	}

	// Count should be 3
	count, err = dao.File().CountBySHA256(sha256)
	if err != nil {
		t.Errorf("Failed to count files: %s", err)
	}

	if count != 3 {
		t.Errorf("Expected count to be 3, got %d", count)
	}

	// Mark one file as deleted
	files, err := dao.File().GetByBin(bin.Id, true)
	if err != nil {
		t.Error(err)
	}
	_ = files[0].DeletedAt.Scan(time.Now().UTC())
	err = dao.File().Update(&files[0])
	if err != nil {
		t.Error(err)
	}

	// Count should be 2 (one deleted)
	count, err = dao.File().CountBySHA256(sha256)
	if err != nil {
		t.Errorf("Failed to count files after marking one deleted: %s", err)
	}
	if count != 2 {
		t.Errorf("Expected count to be 2 after marking one deleted, got %d", count)
	}

	// Mark the bin as deleted
	_ = bin.DeletedAt.Scan(time.Now().UTC())
	err = dao.Bin().Update(bin)
	if err != nil {
		t.Error(err)
	}

	// Count should be 0 (bin deleted, so all files in it are considered inactive)
	count, err = dao.File().CountBySHA256(sha256)
	if err != nil {
		t.Errorf("Failed to count files after marking bin deleted: %s", err)
	}
	if count != 0 {
		t.Errorf("Expected count to be 0 after marking bin deleted, got %d", count)
	}
}

func TestFileCountBySHA256WithExpiredBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	sha256Active := "f3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b856"
	sha256Expired := "a4b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b857"

	// Create active (non-expired) bin
	activeBin := &ds.Bin{
		Id:        "activebintest",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	_, err = dao.Bin().Insert(activeBin)
	if err != nil {
		t.Error(err)
	}

	// Create expired bin (expired 1 hour ago)
	expiredBin := &ds.Bin{
		Id:        "expiredbintest",
		ExpiredAt: time.Now().UTC().Add(-time.Hour),
	}
	_, err = dao.Bin().Insert(expiredBin)
	if err != nil {
		t.Error(err)
	}

	// Create file_content records
	activeContent := &ds.FileContent{
		SHA256:    sha256Active,
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(activeContent)
	if err != nil {
		t.Error(err)
	}

	expiredContent := &ds.FileContent{
		SHA256:    sha256Expired,
		Bytes:     100,
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "application/octet-stream",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(expiredContent)
	if err != nil {
		t.Error(err)
	}

	// Create file in active bin
	activeFile := &ds.File{
		Filename: "activefile.txt",
		Bin:      activeBin.Id,
		Bytes:    100,
		SHA256:   sha256Active,
	}
	err = dao.File().Insert(activeFile)
	if err != nil {
		t.Error(err)
	}

	// Create file in expired bin
	expiredFile := &ds.File{
		Filename: "expiredfile.txt",
		Bin:      expiredBin.Id,
		Bytes:    100,
		SHA256:   sha256Expired,
	}
	err = dao.File().Insert(expiredFile)
	if err != nil {
		t.Error(err)
	}

	// Count for active bin's file should be 1 (bin not expired)
	count, err := dao.File().CountBySHA256(sha256Active)
	if err != nil {
		t.Errorf("Failed to count files: %s", err)
	}
	if count != 1 {
		t.Errorf("Expected count to be 1 for active bin, got %d", count)
	}

	// Count for expired bin's file should be 0 (bin expired)
	count, err = dao.File().CountBySHA256(sha256Expired)
	if err != nil {
		t.Errorf("Failed to count files in expired bin: %s", err)
	}
	if count != 0 {
		t.Errorf("Expected count to be 0 for expired bin, got %d", count)
	}
}

func TestDeduplicationFlow(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	sha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Create bin
	bin := &ds.Bin{}
	bin.Id = "testbin456"
	bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 24)
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Fatalf("Failed to insert bin: %s", err)
	}
	t.Logf("Created bin with ID: %s", bin.Id)

	// Simulate uploading same content 3 times
	for i := 0; i < 3; i++ {
		// Create file_content record (or update existing)
		content := &ds.FileContent{
			SHA256:    sha256,
			Bytes:     100,
			MD5:       "d41d8cd98f00b204e9800998ecf8427e",
			Mime:      "application/octet-stream",
			InStorage: true,
		}
		err = dao.FileContent().InsertOrIncrement(content)
		if err != nil {
			t.Fatalf("Upload %d: Failed to insert/update file_content: %s", i, err)
		}

		// Create file record
		file := &ds.File{
			Filename: fmt.Sprintf("file%d.txt", i),
			Bin:      bin.Id,
			Bytes:    100,
			SHA256:   sha256,
		}
		err = dao.File().Insert(file)
		if err != nil {
			t.Fatalf("Upload %d: Failed to insert file: %s (bin=%s, filename=%s)", i, err, bin.Id, file.Filename)
		}
		t.Logf("Successfully inserted file %d: %s", file.Id, file.Filename)
	}

	// Verify content exists
	content, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	if !content.InStorage {
		t.Errorf("Expected content to be in storage")
	}

	// Simulate deleting 2 files (mark as deleted)
	t.Logf("Attempting to get files from bin: %s", bin.Id)
	files, err := dao.File().GetByBin(bin.Id, true)
	if err != nil {
		t.Error(err)
	}

	if len(files) != 3 {
		t.Fatalf("Expected 3 files in bin %s, got %d", bin.Id, len(files))
	}

	for i := 0; i < 2; i++ {
		// Mark file as deleted
		_ = files[i].DeletedAt.Scan(time.Now().UTC())
		err = dao.File().Update(&files[i])
		if err != nil {
			t.Errorf("Delete %d: Failed to mark file as deleted: %s", i, err)
		}
	}

	// Content should NOT be pending delete (still has 1 non-deleted file)
	pending, err := dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Errorf("Failed to get pending deletes: %s", err)
	}

	if len(pending) != 0 {
		t.Errorf("Expected 0 pending deletes, got %d", len(pending))
	}

	// Mark the last file as deleted
	_ = files[2].DeletedAt.Scan(time.Now().UTC())
	err = dao.File().Update(&files[2])
	if err != nil {
		t.Errorf("Failed to mark last file as deleted: %s", err)
	}

	// Content should NOW be pending delete (COUNT(*) where deleted_at IS NULL = 0)
	pending, err = dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Errorf("Failed to get pending deletes: %s", err)
	}

	if len(pending) != 1 {
		t.Errorf("Expected 1 pending delete, got %d", len(pending))
	}

	if len(pending) > 0 && pending[0].SHA256 != sha256 {
		t.Errorf("Expected pending delete to be %s, got %s", sha256, pending[0].SHA256)
	}
}
