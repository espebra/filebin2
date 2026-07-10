package dbl

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
)

var invalidBin = regexp.MustCompile("[^A-Za-z0-9-_.]")

type BinDao struct {
	db      *sql.DB
	metrics DBMetricsObserver
}

func (d *BinDao) ValidateInput(bin *ds.Bin) error {
	// Reject invalid bins
	if invalidBin.MatchString(bin.Id) {
		return errors.New("the bin contains invalid characters")
	}
	// Ensure decent length
	if len(bin.Id) < 8 {
		return errors.New("the bin is too short")
	}
	if len(bin.Id) > 60 {
		return errors.New("the bin is too long")
	}
	// Do not allow the bin to start with .
	if strings.HasPrefix(bin.Id, ".") {
		return errors.New("invalid bin specified")
	}
	if bin.UpdatedAt.After(bin.ExpiredAt) {
		return errors.New("the bin cannot be updated when it has expired")
	}
	return nil
}

func (d *BinDao) GenerateId() string {
	characters := []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	length := 16
	maxAttempts := 10
	charLen := big.NewInt(int64(len(characters)))

	for attempt := 0; attempt < maxAttempts; attempt++ {
		id := make([]rune, length)
		for i := range id {
			n, err := rand.Int(rand.Reader, charLen)
			if err != nil {
				// Fallback to first character on error (extremely rare)
				id[i] = characters[0]
				continue
			}
			id[i] = characters[n.Int64()]
		}
		idStr := string(id)

		// Skip uniqueness check if no database connection (e.g., in tests)
		if d.db == nil {
			return idStr
		}

		// Check if this ID already exists
		_, found, err := d.GetByID(idStr)
		if err != nil {
			// Database error, try again
			continue
		}
		if !found {
			// ID is unique
			return idStr
		}
		// ID exists, try again
	}

	// Fallback: return the last generated ID and let the database constraint catch duplicates
	// This should be extremely rare given the 36^16 ID space
	id := make([]rune, length)
	for i := range id {
		n, err := rand.Int(rand.Reader, charLen)
		if err != nil {
			id[i] = characters[0]
			continue
		}
		id[i] = characters[n.Int64()]
	}
	return string(id)
}

func (d *BinDao) GetByID(id string) (bin ds.Bin, found bool, err error) {
	// Get bin info
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename), bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id = file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.id = $1 GROUP BY bin.id LIMIT 1"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, id).Scan(&bin.Id, &bin.Readonly, &bin.Downloads, &bin.FileDownloads, &bin.Bytes, &bin.Files, &bin.UpdatedAt, &bin.CreatedAt, &bin.ApprovedAt, &bin.ExpiredAt, &bin.DeletedAt)
	observeQuery(d.metrics, "bin_get_by_id", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return bin, false, nil
		} else {
			return bin, false, err
		}
	}
	hydrateBin(&bin)
	return bin, true, nil
}

func (d *BinDao) Insert(bin *ds.Bin) (inserted bool, err error) {
	if err := d.ValidateInput(bin); err != nil {
		return false, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	downloads := uint64(0)
	updates := uint64(0)
	readonly := false
	bin.ExpiredAt = bin.ExpiredAt.UTC().Truncate(time.Microsecond)
	if bin.IsApproved() {
		bin.ApprovedAt.Time = bin.ApprovedAt.Time.UTC().Truncate(time.Microsecond)
	}
	sqlStatement := "INSERT INTO bin (id, readonly, downloads, updates, updated_at, created_at, approved_at, expired_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING RETURNING id"
	var id string
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, bin.Id, readonly, downloads, updates, now, now, bin.ApprovedAt, bin.ExpiredAt).Scan(&id)
	observeQuery(d.metrics, "bin_insert", t0, err)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// markDeleted soft-deletes the bin with a targeted, guarded update. The extra
// guard (e.g. an expiry re-check) is appended to the WHERE clause. Returns
// false if no matching row was found, meaning the bin was already deleted or
// the extra guard no longer holds.
func (d *BinDao) markDeleted(bin *ds.Bin, extraGuard string, operation string) (deleted bool, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL" + extraGuard + " RETURNING id"
	var id string
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, now, bin.Id).Scan(&id)
	observeQuery(d.metrics, operation, t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	_ = bin.DeletedAt.Scan(now)
	bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
	bin.UpdatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return true, nil
}

// MarkDeleted soft-deletes the bin unless it is already deleted. Targeted and
// guarded so it cannot revert concurrent changes to other moderation fields.
// Returns false if the bin does not exist or is already deleted.
func (d *BinDao) MarkDeleted(bin *ds.Bin) (deleted bool, err error) {
	return d.markDeleted(bin, "", "bin_mark_deleted")
}

// MarkDeletedIfExpired soft-deletes the bin only if it is still expired. The
// expiry re-check makes sure a bin that a concurrent upload just revived (by
// extending expired_at through Touch) is left alone. Returns false if the bin
// does not exist, is already deleted, or is no longer expired.
func (d *BinDao) MarkDeletedIfExpired(bin *ds.Bin) (deleted bool, err error) {
	return d.markDeleted(bin, " AND expired_at < NOW()", "bin_mark_deleted_if_expired")
}

// Lock makes the bin read only unless it has been deleted. Targeted and
// guarded so it cannot resurrect a bin that was deleted concurrently. Returns
// false if the bin does not exist or is deleted.
func (d *BinDao) Lock(bin *ds.Bin) (locked bool, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET readonly = true, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL RETURNING id"
	var id string
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, now, bin.Id).Scan(&id)
	observeQuery(d.metrics, "bin_lock", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	bin.Readonly = true
	bin.UpdatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return true, nil
}

// Approve sets the approval timestamp unless the bin has been deleted.
// Targeted and guarded so it cannot revert concurrent changes to other
// moderation fields. Returns false if the bin does not exist or is deleted.
func (d *BinDao) Approve(bin *ds.Bin) (approved bool, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET approved_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL RETURNING id"
	var id string
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, now, bin.Id).Scan(&id)
	observeQuery(d.metrics, "bin_approve", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	_ = bin.ApprovedAt.Scan(now)
	bin.ApprovedAtRelative = humanize.Time(bin.ApprovedAt.Time)
	bin.UpdatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return true, nil
}

// TouchUpdatedAt refreshes the bin's updated_at timestamp without touching
// expiration or any moderation field. Returns false if the bin does not exist
// or is deleted.
func (d *BinDao) TouchUpdatedAt(bin *ds.Bin) (updated bool, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET updated_at = $1 WHERE id = $2 AND deleted_at IS NULL RETURNING id"
	var id string
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, now, bin.Id).Scan(&id)
	observeQuery(d.metrics, "bin_touch_updated_at", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	bin.UpdatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return true, nil
}

// Touch refreshes the bin's updated_at and expired_at timestamps without
// touching the moderation fields (readonly, approved_at, deleted_at). It only
// affects bins that are still writable (not deleted and not readonly), so a
// slow, in-flight upload cannot resurrect or un-moderate a bin that an admin or
// the lurker changed concurrently while the upload was running. Returns false
// if no matching writable row was found (i.e. the bin was deleted or locked
// during the upload).
func (d *BinDao) Touch(bin *ds.Bin) (updated bool, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiredAt := bin.ExpiredAt.UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET updated_at = $1, expired_at = $2 WHERE id = $3 AND deleted_at IS NULL AND readonly = false RETURNING id"
	var id string
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, now, expiredAt, bin.Id).Scan(&id)
	observeQuery(d.metrics, "bin_touch", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	bin.UpdatedAt = now
	bin.ExpiredAt = expiredAt
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return true, nil
}

func (d *BinDao) Delete(bin *ds.Bin) (err error) {
	sqlStatement := "DELETE FROM bin WHERE id = $1"
	t0 := time.Now()
	res, err := d.db.Exec(sqlStatement, bin.Id)
	observeQuery(d.metrics, "bin_delete", t0, err)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("Bin does not exist")
	} else {
		return nil
	}
}

func (d *BinDao) RegisterDownload(bin *ds.Bin) (err error) {
	sqlStatement := "UPDATE bin SET downloads = downloads + 1 WHERE id = $1 RETURNING downloads"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, bin.Id).Scan(&bin.Downloads)
	observeQuery(d.metrics, "bin_register_download", t0, err)
	if err != nil {
		return err
	}
	return nil
}

func (d *BinDao) RegisterUpdate(bin *ds.Bin) (err error) {
	bin.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET updates = updates + 1, updated_at = $1 WHERE id = $2 RETURNING updates"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, bin.UpdatedAt, bin.Id).Scan(&bin.Updates)
	observeQuery(d.metrics, "bin_register_update", t0, err)
	if err != nil {
		return err
	}
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return nil
}

func (d *BinDao) GetAll() (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename), bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.expired_at > $1 AND bin.deleted_at IS NULL GROUP BY bin.id ORDER BY bin.updated_at DESC"
	bins, err = d.binQuery(sqlStatement, now)
	return bins, err
}

func (d *BinDao) GetPendingDelete() (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename) AS files, bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id = file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.expired_at < $1 AND bin.deleted_at IS NULL GROUP BY bin.id"
	bins, err = d.binQuery(sqlStatement, now)
	return bins, err
}

func (d *BinDao) GetLastUpdated(limit int) (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename), bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.expired_at > $1 AND bin.deleted_at IS NULL GROUP BY bin.id ORDER BY bin.updated_at DESC LIMIT $2"
	bins, err = d.binQuery(sqlStatement, now, limit)
	return bins, err
}

func (d *BinDao) GetByBytes(limit int) (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename), bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.expired_at > $1 AND bin.deleted_at IS NULL GROUP BY bin.id ORDER BY COALESCE(SUM(file_content.bytes), 0) DESC LIMIT $2"
	bins, err = d.binQuery(sqlStatement, now, limit)
	return bins, err
}

func (d *BinDao) GetByDownloads(limit int) (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename), bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.expired_at > $1 AND bin.deleted_at IS NULL GROUP BY bin.id ORDER BY bin.downloads + COALESCE(SUM(file.downloads), 0) DESC LIMIT $2"
	bins, err = d.binQuery(sqlStatement, now, limit)
	return bins, err
}

func (d *BinDao) GetByFiles(limit int) (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename), bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.expired_at > $1 AND bin.deleted_at IS NULL GROUP BY bin.id ORDER BY COUNT(file.filename) DESC LIMIT $2"
	bins, err = d.binQuery(sqlStatement, now, limit)
	return bins, err
}

func (d *BinDao) GetByCreated(limit int) (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.downloads), 0), COALESCE(SUM(file_content.bytes), 0), COUNT(file.filename), bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.deleted_at IS NULL LEFT JOIN file_content ON file.sha256 = file_content.sha256 AND file_content.in_storage = true WHERE bin.expired_at > $1 AND bin.deleted_at IS NULL GROUP BY bin.id ORDER BY bin.created_at ASC LIMIT $2"
	bins, err = d.binQuery(sqlStatement, now, limit)
	return bins, err
}

func (d *BinDao) binQuery(sqlStatement string, params ...interface{}) (bins []ds.Bin, err error) {
	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, params...)
	observeQuery(d.metrics, "bin_query", t0, err)
	if err != nil {
		return bins, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Downloads, &bin.FileDownloads, &bin.Bytes, &bin.Files, &bin.Updates, &bin.UpdatedAt, &bin.CreatedAt, &bin.ApprovedAt, &bin.ExpiredAt, &bin.DeletedAt)
		if err != nil {
			return bins, err
		}
		hydrateBin(&bin)
		bins = append(bins, bin)
	}
	if err = rows.Err(); err != nil {
		return bins, err
	}
	return bins, nil
}
