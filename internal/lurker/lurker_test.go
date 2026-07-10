package lurker

import (
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/s3"
)

const (
	testDbName      = "db"
	testDbUser      = "username"
	testDbPassword  = "changeme"
	testDbHost      = "db"
	testDbPort      = 5432
	testS3Endpoint  = "s3:5553"
	testS3Region    = "us-east-1"
	testS3Bucket    = "filebin-lurker-test"
	testS3AccessKey = "s3accesskey"
	testS3SecretKey = "s3secretkey"
)

func tearUp() (dbl.DAO, s3.S3AO, error) {
	dao, err := dbl.Init(dbl.DBConfig{
		Host:            testDbHost,
		Port:            testDbPort,
		Name:            testDbName,
		Username:        testDbUser,
		Password:        testDbPassword,
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	})
	if err != nil {
		return dao, s3.S3AO{}, err
	}
	if err := dao.ResetDB(); err != nil {
		return dao, s3.S3AO{}, err
	}
	s3ao, err := s3.Init(s3.Config{
		Endpoint:             testS3Endpoint,
		Bucket:               testS3Bucket,
		Region:               testS3Region,
		AccessKey:            testS3AccessKey,
		SecretKey:            testS3SecretKey,
		Secure:               false,
		PresignExpiry:        time.Second * 10,
		Timeout:              time.Second * 30,
		TransferTimeout:      time.Minute * 10,
		MultipartPartSize:    64 * 1024 * 1024, // 64 MB
		MultipartConcurrency: 3,
	})
	if err != nil {
		return dao, s3ao, err
	}
	return dao, s3ao, nil
}

func tearDown(dao dbl.DAO, s3ao s3.S3AO) {
	_ = dao.ResetDB()
	_ = dao.Close()
	_ = s3ao.RemoveBucket()
}

func newTestLurker(dao *dbl.DAO, s3ao *s3.S3AO) *Lurker {
	l := New(dao, s3ao, nil)
	l.Init(60, 0, 86400)
	return l
}

// seedContent stores an object in S3 and creates the corresponding
// file_content record with in_storage=true and no file references.
func seedContent(t *testing.T, dao dbl.DAO, s3ao s3.S3AO, sha256 string, content string) {
	t.Helper()
	if err := s3ao.PutObjectByHash(sha256, strings.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("Failed to upload to S3: %s", err)
	}
	fileContent := &ds.FileContent{
		SHA256:    sha256,
		Bytes:     uint64(len(content)),
		MD5:       "d41d8cd98f00b204e9800998ecf8427e",
		Mime:      "text/plain",
		InStorage: true,
	}
	if err := dao.FileContent().InsertOrIncrement(fileContent); err != nil {
		t.Fatalf("Failed to insert file_content: %s", err)
	}
}

// TestDeletePendingContentDeletesOrphans verifies that content without any
// active file reference is claimed and removed from S3.
func TestDeletePendingContentDeletesOrphans(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao, s3ao)

	sha256 := "1111111111111111111111111111111111111111111111111111111111111111"
	seedContent(t, dao, s3ao, sha256, "orphaned content")

	l := newTestLurker(&dao, &s3ao)
	l.DeletePendingContent()

	if _, err := s3ao.StatObject(sha256); err == nil {
		t.Error("Object should have been removed from S3")
	}
	dbContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if dbContent.InStorage {
		t.Error("in_storage should be false after deletion")
	}
}

// TestDeletePendingContentKeepsReferencedContent verifies that content with
// an active file reference in a valid bin is left alone.
func TestDeletePendingContentKeepsReferencedContent(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao, s3ao)

	sha256 := "2222222222222222222222222222222222222222222222222222222222222222"
	content := "referenced content"
	seedContent(t, dao, s3ao, sha256, content)

	bin := &ds.Bin{
		Id:        "lurkertestbin",
		ExpiredAt: time.Now().UTC().Add(time.Hour * 24),
	}
	if _, err := dao.Bin().Insert(bin); err != nil {
		t.Fatalf("Failed to insert bin: %s", err)
	}
	file := &ds.File{
		Filename: "file.txt",
		Bin:      bin.Id,
		Bytes:    uint64(len(content)),
		SHA256:   sha256,
	}
	if _, err := dao.File().Insert(file); err != nil {
		t.Fatalf("Failed to insert file: %s", err)
	}

	l := newTestLurker(&dao, &s3ao)
	l.DeletePendingContent()

	if _, err := s3ao.StatObject(sha256); err != nil {
		t.Errorf("Object with an active reference should still exist in S3: %s", err)
	}
	dbContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if !dbContent.InStorage {
		t.Error("in_storage should still be true for referenced content")
	}
}

// TestDeletePendingContentSkipsLockedContent deterministically verifies the
// race protection: while an upload holds the content lock, the lurker must
// not claim or delete the content, and must pick it up again on a later run
// once the lock is released.
func TestDeletePendingContentSkipsLockedContent(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao, s3ao)

	sha256 := "3333333333333333333333333333333333333333333333333333333333333333"
	seedContent(t, dao, s3ao, sha256, "content locked by an upload")

	// Simulate an in-flight upload of the same content holding the lock.
	unlock, err := dao.FileContent().LockContent(sha256)
	if err != nil {
		t.Fatal(err)
	}

	l := newTestLurker(&dao, &s3ao)
	l.DeletePendingContent()

	// The lurker must have skipped the locked content entirely.
	if _, err := s3ao.StatObject(sha256); err != nil {
		t.Errorf("Object should still exist in S3 while the content lock is held: %s", err)
	}
	dbContent, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if !dbContent.InStorage {
		t.Error("in_storage should still be true while the content lock is held")
	}

	unlock()

	// With the lock released and still no active reference, the next run
	// must delete the content.
	l.DeletePendingContent()

	if _, err := s3ao.StatObject(sha256); err == nil {
		t.Error("Object should have been removed from S3 after the lock was released")
	}
	dbContent, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if dbContent.InStorage {
		t.Error("in_storage should be false after deletion")
	}
}
