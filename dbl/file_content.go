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
	sqlStatement := "SELECT sha256, bytes, reference_count, in_storage, created_at, last_referenced_at FROM file_content WHERE sha256 = $1"
	err := d.db.QueryRow(sqlStatement, sha256).Scan(
		&content.SHA256,
		&content.Bytes,
		&content.ReferenceCount,
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

// InsertOrIncrement atomically inserts a new file content record or increments
// the reference count if it already exists
func (d *FileContentDao) InsertOrIncrement(content *ds.FileContent) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := `INSERT INTO file_content (sha256, bytes, reference_count, in_storage, created_at, last_referenced_at)
VALUES ($1, $2, 1, $3, $4, $5)
ON CONFLICT (sha256) DO UPDATE SET
    reference_count = file_content.reference_count + 1,
    last_referenced_at = $6
RETURNING reference_count`

	err := d.db.QueryRow(sqlStatement,
		content.SHA256,
		content.Bytes,
		content.InStorage,
		now,
		now,
		now,
	).Scan(&content.ReferenceCount)

	if err != nil {
		return err
	}

	content.CreatedAt = now
	content.LastReferencedAt = now
	return nil
}

// DecrementRefCount atomically decrements the reference count and returns the new count
func (d *FileContentDao) DecrementRefCount(sha256 string) (int, error) {
	var newCount int
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := `UPDATE file_content
SET reference_count = GREATEST(0, reference_count - 1),
    last_referenced_at = $2
WHERE sha256 = $1
RETURNING reference_count`

	err := d.db.QueryRow(sqlStatement, sha256, now).Scan(&newCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, errors.New("File content not found")
		}
		return 0, err
	}
	return newCount, nil
}

// GetPendingDelete returns file content records that have zero references
// and are still in storage (ready to be cleaned up by lurker)
func (d *FileContentDao) GetPendingDelete() ([]ds.FileContent, error) {
	sqlStatement := `SELECT sha256, bytes, reference_count, in_storage, created_at, last_referenced_at
FROM file_content
WHERE reference_count = 0 AND in_storage = true
ORDER BY last_referenced_at ASC`

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
			&content.ReferenceCount,
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
SET bytes = $2, reference_count = $3, in_storage = $4, last_referenced_at = $5
WHERE sha256 = $1`

	res, err := d.db.Exec(sqlStatement,
		content.SHA256,
		content.Bytes,
		content.ReferenceCount,
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
	sqlStatement := `SELECT sha256, bytes, reference_count, in_storage, created_at, last_referenced_at
FROM file_content
ORDER BY reference_count DESC, bytes DESC`

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
			&content.ReferenceCount,
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
