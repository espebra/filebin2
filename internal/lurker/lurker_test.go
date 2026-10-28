package lurker

import (
	"bufio"
	sha256pkg "crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/audit"
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
	l.backfillPause = 0
	return l
}

// seedContent stores an object in S3 and creates the corresponding
// file_content record with in_storage=true and no file references. It
// returns the SHA256 of the content, which is its key.
func seedContent(t *testing.T, dao dbl.DAO, s3ao s3.S3AO, content string) string {
	t.Helper()
	sha256 := fmt.Sprintf("%x", sha256pkg.Sum256([]byte(content)))
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
	return sha256
}

// TestDeletePendingContentDeletesOrphans verifies that content without any
// active file reference is claimed and removed from S3.
func TestDeletePendingContentDeletesOrphans(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao, s3ao)

	sha256 := seedContent(t, dao, s3ao, "orphaned content")

	l := newTestLurker(&dao, &s3ao)
	l.DeletePendingContent()

	if _, err := s3ao.StatObject(sha256); err == nil {
		t.Error("Object should have been removed from S3")
	}
	dbContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if dbContent.InStorage {
		t.Error("in_storage should be false after deletion")
	}
}

// TestAuditEvents verifies that expired bins and deleted content are
// written to the audit log.
func TestAuditEvents(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao, s3ao)

	bin := &ds.Bin{Id: "expiredauditbin", ExpiredAt: time.Now().Add(-time.Hour)}
	if _, err := dao.Bin().Insert(bin); err != nil {
		t.Fatalf("Failed to insert bin: %s", err)
	}
	sha256 := seedContent(t, dao, s3ao, "orphaned audit content")

	dir := t.TempDir()
	auditLog, err := audit.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	l := newTestLurker(&dao, &s3ao)
	l.SetAuditLogger(auditLog, 0)
	l.DeletePendingBins()
	l.DeletePendingContent()
	auditLog.Close()

	paths, err := filepath.Glob(filepath.Join(dir, "audit-*.jsonl"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("Expected one audit log file, got %v (%v)", paths, err)
	}
	f, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var events []audit.Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e audit.Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("Invalid audit log line: %s", err)
		}
		events = append(events, e)
	}
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %+v", events)
	}
	if events[0].Event != "bin_expired" || events[0].Bin != bin.Id || events[0].Actor != audit.ActorLurker {
		t.Errorf("Unexpected bin event: %+v", events[0])
	}
	if events[1].Event != "content_deleted" || events[1].SHA256 != sha256 || events[1].Actor != audit.ActorLurker {
		t.Errorf("Unexpected content event: %+v", events[1])
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

	content := "referenced content"
	sha256 := seedContent(t, dao, s3ao, content)

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
	dbContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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

	sha256 := seedContent(t, dao, s3ao, "content locked by an upload")

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
	dbContent, found, err := dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
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
	dbContent, found, err = dao.FileContent().GetBySHA256(sha256)
	if err != nil || !found {
		t.Fatalf("Failed to get file_content: %s", err)
	}
	if dbContent.InStorage {
		t.Error("in_storage should be false after deletion")
	}
}

// TestBackfillDeletedFiles verifies that files left active in bins that were
// deleted before files were deleted together with their bin are soft-deleted
// with the deletion time of the bin, and that files in active bins are left
// alone.
func TestBackfillDeletedFiles(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer tearDown(dao, s3ao)

	sha256 := seedContent(t, dao, s3ao, "backfill content")

	insertFile := func(binID string) *ds.File {
		t.Helper()
		bin := &ds.Bin{Id: binID, ExpiredAt: time.Now().UTC().Add(time.Hour)}
		if _, err := dao.Bin().Insert(bin); err != nil {
			t.Fatalf("Failed to insert bin: %s", err)
		}
		file := &ds.File{Filename: "file.txt", Bin: bin.Id, Bytes: 1, SHA256: sha256}
		if _, err := dao.File().Insert(file); err != nil {
			t.Fatalf("Failed to insert file: %s", err)
		}
		return file
	}

	// Simulate a bin deleted before files were deleted together with it:
	// delete the bin, then make its file active again.
	legacyFile := insertFile("backfilllegacy")
	legacyBin, _, err := dao.Bin().GetByID("backfilllegacy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dao.Bin().MarkDeleted(&legacyBin); err != nil {
		t.Fatal(err)
	}
	dbLegacyFile, _, err := dao.File().GetByID(legacyFile.Id)
	if err != nil {
		t.Fatal(err)
	}
	dbLegacyFile.DeletedAt = sql.NullTime{}
	if err := dao.File().Update(&dbLegacyFile); err != nil {
		t.Fatal(err)
	}

	activeFile := insertFile("backfillactive")

	l := newTestLurker(&dao, &s3ao)
	l.BackfillDeletedFiles()

	if !l.backfillDone {
		t.Error("Was expecting the backfill to be done")
	}
	dbLegacyFile, _, err = dao.File().GetByID(legacyFile.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !dbLegacyFile.IsDeleted() {
		t.Error("Was expecting the file in the deleted bin to be soft-deleted")
	}
	if !dbLegacyFile.DeletedAt.Time.Equal(legacyBin.DeletedAt.Time) {
		t.Errorf("Was expecting file deleted_at %s to match bin deleted_at %s", dbLegacyFile.DeletedAt.Time, legacyBin.DeletedAt.Time)
	}
	dbActiveFile, _, err := dao.File().GetByID(activeFile.Id)
	if err != nil {
		t.Fatal(err)
	}
	if dbActiveFile.IsDeleted() {
		t.Error("Was expecting the file in the active bin to stay active")
	}

	// A new lurker finds nothing to do and does not start a pass.
	l = newTestLurker(&dao, &s3ao)
	l.BackfillDeletedFiles()
	if !l.backfillDone || l.backfillStarted {
		t.Errorf("Was expecting the backfill to be done without starting a pass, got done=%t started=%t", l.backfillDone, l.backfillStarted)
	}
}
