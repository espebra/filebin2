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

	// First insert should set reference count to 1
	content := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     100,
		InStorage: true,
	}

	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Errorf("Failed to insert file content: %s", err)
	}

	if content.ReferenceCount != 1 {
		t.Errorf("Expected reference count to be 1 on first insert, got %d", content.ReferenceCount)
	}

	// Second insert should increment reference count to 2
	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Errorf("Failed to increment file content: %s", err)
	}

	if content.ReferenceCount != 2 {
		t.Errorf("Expected reference count to be 2 after increment, got %d", content.ReferenceCount)
	}

	// Third insert should increment reference count to 3
	err = dao.FileContent().InsertOrIncrement(content)
	if err != nil {
		t.Errorf("Failed to increment file content: %s", err)
	}

	if content.ReferenceCount != 3 {
		t.Errorf("Expected reference count to be 3 after second increment, got %d", content.ReferenceCount)
	}

	// Verify the database has the correct count
	dbContent, err := dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	if dbContent.ReferenceCount != 3 {
		t.Errorf("Expected reference count in DB to be 3, got %d", dbContent.ReferenceCount)
	}
}

func TestFileContentDecrementRefCount(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer tearDown(dao)

	// Insert content with initial ref count
	content := &ds.FileContent{
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Bytes:     100,
		InStorage: true,
	}

	// Increment to ref count 3
	dao.FileContent().InsertOrIncrement(content)
	dao.FileContent().InsertOrIncrement(content)
	dao.FileContent().InsertOrIncrement(content)

	// Verify it's at 3
	if content.ReferenceCount != 3 {
		t.Errorf("Expected reference count to be 3, got %d", content.ReferenceCount)
	}

	// Decrement to 2
	newCount, err := dao.FileContent().DecrementRefCount(content.SHA256)
	if err != nil {
		t.Errorf("Failed to decrement ref count: %s", err)
	}

	if newCount != 2 {
		t.Errorf("Expected new count to be 2, got %d", newCount)
	}

	// Decrement to 1
	newCount, err = dao.FileContent().DecrementRefCount(content.SHA256)
	if err != nil {
		t.Errorf("Failed to decrement ref count: %s", err)
	}

	if newCount != 1 {
		t.Errorf("Expected new count to be 1, got %d", newCount)
	}

	// Decrement to 0
	newCount, err = dao.FileContent().DecrementRefCount(content.SHA256)
	if err != nil {
		t.Errorf("Failed to decrement ref count: %s", err)
	}

	if newCount != 0 {
		t.Errorf("Expected new count to be 0, got %d", newCount)
	}

	// Verify the database has ref count 0
	dbContent, err := dao.FileContent().GetBySHA256(content.SHA256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	if dbContent.ReferenceCount != 0 {
		t.Errorf("Expected reference count in DB to be 0, got %d", dbContent.ReferenceCount)
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

	// Create content with ref count 0 and in_storage true (should be pending delete)
	content1 := &ds.FileContent{
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Bytes:     100,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content1)
	dao.FileContent().DecrementRefCount(content1.SHA256)

	// Create content with ref count > 0 and in_storage true (should NOT be pending delete)
	content2 := &ds.FileContent{
		SHA256:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Bytes:     200,
		InStorage: true,
	}
	dao.FileContent().InsertOrIncrement(content2)

	// Create content with ref count 0 and in_storage false (should NOT be pending delete)
	content3 := &ds.FileContent{
		SHA256:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Bytes:     300,
		InStorage: false,
	}
	dao.FileContent().InsertOrIncrement(content3)
	dao.FileContent().DecrementRefCount(content3.SHA256)
	// Update to set in_storage to false
	dbContent3, _ := dao.FileContent().GetBySHA256(content3.SHA256)
	dbContent3.InStorage = false
	dao.FileContent().Update(dbContent3)

	// Get pending deletes
	pending, err := dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Errorf("Failed to get pending deletes: %s", err)
	}

	// Should only find content1
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending delete, got %d", len(pending))
	}

	if len(pending) > 0 && pending[0].SHA256 != content1.SHA256 {
		t.Errorf("Expected pending delete to be %s, got %s", content1.SHA256, pending[0].SHA256)
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
	bin := &ds.Bin{}
	bin.Id = "testbin123"
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
		// Increment reference count (simulates upload)
		content := &ds.FileContent{
			SHA256:    sha256,
			Bytes:     100,
			InStorage: true,
		}
		err = dao.FileContent().InsertOrIncrement(content)
		if err != nil {
			t.Fatalf("Upload %d: Failed to increment ref count: %s", i, err)
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

	// Verify reference count is 3
	content, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	if content.ReferenceCount != 3 {
		t.Errorf("Expected reference count to be 3, got %d", content.ReferenceCount)
	}

	// Simulate deleting 2 files
	t.Logf("Attempting to get files from bin: %s", bin.Id)
	files, err := dao.File().GetByBin(bin.Id, true)
	if err != nil {
		t.Error(err)
	}

	if len(files) != 3 {
		t.Fatalf("Expected 3 files in bin %s, got %d", bin.Id, len(files))
	}

	for i := 0; i < 2; i++ {
		// Decrement reference count (simulates delete)
		newCount, err := dao.FileContent().DecrementRefCount(sha256)
		if err != nil {
			t.Errorf("Delete %d: Failed to decrement ref count: %s", i, err)
		}

		expectedCount := 3 - (i + 1)
		if newCount != expectedCount {
			t.Errorf("Delete %d: Expected new count to be %d, got %d", i, expectedCount, newCount)
		}
		// File record doesn't need to be updated - in_storage tracking is now in file_content
	}

	// Verify reference count is 1
	content, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Errorf("Failed to get file content: %s", err)
	}

	if content.ReferenceCount != 1 {
		t.Errorf("Expected reference count to be 1 after 2 deletes, got %d", content.ReferenceCount)
	}

	// Content should NOT be pending delete (still has 1 reference)
	pending, err := dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Errorf("Failed to get pending deletes: %s", err)
	}

	if len(pending) != 0 {
		t.Errorf("Expected 0 pending deletes, got %d", len(pending))
	}

	// Delete the last file
	newCount, err := dao.FileContent().DecrementRefCount(sha256)
	if err != nil {
		t.Errorf("Failed to decrement ref count on last delete: %s", err)
	}

	if newCount != 0 {
		t.Errorf("Expected new count to be 0, got %d", newCount)
	}

	// Content should NOW be pending delete (ref count = 0, in_storage = true)
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
