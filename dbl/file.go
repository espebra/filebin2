package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type FileDao struct {
	db *sql.DB
}

func setCategory(file *ds.File) {
	if strings.HasPrefix(file.Mime, "image") {
		file.Category = "image"
	} else if strings.HasPrefix(file.Mime, "video") {
		file.Category = "video"
	} else {
		file.Category = "unknown"
	}
}

func (d *FileDao) ValidateInput(file *ds.File) error {
	// Trim whitespace before and after the filename.
	file.Filename = strings.TrimSpace(file.Filename)

	// If the filename is empty, error out.
	if len(file.Filename) == 0 {
		return errors.New("Filename not specified")
	}

	// Create a new variable to use for filename modifications.
	n := file.Filename

	// Extract the basename from the filename, in case the filename
	// is not clean and contains a folder structure.
	// folder structure.
	n = filepath.Base(n)

	// Replace all invalid UTF-8 characters in the filename with _
	n = strings.ToValidUTF8(n, "_")

	// Mapping function to replace non-safe characters with underscore.
	// It is possible that this filter can be extended to allow more
	// unicode categories.
	safe := func(r rune) rune {
		switch {
		// Allow numbers
		case unicode.IsNumber(r):
			//fmt.Printf("Character check: r=%q is a number\n", r)
			return r
		// Allow letters
		case unicode.IsLetter(r):
			//fmt.Printf("Character check: r=%q is a letter\n", r)
			return r
		// Allow certain other characters
		case strings.ContainsAny(string(r), "-_=+,.()[] "):
			//fmt.Printf("Character check: r=%q is a valid character\n", r)
			return r
		}
		//fmt.Printf("Invalid character (%q) in filename replaced with underscore\n", r)
		// All other characters are replaced with an underscore
		return '_'

	}
	n = strings.Map(safe, n)

	// . is not allowed as the first character
	if strings.HasPrefix(n, ".") {
		n = strings.Replace(n, ".", "_", 1)
	}

	// Replace redundant spaces with single spaces
	n = strings.Join(strings.Fields(n), " ")

	// Truncate long filenames
	// XXX: The maximum length could be made configurable
	if len(n) > 120 {
		fmt.Printf("Truncating filename to 120 characters. Counting %d characters in %q.\n", len(n), n)
		n = n[:120]
	}

	if file.Filename != n {
		// Log that the filename was modified.
		fmt.Printf("Modifying filename during upload from %q to %q (%s)\n", file.Filename, n, file.Bin)
	}

	file.Filename = n

	return nil
}

func (d *FileDao) GetByID(id int) (file ds.File, found bool, err error) {
	sqlStatement := "SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.client_id, f.headers, f.updated_at, f.created_at, f.deleted_at FROM file f JOIN file_content fc ON f.sha256 = fc.sha256 WHERE f.id = $1 LIMIT 1"
	err = d.db.QueryRow(sqlStatement, id).Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.InStorage, &file.IP, &file.ClientId, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, false, nil
		}
		return file, false, err
	}
	// https://github.com/lib/pq/issues/329
	file.UpdatedAt = file.UpdatedAt.UTC()
	file.CreatedAt = file.CreatedAt.UTC()
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.CreatedAtRelative = humanize.Time(file.CreatedAt)
	if file.IsDeleted() {
		file.DeletedAt.Time = file.DeletedAt.Time.UTC()
		file.DeletedAtRelative = humanize.Time(file.DeletedAt.Time)
	}
	file.BytesReadable = humanize.Bytes(file.Bytes)
	return file, true, nil
}

func (d *FileDao) GetByName(bin string, filename string) (file ds.File, found bool, err error) {
	sqlStatement := "SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.client_id, f.headers, f.updated_at, f.created_at, f.deleted_at FROM file f JOIN file_content fc ON f.sha256 = fc.sha256 WHERE f.bin_id = $1 AND f.filename = $2 LIMIT 1"
	err = d.db.QueryRow(sqlStatement, bin, filename).Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.InStorage, &file.IP, &file.ClientId, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, false, nil
		}
		return file, false, err
	}
	// https://github.com/lib/pq/issues/329
	file.UpdatedAt = file.UpdatedAt.UTC()
	file.CreatedAt = file.CreatedAt.UTC()
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.CreatedAtRelative = humanize.Time(file.CreatedAt)
	if file.IsDeleted() {
		file.DeletedAt.Time = file.DeletedAt.Time.UTC()
		file.DeletedAtRelative = humanize.Time(file.DeletedAt.Time)
	}
	file.BytesReadable = humanize.Bytes(file.Bytes)
	setCategory(&file)
	return file, true, nil
}

// IsAvailableForDownload checks if a file is available for download by verifying:
// - The file is not deleted
// - The bin is not deleted
// - The bin has not expired
// - The file content exists in storage
func (d *FileDao) IsAvailableForDownload(fileId int) (bool, error) {
	var available bool
	sqlStatement := `
		SELECT EXISTS(
			SELECT 1
			FROM file f
			JOIN bin b ON f.bin_id = b.id
			JOIN file_content fc ON f.sha256 = fc.sha256
			WHERE f.id = $1
				AND f.deleted_at IS NULL
				AND b.deleted_at IS NULL
				AND b.expired_at > NOW()
				AND fc.in_storage = true
		)`

	err := d.db.QueryRow(sqlStatement, fileId).Scan(&available)
	if err != nil {
		return false, err
	}

	return available, nil
}

func (d *FileDao) Insert(file *ds.File) (err error) {
	if err := d.ValidateInput(file); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	downloads := 0
	updates := 0

	// Some kind of default value, not NULL
	if file.IP == "" {
		file.IP = "N/A"
	}
	if file.Headers == "" {
		file.Headers = "N/A"
	}

	sqlStatement := "INSERT INTO file (bin_id, filename, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id"
	err = d.db.QueryRow(sqlStatement, file.Bin, file.Filename, file.SHA256, downloads, updates, file.IP, file.ClientId, file.Headers, now, now, file.DeletedAt).Scan(&file.Id)
	if err != nil {
		return err
	}
	file.Downloads = uint64(downloads)
	file.Updates = uint64(updates)
	file.UpdatedAt = now
	file.CreatedAt = now
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.CreatedAtRelative = humanize.Time(file.CreatedAt)
	if file.IsDeleted() {
		file.DeletedAtRelative = humanize.Time(file.DeletedAt.Time)
	}
	file.BytesReadable = humanize.Bytes(file.Bytes)
	setCategory(file)
	return nil
}

func (d *FileDao) Update(file *ds.File) (err error) {
	var id int
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE file SET filename = $1, sha256 = $2, updates = $3, updated_at = $4, deleted_at = $5, ip = $6, headers = $7, client_id = $8 WHERE id = $9 RETURNING id"
	err = d.db.QueryRow(sqlStatement, file.Filename, file.SHA256, file.Updates, now, file.DeletedAt, file.IP, file.Headers, file.ClientId, file.Id).Scan(&id)
	if err != nil {
		return err
	}
	file.UpdatedAt = now
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	if file.IsDeleted() {
		file.DeletedAtRelative = humanize.Time(file.DeletedAt.Time)
	}
	file.BytesReadable = humanize.Bytes(file.Bytes)
	setCategory(file)
	return nil
}

func (d *FileDao) Delete(file *ds.File) (err error) {
	sqlStatement := "DELETE FROM file WHERE id = $1"
	res, err := d.db.Exec(sqlStatement, file.Id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("File does not exist")
	}
	return nil
}

func (d *FileDao) RegisterDownload(file *ds.File) (err error) {
	sqlStatement := "UPDATE file SET downloads = downloads + 1 WHERE id = $1 RETURNING downloads"
	err = d.db.QueryRow(sqlStatement, file.Id).Scan(&file.Downloads)
	if err != nil {
		return err
	}
	return nil
}

func (d *FileDao) GetByBin(id string, inStorage bool) (files []ds.File, err error) {
	// Join with file_content to check if content is actually in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.client_id, f.headers, f.updated_at, f.created_at, f.deleted_at
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE f.bin_id = $1 AND fc.in_storage = $2 AND f.deleted_at IS NULL
		ORDER BY f.filename ASC`
	files, err = d.fileQuery(sqlStatement, id, inStorage)
	return files, err
}

func (d *FileDao) GetAll(available bool) (files []ds.File, err error) {
	// Join with file_content to check if content is actually in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.client_id, f.headers, f.updated_at, f.created_at, f.deleted_at
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE fc.in_storage = $1 AND f.deleted_at IS NULL
		ORDER BY f.filename ASC`
	files, err = d.fileQuery(sqlStatement, available)
	return files, err
}

func (d *FileDao) GetPendingDelete() (files []ds.File, err error) {
	// Files are soft-deleted by setting deleted_at. No further action needed by lurker.
	// Content cleanup is handled by FileContent.GetPendingDelete() which checks for orphaned content.
	// Return empty list to avoid logging "pending removal" for already-deleted files.
	return files, nil
}

func (d *FileDao) GetTopDownloads(limit int) (files []ds.File, err error) {
	// Join with file_content to only show files whose content is still in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.client_id, f.headers, f.updated_at, f.created_at, f.deleted_at
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		ORDER BY f.downloads DESC LIMIT $1`
	files, err = d.fileQuery(sqlStatement, limit)
	return files, err
}

func (d *FileDao) fileQuery(sqlStatement string, params ...interface{}) (files []ds.File, err error) {
	rows, err := d.db.Query(sqlStatement, params...)
	if err != nil {
		return files, err
	}
	defer rows.Close()
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.InStorage, &file.IP, &file.ClientId, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
		if err != nil {
			return files, err
		}
		// https://github.com/lib/pq/issues/329
		file.UpdatedAt = file.UpdatedAt.UTC()
		file.CreatedAt = file.CreatedAt.UTC()
		file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
		file.CreatedAtRelative = humanize.Time(file.CreatedAt)
		if file.IsDeleted() {
			file.DeletedAt.Time = file.DeletedAt.Time.UTC()
			file.DeletedAtRelative = humanize.Time(file.DeletedAt.Time)
		}
		file.BytesReadable = humanize.Bytes(file.Bytes)
		file.URL = path.Join("/", file.Bin, file.Filename)
		setCategory(&file)
		files = append(files, file)
	}
	return files, nil
}

func (d *FileDao) FilesByChecksum(limit int) (files []ds.FileByChecksum, err error) {
	// Join with file_content to only count files whose content is still in storage
	sqlStatement := `SELECT f.sha256, COUNT(f.sha256) as c, fc.mime, fc.bytes, COUNT(f.sha256) * fc.bytes AS bytes_total, SUM(f.downloads), SUM(f.updates)
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		GROUP BY f.sha256, fc.mime, fc.bytes
		ORDER BY c DESC LIMIT $1`

	rows, err := d.db.Query(sqlStatement, limit)
	if err != nil {
		return files, err
	}
	defer rows.Close()
	for rows.Next() {
		var file ds.FileByChecksum
		err = rows.Scan(&file.SHA256, &file.Count, &file.Mime, &file.Bytes, &file.BytesTotal, &file.DownloadsTotal, &file.UpdatesTotal)
		if err != nil {
			return files, err
		}
		file.BytesReadable = humanize.Bytes(file.Bytes)
		file.BytesTotalReadable = humanize.Bytes(file.BytesTotal)
		files = append(files, file)
	}
	return files, nil
}

func (d *FileDao) FileByChecksum(sha256 string) (files []ds.File, err error) {
	sqlStatement := "SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.client_id, f.headers, f.updated_at, f.created_at, f.deleted_at FROM file f JOIN file_content fc ON f.sha256 = fc.sha256 WHERE f.sha256 = $1 ORDER BY f.downloads DESC, f.updates DESC"
	files, err = d.fileQuery(sqlStatement, sha256)
	return files, err
}

// CountBySHA256 returns the count of active file references with the given SHA256
// (active = file not deleted AND bin not deleted)
func (d *FileDao) CountBySHA256(sha256 string) (int, error) {
	var count int
	sqlStatement := `SELECT COUNT(*) FROM file f
JOIN bin b ON f.bin_id = b.id
WHERE f.sha256 = $1 AND f.deleted_at IS NULL AND b.deleted_at IS NULL`
	err := d.db.QueryRow(sqlStatement, sha256).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
