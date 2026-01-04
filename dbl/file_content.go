package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/espebra/filebin2/ds"
	"time"
)

type FileContentDao struct {
	db *sql.DB
}

// GetBySHA256 retrieves a file content record by its SHA256 hash
func (d *FileContentDao) GetBySHA256(sha256 string) (*ds.FileContent, error) {
	var content ds.FileContent
	sqlStatement := "SELECT sha256, bytes, downloads, in_storage, created_at, last_referenced_at FROM file_content WHERE sha256 = $1"
	err := d.db.QueryRow(sqlStatement, sha256).Scan(
		&content.SHA256,
		&content.Bytes,
		&content.Downloads,
		&content.InStorage,
		&content.CreatedAt,
		&content.LastReferencedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("File content not found")
		}
		return nil, err
	}
	return &content, nil
}

// InsertOrIncrement inserts a new file content record or updates last_referenced_at if it already exists
func (d *FileContentDao) InsertOrIncrement(content *ds.FileContent) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := `INSERT INTO file_content (sha256, bytes, in_storage, created_at, last_referenced_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (sha256) DO UPDATE SET
    in_storage = EXCLUDED.in_storage,
    last_referenced_at = EXCLUDED.last_referenced_at`

	_, err := d.db.Exec(sqlStatement,
		content.SHA256,
		content.Bytes,
		content.InStorage,
		now,
		now,
	)

	if err != nil {
		return err
	}

	content.CreatedAt = now
	content.LastReferencedAt = now
	return nil
}

// IncrementDownloads atomically increments the download counter for content
func (d *FileContentDao) IncrementDownloads(sha256 string) error {
	sqlStatement := `UPDATE file_content
SET downloads = downloads + 1
WHERE sha256 = $1`

	res, err := d.db.Exec(sqlStatement, sha256)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("File content not found")
	}

	return nil
}

// GetPendingDelete returns file content records that have zero active references
// (active = file exists AND file not deleted AND bin not deleted) and still in storage
func (d *FileContentDao) GetPendingDelete() ([]ds.FileContent, error) {
	sqlStatement := `SELECT fc.sha256, fc.bytes, fc.downloads, fc.in_storage, fc.created_at, fc.last_referenced_at
FROM file_content fc
LEFT JOIN file f ON fc.sha256 = f.sha256
LEFT JOIN bin b ON f.bin_id = b.id
WHERE fc.in_storage = true
GROUP BY fc.sha256, fc.bytes, fc.downloads, fc.in_storage, fc.created_at, fc.last_referenced_at
HAVING COUNT(CASE WHEN f.id IS NOT NULL AND f.deleted_at IS NULL AND b.deleted_at IS NULL THEN 1 END) = 0
ORDER BY fc.last_referenced_at ASC`

	rows, err := d.db.Query(sqlStatement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contents []ds.FileContent
	for rows.Next() {
		var content ds.FileContent
		err := rows.Scan(
			&content.SHA256,
			&content.Bytes,
			&content.Downloads,
			&content.InStorage,
			&content.CreatedAt,
			&content.LastReferencedAt,
		)
		if err != nil {
			return nil, err
		}
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
SET bytes = $2, in_storage = $3, last_referenced_at = $4
WHERE sha256 = $1`

	res, err := d.db.Exec(sqlStatement,
		content.SHA256,
		content.Bytes,
		content.InStorage,
		now,
	)
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

// Delete removes a file content record from the database
func (d *FileContentDao) Delete(sha256 string) error {
	sqlStatement := "DELETE FROM file_content WHERE sha256 = $1"
	res, err := d.db.Exec(sqlStatement, sha256)
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
	sqlStatement := `SELECT sha256, bytes, downloads, in_storage, created_at, last_referenced_at
FROM file_content
ORDER BY bytes DESC, created_at DESC`

	rows, err := d.db.Query(sqlStatement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contents []ds.FileContent
	for rows.Next() {
		var content ds.FileContent
		err := rows.Scan(
			&content.SHA256,
			&content.Bytes,
			&content.Downloads,
			&content.InStorage,
			&content.CreatedAt,
			&content.LastReferencedAt,
		)
		if err != nil {
			fmt.Printf("Error scanning row: %s\n", err.Error())
			continue
		}
		contents = append(contents, content)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return contents, nil
}
