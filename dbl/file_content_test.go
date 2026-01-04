package dbl

import (
	"fmt"
	"github.com/espebra/filebin2/ds"
	"testing"
	"time"
)

func TestFileContentInsertOrIncrement(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// First insert should create the record
	content := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     100,
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

func TestFileContentIncrementDownloads(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Insert content
	content := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     100,
		InStorage: true,
	}

	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Errorf("Failed to insert file content: %s", err)
	}

	// Verify initial downloads is 0
	dbContent, err := dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	if dbContent.Downloads != 0 {
		t.Errorf("Expected initial downloads to be 0, got %d", dbContent.Downloads)
	}

	// Increment downloads 5 times
	for i := 1; i <= 5; i++ {
		err = dao.FileContent().IncrementDownloads(content.SHA256)
		if err != nil {
			t.Errorf("Failed to increment downloads (iteration %d): %s", i, err)
		}

		// Verify the count after each increment
		dbContent, err = dao.FileContent().GetBySHA256(content.SHA256)
		if err != nil {
			t.Errorf("Failed to get file content after increment %d: %s", i, err)
		}

		if dbContent.Downloads != uint64(i) {
			t.Errorf("Expected downloads to be %d after increment %d, got %d", i, i, dbContent.Downloads)
		}
	}

	// Final verification
	dbContent, err = dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get file content for final check: %s", err)
	}

	if dbContent.Downloads != 5 {
		t.Errorf("Expected final downloads count to be 5, got %d", dbContent.Downloads)
	}

	// Test incrementing downloads on non-existent content
	err = dao.FileContent().IncrementDownloads("nonexistent0000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Error("Expected error when incrementing downloads for non-existent content")
	}
}

func TestFileContentGetPendingDelete(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Create a bin for testing
	bin := &ds.Bin{
		Id:        "testbin789",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	dao.Bin().Insert(bin)

	// Create content1 with no file references (should be pending delete)
	content1 := &ds.FileContent{
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Bytes:     100,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content1)
	// No file records created, so COUNT(*) will be 0

	// Create content2 with one non-deleted file (should NOT be pending delete)
	content2 := &ds.FileContent{
		SHA256:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Bytes:     200,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content2)
	file2 := &ds.File{
		Filename: "file2.txt",
		Bin:      bin.Id,
		Bytes:    200,
		SHA256:   content2.SHA256,
	}
	dao.File().Insert(file2)

	// Create content3 with files but all marked as deleted (should be pending delete)
	content3 := &ds.FileContent{
		SHA256:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Bytes:     300,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content3)
	file3 := &ds.File{
		Filename: "file3.txt",
		Bin:      bin.Id,
		Bytes:    300,
		SHA256:   content3.SHA256,
	}
	dao.File().Insert(file3)
	// Mark as deleted
	file3.DeletedAt.Scan(time.Now().UTC())
	dao.File().Update(file3)

	// Create content4 with in_storage=false (should NOT be pending delete)
	content4 := &ds.FileContent{
		SHA256:    "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		Bytes:     400,
		InStorage: false,
	}
	dao.FileContent().InsertOrIncrement(content4)

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

func TestFileContentGetAll(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Create multiple content records
	content1 := &ds.FileContent{
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Bytes:     100,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content1)

	content2 := &ds.FileContent{
		SHA256:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Bytes:     200,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content2)

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
	defer tearDown(dao)

	// Create content
	content := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     100,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content)

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
	defer tearDown(dao)

	sha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Create bin
	bin := &ds.Bin{
		Id:        "testbin123",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	err = dao.Bin().Insert(bin)
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
	files[0].DeletedAt.Scan(time.Now().UTC())
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
	bin.DeletedAt.Scan(time.Now().UTC())
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

func TestDeduplicationFlow(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	sha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Create bin
	bin := &ds.Bin{}
	bin.Id = "testbin456"
	bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 24)
	err = dao.Bin().Insert(bin)
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
		files[i].DeletedAt.Scan(time.Now().UTC())
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
	files[2].DeletedAt.Scan(time.Now().UTC())
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
