package dbl

import (
	"fmt"
	"github.com/espebra/filebin2/internal/ds"
	"testing"
	"time"
)

func TestGetBinById(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	id := "1234567890"
	bin := &ds.Bin{}
	bin.Id = id
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	dbBin, found, err := dao.Bin().GetByID(id)
	if err != nil {
		t.Error(err)
	}
	if found == false {
		t.Errorf("Expected found to be true as the bin exists.")
	}
	if dbBin.Id != id {
		t.Errorf("Was expecting bin id %s, got %s instead.", id, dbBin.Id)
	}
	if dbBin.Files != 0 {
		t.Errorf("Was expecting number of files to be 0, got %d\n", dbBin.Files)
	}

	err = dao.Bin().RegisterDownload(bin)
	if err != nil {
		t.Error(err)
	}
	if bin.Downloads != 1 {
		t.Errorf("Was expecting the number of downloads to be 1, not %d\n", bin.Downloads)
	}

	if bin.Bytes != 0 {
		t.Errorf("Was expecting bytes to be 0, not %d\n", bin.Bytes)
	}

	if bin.Files != 0 {
		t.Errorf("Was expecting number of files to be 0, not %d\n", bin.Files)
	}
}

func TestInsertDuplicatedBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890"
	inserted, err := dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}
	if !inserted {
		t.Error("Expected the first insert to succeed")
	}

	inserted, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}
	if inserted {
		t.Error("Expected the second insert to be a no-op due to conflict")
	}
}

func TestBinTooLong(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890123456789012345678901234567890123456789012345678901"
	_, err = dao.Bin().Insert(bin)
	if err == nil {
		t.Errorf("Was expecting an error here, the bin id is too long.")
	}
}

func TestGetAllBins(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	count := 50
	for i := 0; i < count; i++ {
		bin := &ds.Bin{}
		bin.Id = fmt.Sprintf("somebin-%d", i)
		bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
		if _, err := dao.Bin().Insert(bin); err != nil {
			t.Error(err)
			break
		}
	}

	bins, err := dao.Bin().GetAll()
	if err != nil {
		t.Error(err)
	}

	if len(bins) != count {
		t.Errorf("Was expecting to find %d bins, got %d instead.", count, len(bins))
	}
}

func TestDeleteBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890"
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	dbBin, _, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}

	err = dao.Bin().Delete(&dbBin)
	if err != nil {
		t.Error(err)
	}

	_, found, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Errorf("Did not expect an error even though the bin was deleted earlier: %s\n", err.Error())
	}
	if found == true {
		t.Errorf("Expected found to be false as the bin was deleted earlier.")
	}
}

func TestMarkDeletedBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890"
	bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	deleted, err := dao.Bin().MarkDeleted(bin)
	if err != nil {
		t.Error(err)
	}
	if !deleted {
		t.Errorf("Was expecting MarkDeleted to succeed on an existing bin")
	}

	dbBin, _, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}
	if !dbBin.IsDeleted() {
		t.Errorf("Was expecting the bin to be deleted")
	}

	// A second delete is a no-op
	deleted, err = dao.Bin().MarkDeleted(bin)
	if err != nil {
		t.Error(err)
	}
	if deleted {
		t.Errorf("Was expecting MarkDeleted to return false on an already deleted bin")
	}

	// Non-existing bins are not an error, just not deleted
	missing := &ds.Bin{Id: "does-not-exist"}
	deleted, err = dao.Bin().MarkDeleted(missing)
	if err != nil {
		t.Error(err)
	}
	if deleted {
		t.Errorf("Was expecting MarkDeleted to return false on a non-existing bin")
	}
}

// TestLockDoesNotResurrectDeletedBin pins the delete-vs-lock race: a lock that
// loses the race against a concurrent delete must not resurrect the bin.
func TestLockDoesNotResurrectDeletedBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890"
	bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	// Lock works on a live bin
	locked, err := dao.Bin().Lock(bin)
	if err != nil {
		t.Error(err)
	}
	if !locked {
		t.Errorf("Was expecting Lock to succeed on an existing bin")
	}

	if _, err := dao.Bin().MarkDeleted(bin); err != nil {
		t.Error(err)
	}

	// Lock and Approve on a deleted bin fail and leave deleted_at intact
	locked, err = dao.Bin().Lock(bin)
	if err != nil {
		t.Error(err)
	}
	if locked {
		t.Errorf("Was expecting Lock to return false on a deleted bin")
	}
	approved, err := dao.Bin().Approve(bin)
	if err != nil {
		t.Error(err)
	}
	if approved {
		t.Errorf("Was expecting Approve to return false on a deleted bin")
	}
	updated, err := dao.Bin().TouchUpdatedAt(bin)
	if err != nil {
		t.Error(err)
	}
	if updated {
		t.Errorf("Was expecting TouchUpdatedAt to return false on a deleted bin")
	}

	dbBin, _, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}
	if !dbBin.IsDeleted() {
		t.Errorf("Was expecting the bin to still be deleted")
	}
}

func TestApproveBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890"
	bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	approved, err := dao.Bin().Approve(bin)
	if err != nil {
		t.Error(err)
	}
	if !approved {
		t.Errorf("Was expecting Approve to succeed on an existing bin")
	}

	dbBin, _, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}
	if !dbBin.IsApproved() {
		t.Errorf("Was expecting the bin to be approved")
	}
}

// TestMarkDeletedIfExpiredRevival pins the lurker-vs-upload race: a bin that a
// concurrent upload revived (by extending its expiration through Touch) must
// not be deleted by the lurker's expiry cleanup.
func TestMarkDeletedIfExpiredRevival(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890"
	bin.ExpiredAt = time.Now().UTC().Add(-time.Hour)
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}

	// An upload revives the expired bin by extending its expiration, as
	// uploadFile does between the lurker's GetPendingDelete and its delete.
	bin.ExpiredAt = time.Now().UTC().Add(time.Hour)
	touched, err := dao.Bin().Touch(bin)
	if err != nil {
		t.Error(err)
	}
	if !touched {
		t.Errorf("Was expecting Touch to succeed on a live bin")
	}

	// The lurker's guarded delete leaves the revived bin alone
	deleted, err := dao.Bin().MarkDeletedIfExpired(bin)
	if err != nil {
		t.Error(err)
	}
	if deleted {
		t.Errorf("Was expecting MarkDeletedIfExpired to return false on a revived bin")
	}
	dbBin, _, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}
	if dbBin.IsDeleted() {
		t.Errorf("Was expecting the revived bin to not be deleted")
	}

	// Once actually expired, the guarded delete succeeds
	bin.ExpiredAt = time.Now().UTC().Add(-time.Minute)
	if _, err := dao.Bin().Touch(bin); err != nil {
		t.Error(err)
	}
	deleted, err = dao.Bin().MarkDeletedIfExpired(bin)
	if err != nil {
		t.Error(err)
	}
	if !deleted {
		t.Errorf("Was expecting MarkDeletedIfExpired to succeed on an expired bin")
	}
}

func TestDeleteNonExistingBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "1234567890"
	err = dao.Bin().Delete(bin)
	if err == nil {
		t.Errorf("Was expecting an error here, bin %v does not exist.", bin)
	}
}

func TestInvalidBinInput(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	bin := &ds.Bin{}
	bin.Id = "12345"
	_, err = dao.Bin().Insert(bin)
	if err == nil {
		t.Error("Expected an error since bin is too short")
	}

	bin.Id = "..."
	_, err = dao.Bin().Insert(bin)
	if err == nil {
		t.Error("Expected an error since bin is invalid")
	}

	bin.Id = "%&/()"
	_, err = dao.Bin().Insert(bin)
	if err == nil {
		t.Error("Expected an error since bin contains invalid characters")
	}
}

func TestFileCount(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	type testcase struct {
		Bin   string
		Files uint64
		Bytes uint64
	}

	testcases := []testcase{
		{
			Bin:   "firstbin",
			Files: 10,
			Bytes: 1,
		}, {
			Bin:   "secondbin",
			Files: 20,
			Bytes: 2,
		}, {
			Bin:   "thirdbin",
			Files: 30,
			Bytes: 100,
		},
	}

	// Create file_content records for each unique byte size
	// In content-addressable storage, files with same content (SHA256) must have same size
	sha256ForBytes := map[uint64]string{
		1:   "1111111111111111111111111111111111111111111111111111111111111111",
		2:   "2222222222222222222222222222222222222222222222222222222222222222",
		100: "0000000000000000000000000000000000000000000000000000000000000064",
	}

	for bytes, sha256 := range sha256ForBytes {
		content := &ds.FileContent{
			SHA256:    sha256,
			Bytes:     bytes,
			MD5:       "d41d8cd98f00b204e9800998ecf8427e",
			Mime:      "application/octet-stream",
			InStorage: true,
		}
		err = dao.FileContent().InsertOrIncrement(content)
		if err != nil {
			t.Error(err)
		}
	}

	for _, tc := range testcases {
		bin := &ds.Bin{}
		bin.Id = tc.Bin
		bin.ExpiredAt = time.Now().UTC().Add(time.Hour * 1)
		_, err = dao.Bin().Insert(bin)
		if err != nil {
			t.Error(err)
		}

		for i := 0; i < int(tc.Files); i++ {
			// Create some files
			file := &ds.File{}
			file.Bin = bin.Id // Foreign key
			file.Filename = fmt.Sprintf("testfile-%d", i)
			// Use different SHA256 for different byte sizes (content-addressable storage)
			file.SHA256 = sha256ForBytes[tc.Bytes]
			_, err = dao.File().Insert(file)
			if err != nil {
				t.Error(err)
			}
		}

		dbBin, found, err := dao.Bin().GetByID(bin.Id)
		if err != nil {
			t.Error(err)
		}
		if found == false {
			t.Errorf("Expected found to be true as the bin exists.")
		}
		if dbBin.Files != tc.Files {
			t.Errorf("Was expecting number of files in bin %s to be %d, got %d instead.\n", bin.Id, tc.Files, dbBin.Files)
		}
		if dbBin.Bytes != (tc.Bytes * tc.Files) {
			t.Errorf("Was expecting %d bytes in total in bin %s, got %d instead.\n", (tc.Bytes * tc.Files), bin.Id, dbBin.Bytes)
		}
	}

	dbBins, err := dao.Bin().GetAll()
	if err != nil {
		t.Error(err)
	}

	if len(dbBins) != len(testcases) {
		t.Errorf("Was expecting %d bins, got %d.\n", len(testcases), len(dbBins))
	}

	for _, bin := range dbBins {
		for _, tc := range testcases {
			if bin.Id == tc.Bin {
				if bin.Files != tc.Files {
					t.Errorf("Was expecting %d files in bin %s, got %d.\n", tc.Files, bin.Id, bin.Files)
				}
				if bin.Bytes != (tc.Files * tc.Bytes) {
					t.Errorf("Was expecting %d bytes in bin %s, got %d.\n", (tc.Files * tc.Bytes), bin.Id, bin.Bytes)
				}
			}
		}
	}
}

func TestReviveBin(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = tearDown(dao) }()

	// Revive a deleted bin
	bin := &ds.Bin{}
	bin.Id = "revivedeleted"
	_, err = dao.Bin().Insert(bin)
	if err != nil {
		t.Error(err)
	}
	deleted, err := dao.Bin().MarkDeleted(bin)
	if err != nil {
		t.Error(err)
	}
	if !deleted {
		t.Errorf("Was expecting MarkDeleted to succeed")
	}

	expiredAt := time.Now().UTC().Add(time.Hour)
	revived, err := dao.Bin().Revive(bin, expiredAt)
	if err != nil {
		t.Error(err)
	}
	if !revived {
		t.Errorf("Was expecting Revive to succeed on a deleted bin")
	}

	dbBin, found, err := dao.Bin().GetByID(bin.Id)
	if err != nil {
		t.Error(err)
	}
	if !found {
		t.Errorf("Expected found to be true as the bin exists.")
	}
	if dbBin.IsDeleted() {
		t.Errorf("Was expecting the revived bin to not be deleted")
	}
	if dbBin.IsExpired() {
		t.Errorf("Was expecting the revived bin to not be expired")
	}
	if !dbBin.IsReadable() {
		t.Errorf("Was expecting the revived bin to be readable")
	}

	// Revive an expired bin
	expiredBin := &ds.Bin{}
	expiredBin.Id = "reviveexpired"
	expiredBin.ExpiredAt = time.Now().UTC().Add(-time.Hour)
	_, err = dao.Bin().Insert(expiredBin)
	if err != nil {
		t.Error(err)
	}

	revived, err = dao.Bin().Revive(expiredBin, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Error(err)
	}
	if !revived {
		t.Errorf("Was expecting Revive to succeed on an expired bin")
	}

	dbBin, _, err = dao.Bin().GetByID(expiredBin.Id)
	if err != nil {
		t.Error(err)
	}
	if dbBin.IsExpired() {
		t.Errorf("Was expecting the revived bin to not be expired")
	}

	// Revive a bin that does not exist
	missingBin := &ds.Bin{}
	missingBin.Id = "nosuchbin"
	revived, err = dao.Bin().Revive(missingBin, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Error(err)
	}
	if revived {
		t.Errorf("Was expecting Revive to return false for a bin that does not exist")
	}
}
