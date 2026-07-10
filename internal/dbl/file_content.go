package dbl

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
)

type FileContentDao struct {
	db      *sql.DB
	lockDB  *sql.DB
	metrics DBMetricsObserver
}

// nullString converts empty string to nil (SQL NULL), non-empty to *string
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetBySHA256 retrieves a file content record by its SHA256 hash
func (d *FileContentDao) GetBySHA256(sha256 string) (*ds.FileContent, error) {
	var content ds.FileContent
	var phash sql.NullString
	sqlStatement := "SELECT sha256, bytes, md5, mime, phash, in_storage, blocked, created_at, last_referenced_at FROM file_content WHERE sha256 = $1"
	t0 := time.Now()
	err := d.db.QueryRow(sqlStatement, sha256).Scan(
		&content.SHA256,
		&content.Bytes,
		&content.MD5,
		&content.Mime,
		&phash,
		&content.InStorage,
		&content.Blocked,
		&content.CreatedAt,
		&content.LastReferencedAt,
	)
	observeQuery(d.metrics, "file_content_get_by_sha256", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("File content not found")
		}
		return nil, err
	}
	content.PHash = phash.String
	content.BytesReadable = humanize.Bytes(content.Bytes)
	return &content, nil
}

// LockContent acquires a PostgreSQL advisory lock scoped to the given SHA256,
// blocking until the lock is available. The lock serializes the sections that
// couple file_content.in_storage to the existence of the S3 object: the
// upload path that verifies/uploads the object and records references, and
// the lurker's claim-and-delete. While the lock is held, no S3 delete of the
// object can be in flight, so in_storage and a StatObject check are
// authoritative.
//
// The returned unlock function is idempotent and must be called to release
// the lock. The lock is held by a dedicated database session (from a separate
// connection pool, so lock holders cannot starve regular queries) and is
// released automatically by PostgreSQL if the session dies.
func (d *FileContentDao) LockContent(sha256 string) (unlock func(), err error) {
	t0 := time.Now()
	unlock, _, err = d.lockContent(sha256, false)
	observeQuery(d.metrics, "file_content_lock", t0, err)
	return unlock, err
}

// TryLockContent is the non-blocking variant of LockContent. If the lock is
// held elsewhere it returns acquired=false without waiting.
func (d *FileContentDao) TryLockContent(sha256 string) (unlock func(), acquired bool, err error) {
	t0 := time.Now()
	unlock, acquired, err = d.lockContent(sha256, true)
	observeQuery(d.metrics, "file_content_try_lock", t0, err)
	return unlock, acquired, err
}

func (d *FileContentDao) lockContent(sha256 string, try bool) (func(), bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := d.lockDB.Conn(ctx)
	if err != nil {
		return nil, false, err
	}

	// hashtextextended maps the SHA256 hex string to the bigint key space of
	// pg_advisory_lock. A collision between different hashes only causes
	// needless serialization, never incorrectness.
	if try {
		var acquired bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1, 0))", sha256).Scan(&acquired); err != nil {
			_ = conn.Close()
			return nil, false, err
		}
		if !acquired {
			_ = conn.Close()
			return nil, false, nil
		}
	} else {
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", sha256); err != nil {
			_ = conn.Close()
			return nil, false, err
		}
	}

	var once sync.Once
	unlock := func() {
		once.Do(func() {
			// Fresh context: the unlock must be attempted even if the
			// caller's request context is long gone.
			unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer unlockCancel()
			var released bool
			err := conn.QueryRowContext(unlockCtx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", sha256).Scan(&released)
			if err != nil || !released {
				// Discard the session rather than returning a connection that
				// may still hold the lock to the pool. PostgreSQL releases
				// advisory locks when the session ends.
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			}
			_ = conn.Close()
		})
	}
	return unlock, true, nil
}

// InsertOrIncrement inserts a new file content record or updates last_referenced_at if it already exists.
//
// Callers passing InStorage=true must hold the content lock (LockContent) for
// this SHA256 and must have verified that the S3 object exists (or uploaded
// it) while holding the lock. Otherwise this write can clobber a deletion
// claim made by the lurker and record content as stored when its object is
// about to be, or already is, deleted.
func (d *FileContentDao) InsertOrIncrement(content *ds.FileContent) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := `INSERT INTO file_content (sha256, bytes, md5, mime, phash, in_storage, created_at, last_referenced_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (sha256) DO UPDATE SET
    in_storage = EXCLUDED.in_storage,
    phash = COALESCE(EXCLUDED.phash, file_content.phash),
    last_referenced_at = EXCLUDED.last_referenced_at`

	t0 := time.Now()
	_, err := d.db.Exec(sqlStatement,
		content.SHA256,
		content.Bytes,
		content.MD5,
		content.Mime,
		nullString(content.PHash),
		content.InStorage,
		now,
		now,
	)
	observeQuery(d.metrics, "file_content_insert_or_increment", t0, err)

	if err != nil {
		return err
	}

	content.CreatedAt = now
	content.LastReferencedAt = now
	return nil
}

// GetPendingDelete returns file content records that have zero active references
// (active = file exists AND file not deleted AND bin not deleted AND bin not expired) and still in storage
func (d *FileContentDao) GetPendingDelete() ([]ds.FileContent, error) {
	sqlStatement := `SELECT fc.sha256, fc.bytes, fc.md5, fc.mime, fc.phash, fc.in_storage, fc.blocked, fc.created_at, fc.last_referenced_at
FROM file_content fc
LEFT JOIN file f ON fc.sha256 = f.sha256
LEFT JOIN bin b ON f.bin_id = b.id
WHERE fc.in_storage = true
GROUP BY fc.sha256, fc.bytes, fc.md5, fc.mime, fc.phash, fc.in_storage, fc.blocked, fc.created_at, fc.last_referenced_at
HAVING COUNT(CASE WHEN f.id IS NOT NULL AND f.deleted_at IS NULL AND b.deleted_at IS NULL AND b.expired_at > NOW() THEN 1 END) = 0
ORDER BY fc.last_referenced_at ASC`

	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement)
	observeQuery(d.metrics, "file_content_get_pending_delete", t0, err)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var contents []ds.FileContent
	for rows.Next() {
		var content ds.FileContent
		var phash sql.NullString
		err := rows.Scan(
			&content.SHA256,
			&content.Bytes,
			&content.MD5,
			&content.Mime,
			&phash,
			&content.InStorage,
			&content.Blocked,
			&content.CreatedAt,
			&content.LastReferencedAt,
		)
		if err != nil {
			return nil, err
		}
		content.PHash = phash.String
		content.BytesReadable = humanize.Bytes(content.Bytes)
		contents = append(contents, content)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return contents, nil
}

// Update modifies an existing file content record
func (d *FileContentDao) Update(content *ds.FileContent) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := `UPDATE file_content
SET bytes = $2, md5 = $3, mime = $4, phash = COALESCE($5, phash), in_storage = $6, last_referenced_at = $7
WHERE sha256 = $1`

	t0 := time.Now()
	res, err := d.db.Exec(sqlStatement,
		content.SHA256,
		content.Bytes,
		content.MD5,
		content.Mime,
		nullString(content.PHash),
		content.InStorage,
		now,
	)
	observeQuery(d.metrics, "file_content_update", t0, err)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("File content does not exist")
	}

	content.LastReferencedAt = now
	return nil
}

// ClaimForDeletion atomically marks content as no longer in storage, but only
// if it still has zero active file references (active = file not deleted AND
// bin not deleted AND bin not expired). It returns true if the claim succeeded,
// meaning the caller now owns deleting the corresponding S3 object.
//
// The caller must hold the content lock (LockContent/TryLockContent) for this
// SHA256 across the claim and the subsequent S3 delete. The lock excludes
// concurrent uploads of the same content; the claim must still be committed
// before the S3 object is removed so that once the lock is released, a
// deduplicated upload observes in_storage=false and re-uploads the content.
// The NOT EXISTS guard makes the claim fail if an upload created an active
// reference in the meantime, so referenced content is never deleted.
func (d *FileContentDao) ClaimForDeletion(sha256 string) (bool, error) {
	sqlStatement := `UPDATE file_content fc
SET in_storage = false
WHERE fc.sha256 = $1
  AND fc.in_storage = true
  AND NOT EXISTS (
    SELECT 1 FROM file f
    JOIN bin b ON f.bin_id = b.id
    WHERE f.sha256 = fc.sha256
      AND f.deleted_at IS NULL
      AND b.deleted_at IS NULL
      AND b.expired_at > NOW()
  )`
	t0 := time.Now()
	res, err := d.db.Exec(sqlStatement, sha256)
	observeQuery(d.metrics, "file_content_claim_for_deletion", t0, err)
	if err != nil {
		return false, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetInStorage updates only the in_storage flag for a content record. It is
// used to roll back a deletion claim if the subsequent S3 delete fails, so the
// object (which is still present in S3) is retried on a later run rather than
// being leaked as an orphan.
func (d *FileContentDao) SetInStorage(sha256 string, inStorage bool) error {
	sqlStatement := `UPDATE file_content SET in_storage = $2 WHERE sha256 = $1`
	t0 := time.Now()
	_, err := d.db.Exec(sqlStatement, sha256, inStorage)
	observeQuery(d.metrics, "file_content_set_in_storage", t0, err)
	return err
}

// Delete removes a file content record from the database
func (d *FileContentDao) Delete(sha256 string) error {
	sqlStatement := "DELETE FROM file_content WHERE sha256 = $1"
	t0 := time.Now()
	res, err := d.db.Exec(sqlStatement, sha256)
	observeQuery(d.metrics, "file_content_delete", t0, err)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("File content does not exist")
	}
	return nil
}

// GetAll returns all file content records
func (d *FileContentDao) GetAll() ([]ds.FileContent, error) {
	sqlStatement := `SELECT sha256, bytes, md5, mime, phash, in_storage, blocked, created_at, last_referenced_at
FROM file_content
ORDER BY bytes DESC, created_at DESC`

	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement)
	observeQuery(d.metrics, "file_content_get_all", t0, err)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var contents []ds.FileContent
	for rows.Next() {
		var content ds.FileContent
		var phash sql.NullString
		err := rows.Scan(
			&content.SHA256,
			&content.Bytes,
			&content.MD5,
			&content.Mime,
			&phash,
			&content.InStorage,
			&content.Blocked,
			&content.CreatedAt,
			&content.LastReferencedAt,
		)
		if err != nil {
			return nil, err
		}
		content.PHash = phash.String
		content.BytesReadable = humanize.Bytes(content.Bytes)
		contents = append(contents, content)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return contents, nil
}

// BlockContent marks content as blocked and soft-deletes all file references
func (d *FileContentDao) BlockContent(sha256 string) (retErr error) {
	t0 := time.Now()
	defer func() { observeQuery(d.metrics, "file_content_block", t0, retErr) }()

	// Start a transaction to ensure atomicity
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Mark all file references with this SHA256 as deleted
	sqlDeleteFiles := `UPDATE file SET deleted_at = $1 WHERE sha256 = $2 AND deleted_at IS NULL`
	_, err = tx.Exec(sqlDeleteFiles, now, sha256)
	if err != nil {
		return err
	}

	// Mark the file content as blocked
	sqlBlockContent := `UPDATE file_content SET blocked = true WHERE sha256 = $1`
	res, err := tx.Exec(sqlBlockContent, sha256)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("File content does not exist")
	}

	// Commit the transaction
	return tx.Commit()
}

// UnblockContent unblocks content by setting blocked = false
func (d *FileContentDao) UnblockContent(sha256 string) error {
	sqlStatement := `UPDATE file_content SET blocked = false WHERE sha256 = $1`
	t0 := time.Now()
	res, err := d.db.Exec(sqlStatement, sha256)
	observeQuery(d.metrics, "file_content_unblock", t0, err)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("File content does not exist")
	}

	return nil
}

// DeleteFileReferences soft-deletes all file references for a given SHA256 without blocking the content
func (d *FileContentDao) DeleteFileReferences(sha256 string) error {
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Mark all file references with this SHA256 as deleted
	sqlDeleteFiles := `UPDATE file SET deleted_at = $1 WHERE sha256 = $2 AND deleted_at IS NULL`
	t0 := time.Now()
	res, err := d.db.Exec(sqlDeleteFiles, now, sha256)
	observeQuery(d.metrics, "file_content_delete_refs", t0, err)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("no file references found for this content")
	}

	return nil
}

func (d *FileContentDao) GetByCreated(limit int) (contents []ds.FileByChecksum, err error) {
	sqlStatement := `SELECT fc.sha256, COUNT(f.sha256) as c, fc.mime, fc.bytes,
		COUNT(f.sha256) * fc.bytes AS bytes_total,
		COALESCE(SUM(f.downloads), 0),
		COALESCE(SUM(f.updates), 0),
		fc.blocked,
		fc.created_at,
		fc.last_referenced_at
		FROM file_content fc
		LEFT JOIN file f ON fc.sha256 = f.sha256 AND f.deleted_at IS NULL
		WHERE fc.in_storage = true
		GROUP BY fc.sha256, fc.mime, fc.bytes, fc.blocked, fc.created_at, fc.last_referenced_at
		ORDER BY fc.created_at DESC
		LIMIT $1`

	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, limit)
	observeQuery(d.metrics, "file_content_get_by_created", t0, err)
	if err != nil {
		return contents, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var content ds.FileByChecksum
		err = rows.Scan(&content.SHA256, &content.Count, &content.Mime, &content.Bytes,
			&content.BytesTotal, &content.DownloadsTotal, &content.UpdatesTotal,
			&content.Blocked, &content.CreatedAt, &content.LastReferencedAt)
		if err != nil {
			return contents, err
		}
		content.CreatedAt = content.CreatedAt.UTC()
		content.LastReferencedAt = content.LastReferencedAt.UTC()
		content.CreatedAtRelative = humanize.Time(content.CreatedAt)
		content.LastReferencedAtRelative = humanize.Time(content.LastReferencedAt)
		content.BytesReadable = humanize.Bytes(content.Bytes)
		content.BytesTotalReadable = humanize.Bytes(content.BytesTotal)
		contents = append(contents, content)
	}
	if err = rows.Err(); err != nil {
		return contents, err
	}
	return contents, nil
}

func (d *FileContentDao) GetBlocked(limit int) (contents []ds.FileByChecksum, err error) {
	sqlStatement := `SELECT fc.sha256, COUNT(f.sha256) as c, fc.mime, fc.bytes,
		COUNT(f.sha256) * fc.bytes AS bytes_total,
		COALESCE(SUM(f.downloads), 0),
		COALESCE(SUM(f.updates), 0),
		fc.blocked,
		fc.created_at,
		fc.last_referenced_at
		FROM file_content fc
		LEFT JOIN file f ON fc.sha256 = f.sha256 AND f.deleted_at IS NULL
		WHERE fc.blocked = true
		GROUP BY fc.sha256, fc.mime, fc.bytes, fc.blocked, fc.created_at, fc.last_referenced_at
		ORDER BY fc.last_referenced_at DESC
		LIMIT $1`

	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, limit)
	observeQuery(d.metrics, "file_content_get_blocked", t0, err)
	if err != nil {
		return contents, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var content ds.FileByChecksum
		err = rows.Scan(&content.SHA256, &content.Count, &content.Mime, &content.Bytes,
			&content.BytesTotal, &content.DownloadsTotal, &content.UpdatesTotal,
			&content.Blocked, &content.CreatedAt, &content.LastReferencedAt)
		if err != nil {
			return contents, err
		}
		content.CreatedAt = content.CreatedAt.UTC()
		content.LastReferencedAt = content.LastReferencedAt.UTC()
		content.CreatedAtRelative = humanize.Time(content.CreatedAt)
		content.LastReferencedAtRelative = humanize.Time(content.LastReferencedAt)
		content.BytesReadable = humanize.Bytes(content.Bytes)
		content.BytesTotalReadable = humanize.Bytes(content.BytesTotal)
		contents = append(contents, content)
	}
	if err = rows.Err(); err != nil {
		return contents, err
	}
	return contents, nil
}
