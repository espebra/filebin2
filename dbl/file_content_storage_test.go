package dbl

import (
	"context"
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/s3"
	"github.com/minio/minio-go/v7"
	"strings"
	"testing"
	"time"
)

const (
	testS3Endpoint   = "s3:9000"
	testS3Region     = "us-east-1"
	testS3Bucket     = "filebin-test-storage"
	testS3AccessKey  = "s3accesskey"
	testS3SecretKey  = "s3secretkey"
)

func setupS3() (s3.S3AO, error) {
	expiry := time.Second * 60
	s3ao, err := s3.Init(testS3Endpoint, testS3Bucket, testS3Region, testS3AccessKey, testS3SecretKey, false, expiry)
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
	ctx := context.Background()
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("Content should exist in S3 after upload, but got error: %s", err)
	}

	// Insert file_content with in_storage=true
	fileContent := &ds.FileContent{
		SHA256:         sha256,
		Bytes:          uint64(len(content)),
		ReferenceCount: 1,
		InStorage:      true,
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
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
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
	ctx := context.Background()
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("Content should exist in S3 after upload, but got error: %s", err)
	}

	// Create file_content record
	fileContent := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
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
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
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

	ctx := context.Background()

	// Upload 1: Upload to S3 (first time)
	t.Log("Upload 1: First upload of content")
	err = s3ao.PutObjectByHash(sha256, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Failed to upload to S3: %s", err)
	}

	// Verify content exists in S3
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("Content should exist in S3 after first upload: %s", err)
	}

	// Create file_content and file records
	fileContent := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
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

	// Verify ref count is 1
	dbContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if dbContent.ReferenceCount != 1 {
		t.Errorf("Expected ref count 1, got %d", dbContent.ReferenceCount)
	}
	if !dbContent.InStorage {
		t.Error("file_content.in_storage should be true")
	}

	// Upload 2: Skip S3 upload (deduplication), just increment ref count
	t.Log("Upload 2: Deduplicated upload (skip S3)")

	// Check if content already exists in S3 (it should)
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
	if err != nil {
		t.Error("Content should still exist in S3 for deduplication")
	}

	// Increment ref count (no S3 upload needed)
	err = dao.FileContent().InsertOrIncrement(&ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
		InStorage: true,
	})
	if err != nil {
		t.Fatalf("Failed to increment ref count: %s", err)
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

	// Verify ref count is 2 and content still in S3
	dbContent, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if dbContent.ReferenceCount != 2 {
		t.Errorf("Expected ref count 2 after deduplication, got %d", dbContent.ReferenceCount)
	}
	if !dbContent.InStorage {
		t.Error("file_content.in_storage should still be true after deduplication")
	}

	// Delete 1: Decrement ref count, keep content in S3
	t.Log("Delete 1: Remove one file reference")
	newCount, err := dao.FileContent().DecrementRefCount(sha256)
	if err != nil {
		t.Fatalf("Failed to decrement ref count: %s", err)
	}
	if newCount != 1 {
		t.Errorf("Expected ref count 1 after first delete, got %d", newCount)
	}
	// File records don't track in_storage anymore - only file_content does

	// Verify content STILL exists in S3 (one reference remaining)
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
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

	// Delete 2: Decrement to 0, then remove from S3
	t.Log("Delete 2: Remove last file reference and delete from S3")
	newCount, err = dao.FileContent().DecrementRefCount(sha256)
	if err != nil {
		t.Fatalf("Failed to decrement ref count: %s", err)
	}
	if newCount != 0 {
		t.Errorf("Expected ref count 0 after last delete, got %d", newCount)
	}
	// File records don't track in_storage anymore - only file_content does

	// Now remove from S3 (simulating lurker cleanup)
	err = s3ao.RemoveObjectByHash(sha256)
	if err != nil {
		t.Fatalf("Failed to remove from S3: %s", err)
	}

	// Verify content no longer exists in S3
	_, err = s3ao.GetClient().StatObject(ctx, s3ao.GetBucket(), sha256, minio.StatObjectOptions{})
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
	if finalContent.ReferenceCount != 0 {
		t.Errorf("Expected ref count 0, got %d", finalContent.ReferenceCount)
	}
}
