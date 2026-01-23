package dbl

import (
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/s3"
	"strings"
	"testing"
	"time"
)

const (
	testS3Endpoint  = "s3:9000"
	testS3Region    = "us-east-1"
	testS3Bucket    = "filebin-test-storage"
	testS3AccessKey = "s3accesskey"
	testS3SecretKey = "s3secretkey"
)

func setupS3() (s3.S3AO, error) {
	expiry := time.Second * 60
	timeout := time.Second * 30
	transferTimeout := time.Minute * 10
	s3ao, err := s3.Init(testS3Endpoint, testS3Bucket, testS3Region, testS3AccessKey, testS3SecretKey, false, expiry, timeout, transferTimeout)
	if err != nil {
		return s3ao, err
	}
	return s3ao, nil
}

func teardownS3(s3ao s3.S3AO) error {
	return s3ao.RemoveBucket()
}

// TestFileContentInStorageReflectsS3State verifies that in_storage accurately tracks S3 state
func TestFileContentInStorageReflectsS3State(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer teardownS3(s3ao)

	sha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	content := "test content"

	// Step 1: Upload content to S3 and set in_storage=true
	err = s3ao.PutObjectByHash(sha256, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Failed to upload to S3: %s", err)
	}

	// Verify content exists in S3
	_, err = s3ao.StatObject(sha256)
	if err != nil {
		t.Errorf("Content should exist in S3 after upload, but got error: %s", err)
	}

	// Insert file_content with in_storage=true
	fileContent := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "text/plain",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(fileContent)
	if err != nil {
		t.Fatalf("Failed to insert file_content: %s", err)
	}

	// Verify in_storage is true in database
	dbContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}

	if !dbContent.InStorage {
		t.Error("in_storage should be true when content is in S3")
	}

	// Step 2: Remove content from S3 and set in_storage=false
	err = s3ao.RemoveObjectByHash(sha256)
	if err != nil {
		t.Fatalf("Failed to remove from S3: %s", err)
	}

	// Verify content no longer exists in S3
	_, err = s3ao.StatObject(sha256)
	if err == nil {
		t.Error("Content should not exist in S3 after removal")
	}

	// Update database to reflect S3 state
	dbContent.InStorage = false
	err = dao.FileContent().Update(dbContent)
	if err != nil {
		t.Fatalf("Failed to update file_content: %s", err)
	}

	// Verify in_storage is false in database
	updatedContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get updated file_content: %s", err)
	}

	if updatedContent.InStorage {
		t.Error("in_storage should be false when content is not in S3")
	}
}

// TestFileInStorageReflectsS3State verifies that file.in_storage accurately tracks S3 state
func TestFileInStorageReflectsS3State(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer teardownS3(s3ao)

	sha256 := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
	content := "test file content"

	// Create bin
	bin := &ds.Bin{
		Id:        "teststoragebin",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Fatalf("Failed to insert bin: %s", err)
	}

	// Step 1: Upload to S3 and create file record with in_storage=true
	err = s3ao.PutObjectByHash(sha256, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Failed to upload to S3: %s", err)
	}

	// Verify content exists in S3
	_, err = s3ao.StatObject(sha256)
	if err != nil {
		t.Errorf("Content should exist in S3 after upload, but got error: %s", err)
	}

	// Create file_content record
	fileContent := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "text/plain",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(fileContent)
	if err != nil {
		t.Fatalf("Failed to insert file_content: %s", err)
	}

	// Create file record
	file := &ds.File{
		Filename: "testfile.txt",
		Bin:      bin.Id,
		Bytes:    uint64(len(content)),
		SHA256:   sha256,
	}
	err = dao.File().Insert(file)
	if err != nil {
		t.Fatalf("Failed to insert file: %s", err)
	}

	// Verify file exists
	_, found, err := dao.File().GetByID(file.Id)
	if err != nil {
		t.Fatalf("Failed to get file: %s", err)
	}
	if !found {
		t.Fatal("File should be found")
	}

	// Step 2: Delete from S3 and update file_content.in_storage=false
	err = s3ao.RemoveObjectByHash(sha256)
	if err != nil {
		t.Fatalf("Failed to remove from S3: %s", err)
	}

	// Verify content no longer exists in S3
	_, err = s3ao.StatObject(sha256)
	if err == nil {
		t.Error("Content should not exist in S3 after removal")
	}

	// Update file_content to reflect S3 state (normally done by lurker)
	dbContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	dbContent.InStorage = false
	err = dao.FileContent().Update(dbContent)
	if err != nil {
		t.Fatalf("Failed to update file_content: %s", err)
	}
}

// TestDeduplicationWithS3Storage verifies deduplication works correctly with actual S3 operations
func TestDeduplicationWithS3Storage(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer teardownS3(s3ao)

	sha256 := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	content := "duplicate content"

	// Create bin
	bin := &ds.Bin{
		Id:        "dedupbin123",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	err = dao.Bin().Insert(bin)
	if err != nil {
		t.Fatalf("Failed to insert bin: %s", err)
	}

	// Upload 1: Upload to S3 (first time)
	t.Log("Upload 1: First upload of content")
	err = s3ao.PutObjectByHash(sha256, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Failed to upload to S3: %s", err)
	}

	// Verify content exists in S3
	_, err = s3ao.StatObject(sha256)
	if err != nil {
		t.Errorf("Content should exist in S3 after first upload: %s", err)
	}

	// Create file_content and file records
	fileContent := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "text/plain",
		InStorage: true,
	}
	err = dao.FileContent().InsertOrIncrement(fileContent)
	if err != nil {
		t.Fatalf("Failed to insert file_content: %s", err)
	}

	file1 := &ds.File{
		Filename: "file1.txt",
		Bin:      bin.Id,
		Bytes:    uint64(len(content)),
		SHA256:   sha256,
	}
	err = dao.File().Insert(file1)
	if err != nil {
		t.Fatalf("Failed to insert file1: %s", err)
	}

	// Verify content exists
	dbContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if !dbContent.InStorage {
		t.Error("file_content.in_storage should be true")
	}

	// Upload 2: Skip S3 upload (deduplication), just update last_referenced_at
	t.Log("Upload 2: Deduplicated upload (skip S3)")

	// Check if content already exists in S3 (it should)
	_, err = s3ao.StatObject(sha256)
	if err != nil {
		t.Error("Content should still exist in S3 for deduplication")
	}

	// Update last_referenced_at (no S3 upload needed)
	err = dao.FileContent().InsertOrIncrement(&ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "text/plain",
		InStorage: true,
	})
	if err != nil {
		t.Fatalf("Failed to update file_content: %s", err)
	}

	file2 := &ds.File{
		Filename: "file2.txt",
		Bin:      bin.Id,
		Bytes:    uint64(len(content)),
		SHA256:   sha256,
	}
	err = dao.File().Insert(file2)
	if err != nil {
		t.Fatalf("Failed to insert file2: %s", err)
	}

	// Verify content still in S3
	dbContent, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if !dbContent.InStorage {
		t.Error("file_content.in_storage should still be true after deduplication")
	}

	// Delete 1: Mark first file as deleted, keep content in S3
	t.Log("Delete 1: Remove one file reference")
	file1.DeletedAt.Scan(time.Now().UTC())
	err = dao.File().Update(file1)
	if err != nil {
		t.Fatalf("Failed to mark file1 as deleted: %s", err)
	}

	// Verify content STILL exists in S3 (one reference remaining)
	_, err = s3ao.StatObject(sha256)
	if err != nil {
		t.Error("Content should still exist in S3 with one reference remaining")
	}

	// file_content should still have in_storage=true
	dbContent, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if !dbContent.InStorage {
		t.Error("file_content.in_storage should still be true with references remaining")
	}

	// Verify content is NOT pending delete (still has one non-deleted file)
	pending, err := dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Fatalf("Failed to get pending deletes: %s", err)
	}
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending deletes with one file remaining, got %d", len(pending))
	}

	// Delete 2: Mark last file as deleted, then remove from S3
	t.Log("Delete 2: Remove last file reference and delete from S3")
	file2.DeletedAt.Scan(time.Now().UTC())
	err = dao.File().Update(file2)
	if err != nil {
		t.Fatalf("Failed to mark file2 as deleted: %s", err)
	}

	// Verify content IS pending delete (COUNT(*) where deleted_at IS NULL = 0)
	pending, err = dao.FileContent().GetPendingDelete()
	if err != nil {
		t.Fatalf("Failed to get pending deletes: %s", err)
	}
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending delete with no files remaining, got %d", len(pending))
	}

	// Now remove from S3 (simulating lurker cleanup)
	err = s3ao.RemoveObjectByHash(sha256)
	if err != nil {
		t.Fatalf("Failed to remove from S3: %s", err)
	}

	// Verify content no longer exists in S3
	_, err = s3ao.StatObject(sha256)
	if err == nil {
		t.Error("Content should not exist in S3 after removing last reference")
	}

	// Update file_content to reflect removal (fetch fresh data first)
	dbContent, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content before update: %s", err)
	}
	dbContent.InStorage = false
	err = dao.FileContent().Update(dbContent)
	if err != nil {
		t.Fatalf("Failed to update file_content: %s", err)
	}

	// Verify file_content.in_storage is false
	finalContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get final file_content: %s", err)
	}
	if finalContent.InStorage {
		t.Error("file_content.in_storage should be false after S3 removal")
	}
}

// TestReuploadAfterDeletion verifies that content can be re-uploaded after being deleted from S3
func TestReuploadAfterDeletion(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao)

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer teardownS3(s3ao)

	sha256 := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	content := "test content for reupload"

	t.Run("delete_files_then_reupload", func(t *testing.T) {
		// Create two bins
		bin1 := &ds.Bin{
			Id:        "reupload-bin1",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		dao.Bin().Insert(bin1)

		bin2 := &ds.Bin{
			Id:        "reupload-bin2",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		dao.Bin().Insert(bin2)

		// Upload same content to both bins
		fileContent := &ds.FileContent{
			SHA256:    sha256,
			Bytes:     uint64(len(content)),
			MD5:       "d41d8cd98f00b204e9800998ecf8427e",
			Mime:      "text/plain",
			InStorage: true,
		}
		err = dao.FileContent().InsertOrIncrement(fileContent)
		if err != nil {
			t.Fatalf("Failed to insert file_content: %s", err)
		}

		// Upload to S3
		err = s3ao.PutObjectByHash(sha256, strings.NewReader(content), int64(len(content)))
		if err != nil {
			t.Fatalf("Failed to upload to S3: %s", err)
		}

		// Create two file records
		file1 := &ds.File{
			Filename: "file1.txt",
			Bin:      bin1.Id,
			Bytes:    uint64(len(content)),
			SHA256:   sha256,
		}
		dao.File().Insert(file1)

		file2 := &ds.File{
			Filename: "file2.txt",
			Bin:      bin2.Id,
			Bytes:    uint64(len(content)),
			SHA256:   sha256,
		}
		dao.File().Insert(file2)

		// Delete both files
		file1.DeletedAt.Scan(time.Now().UTC())
		dao.File().Update(file1)

		file2.DeletedAt.Scan(time.Now().UTC())
		dao.File().Update(file2)

		// Simulate lurker: check for pending deletes
		pending, err := dao.FileContent().GetPendingDelete()
		if err != nil {
			t.Fatalf("Failed to get pending deletes: %s", err)
		}
		if len(pending) != 1 {
			t.Errorf("Expected 1 pending delete, got %d", len(pending))
		}

		// Simulate lurker: remove from S3
		err = s3ao.RemoveObjectByHash(sha256)
		if err != nil {
			t.Fatalf("Failed to remove from S3: %s", err)
		}

		// Simulate lurker: mark as not in storage
		dbContent, err := dao.FileContent().GetBySHA256(sha256)
		if err != nil {
			t.Fatalf("Failed to get file_content: %s", err)
		}
		dbContent.InStorage = false
		err = dao.FileContent().Update(dbContent)
		if err != nil {
			t.Fatalf("Failed to update file_content: %s", err)
		}

		// Verify content is not in S3
		_, err = s3ao.StatObject(sha256)
		if err == nil {
			t.Error("Content should not exist in S3 after lurker cleanup")
		}

		// Verify in_storage is false
		dbContent, err = dao.FileContent().GetBySHA256(sha256)
		if err != nil {
			t.Fatalf("Failed to get file_content: %s", err)
		}
		if dbContent.InStorage {
			t.Error("in_storage should be false after lurker cleanup")
		}

		// Now re-upload to a third bin
		bin3 := &ds.Bin{
			Id:        "reupload-bin3",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		dao.Bin().Insert(bin3)

		// Upload to S3 again
		err = s3ao.PutObjectByHash(sha256, strings.NewReader(content), int64(len(content)))
		if err != nil {
			t.Fatalf("Failed to re-upload to S3: %s", err)
		}

		// Call InsertOrIncrement with InStorage: true (simulating upload handler)
		reuploadContent := &ds.FileContent{
			SHA256:    sha256,
			Bytes:     uint64(len(content)),
			MD5:       "d41d8cd98f00b204e9800998ecf8427e",
			Mime:      "text/plain",
			InStorage: true,
		}
		err = dao.FileContent().InsertOrIncrement(reuploadContent)
		if err != nil {
			t.Fatalf("Failed to update file_content on re-upload: %s", err)
		}

		// Create new file record
		file3 := &ds.File{
			Filename: "file3.txt",
			Bin:      bin3.Id,
			Bytes:    uint64(len(content)),
			SHA256:   sha256,
		}
		dao.File().Insert(file3)

		// Verify in_storage is now true
		finalContent, err := dao.FileContent().GetBySHA256(sha256)
		if err != nil {
			t.Fatalf("Failed to get file_content after re-upload: %s", err)
		}
		if !finalContent.InStorage {
			t.Error("in_storage should be true after re-upload")
		}

		// Verify content exists in S3
		_, err = s3ao.StatObject(sha256)
		if err != nil {
			t.Errorf("Content should exist in S3 after re-upload: %s", err)
		}

		// Cleanup - remove bins, S3 object, and file_content record
		dao.Bin().Delete(bin1)
		dao.Bin().Delete(bin2)
		dao.Bin().Delete(bin3)
		s3ao.RemoveObjectByHash(sha256)
		dao.FileContent().Delete(sha256)
	})

	t.Run("delete_bins_then_reupload", func(t *testing.T) {
		sha256_v2 := "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752"
		content_v2 := "test content for bin deletion"

		// Create two bins
		bin4 := &ds.Bin{
			Id:        "reupload-bin4",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		dao.Bin().Insert(bin4)

		bin5 := &ds.Bin{
			Id:        "reupload-bin5",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		dao.Bin().Insert(bin5)

		// Upload same content to both bins
		fileContent := &ds.FileContent{
			SHA256:    sha256_v2,
			Bytes:     uint64(len(content_v2)),
			MD5:       "d41d8cd98f00b204e9800998ecf8427e",
			Mime:      "text/plain",
			InStorage: true,
		}
		err = dao.FileContent().InsertOrIncrement(fileContent)
		if err != nil {
			t.Fatalf("Failed to insert file_content: %s", err)
		}

		// Upload to S3
		err = s3ao.PutObjectByHash(sha256_v2, strings.NewReader(content_v2), int64(len(content_v2)))
		if err != nil {
			t.Fatalf("Failed to upload to S3: %s", err)
		}

		// Create two file records
		file4 := &ds.File{
			Filename: "file4.txt",
			Bin:      bin4.Id,
			Bytes:    uint64(len(content_v2)),
			SHA256:   sha256_v2,
		}
		dao.File().Insert(file4)

		file5 := &ds.File{
			Filename: "file5.txt",
			Bin:      bin5.Id,
			Bytes:    uint64(len(content_v2)),
			SHA256:   sha256_v2,
		}
		dao.File().Insert(file5)

		// Delete both bins (soft delete)
		bin4.DeletedAt.Scan(time.Now().UTC())
		dao.Bin().Update(bin4)

		bin5.DeletedAt.Scan(time.Now().UTC())
		dao.Bin().Update(bin5)

		// Simulate lurker: check for pending deletes
		// Content should be pending because all files belong to deleted bins
		pending, err := dao.FileContent().GetPendingDelete()
		if err != nil {
			t.Fatalf("Failed to get pending deletes: %s", err)
		}
		if len(pending) != 1 {
			t.Errorf("Expected 1 pending delete (bins deleted), got %d", len(pending))
		}

		// Simulate lurker: remove from S3
		err = s3ao.RemoveObjectByHash(sha256_v2)
		if err != nil {
			t.Fatalf("Failed to remove from S3: %s", err)
		}

		// Simulate lurker: mark as not in storage
		dbContent, err := dao.FileContent().GetBySHA256(sha256_v2)
		if err != nil {
			t.Fatalf("Failed to get file_content: %s", err)
		}
		dbContent.InStorage = false
		err = dao.FileContent().Update(dbContent)
		if err != nil {
			t.Fatalf("Failed to update file_content: %s", err)
		}

		// Verify content is not in S3
		_, err = s3ao.StatObject(sha256_v2)
		if err == nil {
			t.Error("Content should not exist in S3 after lurker cleanup")
		}

		// Verify in_storage is false
		dbContent, err = dao.FileContent().GetBySHA256(sha256_v2)
		if err != nil {
			t.Fatalf("Failed to get file_content: %s", err)
		}
		if dbContent.InStorage {
			t.Error("in_storage should be false after lurker cleanup")
		}

		// Now re-upload to a third bin
		bin6 := &ds.Bin{
			Id:        "reupload-bin6",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		dao.Bin().Insert(bin6)

		// Upload to S3 again
		err = s3ao.PutObjectByHash(sha256_v2, strings.NewReader(content_v2), int64(len(content_v2)))
		if err != nil {
			t.Fatalf("Failed to re-upload to S3: %s", err)
		}

		// Call InsertOrIncrement with InStorage: true (simulating upload handler)
		reuploadContent := &ds.FileContent{
			SHA256:    sha256_v2,
			Bytes:     uint64(len(content_v2)),
			MD5:       "d41d8cd98f00b204e9800998ecf8427e",
			Mime:      "text/plain",
			InStorage: true,
		}
		err = dao.FileContent().InsertOrIncrement(reuploadContent)
		if err != nil {
			t.Fatalf("Failed to update file_content on re-upload: %s", err)
		}

		// Create new file record
		file6 := &ds.File{
			Filename: "file6.txt",
			Bin:      bin6.Id,
			Bytes:    uint64(len(content_v2)),
			SHA256:   sha256_v2,
		}
		dao.File().Insert(file6)

		// Verify in_storage is now true
		finalContent, err := dao.FileContent().GetBySHA256(sha256_v2)
		if err != nil {
			t.Fatalf("Failed to get file_content after re-upload: %s", err)
		}
		if !finalContent.InStorage {
			t.Error("in_storage should be true after re-upload (bin deletion scenario)")
		}

		// Verify content exists in S3
		_, err = s3ao.StatObject(sha256_v2)
		if err != nil {
			t.Errorf("Content should exist in S3 after re-upload: %s", err)
		}

		// Cleanup - remove bins, S3 object, and file_content record
		dao.Bin().Delete(bin4)
		dao.Bin().Delete(bin5)
		dao.Bin().Delete(bin6)
		s3ao.RemoveObjectByHash(sha256_v2)
		dao.FileContent().Delete(sha256_v2)
	})
}
