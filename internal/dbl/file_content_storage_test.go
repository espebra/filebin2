package dbl

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/s3"
)

const (
	testS3Endpoint  = "s3:5553"
	testS3Region    = "us-east-1"
	testS3Bucket    = "filebin-test-storage"
	testS3AccessKey = "s3accesskey"
	testS3SecretKey = "s3secretkey"
)

// contentSHA256 returns the hex encoded SHA256 of content, which is the key
// of the content in S3.
func contentSHA256(content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
}

func setupS3() (s3.S3AO, error) {
	s3ao, err := s3.Init(s3.Config{
		Endpoint:             testS3Endpoint,
		Bucket:               testS3Bucket,
		Region:               testS3Region,
		AccessKey:            testS3AccessKey,
		SecretKey:            testS3SecretKey,
		Secure:               false,
		PresignExpiry:        time.Second * 60,
		Timeout:              time.Second * 30,
		TransferTimeout:      time.Minute * 10,
		MultipartPartSize:    64 * 1024 * 1024, // 64 MB
		MultipartConcurrency: 3,
	})
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
	defer func() { _ = tearDown(dao) }()

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = teardownS3(s3ao) }()

	content := "test content"
	sha256 := contentSHA256(content)

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
	dbContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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
	updatedContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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
	defer func() { _ = tearDown(dao) }()

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = teardownS3(s3ao) }()

	content := "test file content"
	sha256 := contentSHA256(content)

	// Create bin
	bin := &ds.Bin{
		Id:        "teststoragebin",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	_, err = dao.Bin().Insert(bin)
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
	_, err = dao.File().Insert(file)
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
	dbContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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
	defer func() { _ = tearDown(dao) }()

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = teardownS3(s3ao) }()

	content := "duplicate content"
	sha256 := contentSHA256(content)

	// Create bin
	bin := &ds.Bin{
		Id:        "dedupbin123",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	_, err = dao.Bin().Insert(bin)
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
	_, err = dao.File().Insert(file1)
	if err != nil {
		t.Fatalf("Failed to insert file1: %s", err)
	}

	// Verify content exists
	dbContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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
	_, err = dao.File().Insert(file2)
	if err != nil {
		t.Fatalf("Failed to insert file2: %s", err)
	}

	// Verify content still in S3
	dbContent, found, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if !dbContent.InStorage {
		t.Error("file_content.in_storage should still be true after deduplication")
	}

	// Delete 1: Mark first file as deleted, keep content in S3
	t.Log("Delete 1: Remove one file reference")
	_ = file1.DeletedAt.Scan(time.Now().UTC())
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
	dbContent, found, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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
	_ = file2.DeletedAt.Scan(time.Now().UTC())
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
	dbContent, found, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
		t.Fatalf("Failed to get file_content before update: %s", err)
	}
	dbContent.InStorage = false
	err = dao.FileContent().Update(dbContent)
	if err != nil {
		t.Fatalf("Failed to update file_content: %s", err)
	}

	// Verify file_content.in_storage is false
	finalContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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
	defer func() { _ = tearDown(dao) }()

	s3ao, err := setupS3()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = teardownS3(s3ao) }()

	content := "test content for reupload"
	sha256 := contentSHA256(content)

	t.Run("delete_files_then_reupload", func(t *testing.T) {
		// Create two bins
		bin1 := &ds.Bin{
			Id:        "reupload-bin1",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		_, _ = dao.Bin().Insert(bin1)

		bin2 := &ds.Bin{
			Id:        "reupload-bin2",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		_, _ = dao.Bin().Insert(bin2)

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
		_, _ = dao.File().Insert(file1)

		file2 := &ds.File{
			Filename: "file2.txt",
			Bin:      bin2.Id,
			Bytes:    uint64(len(content)),
			SHA256:   sha256,
		}
		_, _ = dao.File().Insert(file2)

		// Delete both files
		_ = file1.DeletedAt.Scan(time.Now().UTC())
		_ = dao.File().Update(file1)

		_ = file2.DeletedAt.Scan(time.Now().UTC())
		_ = dao.File().Update(file2)

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
		dbContent, found, err := dao.FileContent().GetBySHA256(sha256)
		if err != nil || !found {
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
		dbContent, found, err = dao.FileContent().GetBySHA256(sha256)
		if err != nil || !found {
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
		_, _ = dao.Bin().Insert(bin3)

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
		_, _ = dao.File().Insert(file3)

		// Verify in_storage is now true
		finalContent, found, err := dao.FileContent().GetBySHA256(sha256)
		if err != nil || !found {
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
		_ = dao.Bin().Delete(bin1)
		_ = dao.Bin().Delete(bin2)
		_ = dao.Bin().Delete(bin3)
		_ = s3ao.RemoveObjectByHash(sha256)
		_ = dao.FileContent().Delete(sha256)
	})

	t.Run("delete_bins_then_reupload", func(t *testing.T) {
		content_v2 := "test content for bin deletion"
		sha256_v2 := contentSHA256(content_v2)

		// Create two bins
		bin4 := &ds.Bin{
			Id:        "reupload-bin4",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		_, _ = dao.Bin().Insert(bin4)

		bin5 := &ds.Bin{
			Id:        "reupload-bin5",
			ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
		}
		_, _ = dao.Bin().Insert(bin5)

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
		_, _ = dao.File().Insert(file4)

		file5 := &ds.File{
			Filename: "file5.txt",
			Bin:      bin5.Id,
			Bytes:    uint64(len(content_v2)),
			SHA256:   sha256_v2,
		}
		_, _ = dao.File().Insert(file5)

		// Delete both bins (soft delete)
		_, _ = dao.Bin().MarkDeleted(bin4)
		_, _ = dao.Bin().MarkDeleted(bin5)

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
		dbContent, found, err := dao.FileContent().GetBySHA256(sha256_v2)
		if err != nil || !found {
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
		dbContent, found, err = dao.FileContent().GetBySHA256(sha256_v2)
		if err != nil || !found {
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
		_, _ = dao.Bin().Insert(bin6)

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
		_, _ = dao.File().Insert(file6)

		// Verify in_storage is now true
		finalContent, found, err := dao.FileContent().GetBySHA256(sha256_v2)
		if err != nil || !found {
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
		_ = dao.Bin().Delete(bin4)
		_ = dao.Bin().Delete(bin5)
		_ = dao.Bin().Delete(bin6)
		_ = s3ao.RemoveObjectByHash(sha256_v2)
		_ = dao.FileContent().Delete(sha256_v2)
	})
}
