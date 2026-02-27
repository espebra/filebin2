package dbl

import (
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
)

type FileDao struct {
	db      *sql.DB
	metrics DBMetricsObserver
}

func (d *FileDao) ValidateInput(file *ds.File) error {
	// Trim whitespace before and after the filename.
	file.Filename = strings.TrimSpace(file.Filename)

	// If the filename is empty, error out.
	if len(file.Filename) == 0 {
		return errors.New("filename not specified")
	}

	// Create a new variable to use for filename modifications.
	n := file.Filename

	// Extract the basename from the filename, in case the filename
	// is not clean and contains a folder structure.
	n = filepath.Base(n)

	// Replace all invalid UTF-8 characters in the filename with _
	n = strings.ToValidUTF8(n, "_")

	// Mapping function to replace non-safe characters with underscore.
	// It is possible that this filter can be extended to allow more
	// unicode categories.
	safe := func(r rune) rune {
		switch {
		case unicode.IsNumber(r):
			return r
		case unicode.IsLetter(r):
			return r
		case strings.ContainsAny(string(r), "-_=+,.()[] "):
			return r
		}
		return '_'
	}
	n = strings.Map(safe, n)

	// Replace redundant spaces with single spaces
	n = strings.Join(strings.Fields(n), " ")

	// . is not allowed as the first character
	if strings.HasPrefix(n, ".") {
		n = strings.Replace(n, ".", "_", 1)
	}

	// Truncate long filenames
	// XXX: The maximum length could be made configurable
	if len(n) > 120 {
		slog.Debug("truncating filename to 120 characters", "original_length", len(n), "filename", n)
		n = strings.ToValidUTF8(strings.TrimRight(n[:120], " "), "_")
	}

	// Reject if the filename became empty after sanitization
	if len(n) == 0 {
		return errors.New("filename not specified")
	}

	if file.Filename != n {
		// Log that the filename was modified.
		slog.Debug("modifying filename during upload", "original", file.Filename, "modified", n, "bin", file.Bin)
	}

	file.Filename = n

	return nil
}

func (d *FileDao) GetByID(id int) (file ds.File, found bool, err error) {
	sqlStatement := "SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms FROM file f JOIN file_content fc ON f.sha256 = fc.sha256 LEFT JOIN bin b ON f.bin_id = b.id WHERE f.id = $1 LIMIT 1"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, id).Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.InStorage, &file.IP, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt, &file.BinDeletedAt, &file.BinExpiredAt, &file.UploadDurationMs)
	observeQuery(d.metrics, "file_get_by_id", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, false, nil
		}
		return file, false, err
	}
	hydrateFile(&file)
	return file, true, nil
}

func (d *FileDao) GetByName(bin string, filename string) (file ds.File, found bool, err error) {
	sqlStatement := "SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms FROM file f JOIN file_content fc ON f.sha256 = fc.sha256 LEFT JOIN bin b ON f.bin_id = b.id WHERE f.bin_id = $1 AND f.filename = $2 LIMIT 1"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, bin, filename).Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.InStorage, &file.IP, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt, &file.BinDeletedAt, &file.BinExpiredAt, &file.UploadDurationMs)
	observeQuery(d.metrics, "file_get_by_name", t0, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, false, nil
		}
		return file, false, err
	}
	hydrateFile(&file)
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

	t0 := time.Now()
	err := d.db.QueryRow(sqlStatement, fileId).Scan(&available)
	observeQuery(d.metrics, "file_is_available", t0, err)
	if err != nil {
		return false, err
	}

	return available, nil
}

func (d *FileDao) Insert(file *ds.File) (bool, error) {
	if err := d.ValidateInput(file); err != nil {
		return false, err
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

	sqlStatement := "INSERT INTO file (bin_id, filename, sha256, downloads, updates, ip, headers, updated_at, created_at, deleted_at, upload_duration_ms) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (bin_id, filename) DO NOTHING RETURNING id"
	t0 := time.Now()
	err := d.db.QueryRow(sqlStatement, file.Bin, file.Filename, file.SHA256, downloads, updates, file.IP, file.Headers, now, now, file.DeletedAt, file.UploadDurationMs).Scan(&file.Id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	observeQuery(d.metrics, "file_insert", t0, err)
	if err != nil {
		return false, err
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
	return true, nil
}

func (d *FileDao) Update(file *ds.File) (err error) {
	var id int
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE file SET filename = $1, sha256 = $2, updates = $3, updated_at = $4, deleted_at = $5, ip = $6, headers = $7, upload_duration_ms = $8 WHERE id = $9 RETURNING id"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, file.Filename, file.SHA256, file.Updates, now, file.DeletedAt, file.IP, file.Headers, file.UploadDurationMs, file.Id).Scan(&id)
	observeQuery(d.metrics, "file_update", t0, err)
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
	t0 := time.Now()
	res, err := d.db.Exec(sqlStatement, file.Id)
	observeQuery(d.metrics, "file_delete", t0, err)
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
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, file.Id).Scan(&file.Downloads)
	observeQuery(d.metrics, "file_register_download", t0, err)
	if err != nil {
		return err
	}
	return nil
}

func (d *FileDao) GetByBin(id string, inStorage bool) (files []ds.File, err error) {
	// Join with file_content to check if content is actually in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		LEFT JOIN bin b ON f.bin_id = b.id
		WHERE f.bin_id = $1 AND fc.in_storage = $2 AND f.deleted_at IS NULL
		ORDER BY f.filename ASC`
	files, err = d.fileQuery(sqlStatement, id, inStorage)
	return files, err
}

func (d *FileDao) GetAll(available bool) (files []ds.File, err error) {
	// Join with file_content to check if content is actually in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		LEFT JOIN bin b ON f.bin_id = b.id
		WHERE fc.in_storage = $1 AND f.deleted_at IS NULL
		ORDER BY f.filename ASC`
	files, err = d.fileQuery(sqlStatement, available)
	return files, err
}

func (d *FileDao) GetTopDownloads(limit int) (files []ds.File, err error) {
	// Join with file_content to only show files whose content is still in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		LEFT JOIN bin b ON f.bin_id = b.id
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		ORDER BY f.downloads DESC LIMIT $1`
	files, err = d.fileQuery(sqlStatement, limit)
	return files, err
}

func (d *FileDao) GetByCreated(limit int) (files []ds.File, err error) {
	// Join with file_content to only show files whose content is still in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		LEFT JOIN bin b ON f.bin_id = b.id
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		ORDER BY f.created_at DESC LIMIT $1`
	files, err = d.fileQuery(sqlStatement, limit)
	return files, err
}

func (d *FileDao) GetByUpdated(limit int) (files []ds.File, err error) {
	// Join with file_content to only show files whose content is still in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		LEFT JOIN bin b ON f.bin_id = b.id
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		ORDER BY f.updated_at DESC LIMIT $1`
	files, err = d.fileQuery(sqlStatement, limit)
	return files, err
}

func (d *FileDao) GetByBytes(limit int) (files []ds.File, err error) {
	// Join with file_content to only show files whose content is still in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		LEFT JOIN bin b ON f.bin_id = b.id
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		ORDER BY fc.bytes DESC LIMIT $1`
	files, err = d.fileQuery(sqlStatement, limit)
	return files, err
}

func (d *FileDao) GetByUpdates(limit int) (files []ds.File, err error) {
	// Join with file_content to only show files whose content is still in storage
	sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		LEFT JOIN bin b ON f.bin_id = b.id
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		ORDER BY f.updates DESC LIMIT $1`
	files, err = d.fileQuery(sqlStatement, limit)
	return files, err
}

// GetRecentUploads returns files uploaded in the last N hours. If mime is
// non-empty, only files matching that MIME type are returned.
func (d *FileDao) GetRecentUploads(mime string, hours int) (files []ds.File, err error) {
	if mime == "" {
		sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
			FROM file f
			JOIN file_content fc ON f.sha256 = fc.sha256
			LEFT JOIN bin b ON f.bin_id = b.id
			WHERE f.created_at > NOW() - $1 * INTERVAL '1 hour' AND f.deleted_at IS NULL
			ORDER BY f.created_at DESC`
		files, err = d.fileQuery(sqlStatement, hours)
	} else {
		sqlStatement := `SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms
			FROM file f
			JOIN file_content fc ON f.sha256 = fc.sha256
			LEFT JOIN bin b ON f.bin_id = b.id
			WHERE f.created_at > NOW() - $1 * INTERVAL '1 hour' AND f.deleted_at IS NULL AND fc.mime = $2
			ORDER BY f.created_at DESC`
		files, err = d.fileQuery(sqlStatement, hours, mime)
	}
	return files, err
}

// GetDistinctMimeTypes returns the set of unique MIME types seen across
// non-deleted files uploaded in the last N hours, sorted alphabetically.
func (d *FileDao) GetDistinctMimeTypes(hours int) (mimeTypes []string, err error) {
	sqlStatement := `SELECT DISTINCT fc.mime
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE f.created_at > NOW() - $1 * INTERVAL '1 hour' AND f.deleted_at IS NULL
		ORDER BY fc.mime ASC`
	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, hours)
	observeQuery(d.metrics, "file_distinct_mime_types", t0, err)
	if err != nil {
		return mimeTypes, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var mime string
		if err = rows.Scan(&mime); err != nil {
			return mimeTypes, err
		}
		mimeTypes = append(mimeTypes, mime)
	}
	if err = rows.Err(); err != nil {
		return mimeTypes, err
	}
	return mimeTypes, nil
}

func (d *FileDao) fileQuery(sqlStatement string, params ...interface{}) (files []ds.File, err error) {
	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, params...)
	observeQuery(d.metrics, "file_query", t0, err)
	if err != nil {
		return files, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.InStorage, &file.IP, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt, &file.BinDeletedAt, &file.BinExpiredAt, &file.UploadDurationMs)
		if err != nil {
			return files, err
		}
		hydrateFile(&file)
		files = append(files, file)
	}
	if err = rows.Err(); err != nil {
		return files, err
	}
	return files, nil
}

func (d *FileDao) FilesByChecksum(limit int) (files []ds.FileByChecksum, err error) {
	// Join with file_content to only count files whose content is still in storage
	sqlStatement := `SELECT f.sha256, COUNT(f.sha256) as c, fc.mime, fc.bytes, COUNT(f.sha256) * fc.bytes AS bytes_total, SUM(f.downloads), SUM(f.updates), fc.blocked
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		GROUP BY f.sha256, fc.mime, fc.bytes, fc.blocked
		ORDER BY c DESC LIMIT $1`

	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, limit)
	observeQuery(d.metrics, "file_by_checksum", t0, err)
	if err != nil {
		return files, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var file ds.FileByChecksum
		err = rows.Scan(&file.SHA256, &file.Count, &file.Mime, &file.Bytes, &file.BytesTotal, &file.DownloadsTotal, &file.UpdatesTotal, &file.Blocked)
		if err != nil {
			return files, err
		}
		file.BytesReadable = humanize.Bytes(file.Bytes)
		file.BytesTotalReadable = humanize.Bytes(file.BytesTotal)
		files = append(files, file)
	}
	if err = rows.Err(); err != nil {
		return files, err
	}
	return files, nil
}

func (d *FileDao) FilesByBytes(limit int) (files []ds.FileByChecksum, err error) {
	// Same as FilesByChecksum but sorted by bytes instead of count
	sqlStatement := `SELECT f.sha256, COUNT(f.sha256) as c, fc.mime, fc.bytes, COUNT(f.sha256) * fc.bytes AS bytes_total, SUM(f.downloads), SUM(f.updates), fc.blocked
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		GROUP BY f.sha256, fc.mime, fc.bytes, fc.blocked
		ORDER BY fc.bytes DESC LIMIT $1`

	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, limit)
	observeQuery(d.metrics, "file_by_bytes", t0, err)
	if err != nil {
		return files, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var file ds.FileByChecksum
		err = rows.Scan(&file.SHA256, &file.Count, &file.Mime, &file.Bytes, &file.BytesTotal, &file.DownloadsTotal, &file.UpdatesTotal, &file.Blocked)
		if err != nil {
			return files, err
		}
		file.BytesReadable = humanize.Bytes(file.Bytes)
		file.BytesTotalReadable = humanize.Bytes(file.BytesTotal)
		files = append(files, file)
	}
	if err = rows.Err(); err != nil {
		return files, err
	}
	return files, nil
}

func (d *FileDao) FilesByBytesTotal(limit int) (files []ds.FileByChecksum, err error) {
	// Same as FilesByChecksum but sorted by total bytes (bytes * count)
	sqlStatement := `SELECT f.sha256, COUNT(f.sha256) as c, fc.mime, fc.bytes, COUNT(f.sha256) * fc.bytes AS bytes_total, SUM(f.downloads), SUM(f.updates), fc.blocked
		FROM file f
		JOIN file_content fc ON f.sha256 = fc.sha256
		WHERE fc.in_storage = true AND f.deleted_at IS NULL
		GROUP BY f.sha256, fc.mime, fc.bytes, fc.blocked
		ORDER BY bytes_total DESC LIMIT $1`

	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, limit)
	observeQuery(d.metrics, "file_by_bytes_total", t0, err)
	if err != nil {
		return files, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var file ds.FileByChecksum
		err = rows.Scan(&file.SHA256, &file.Count, &file.Mime, &file.Bytes, &file.BytesTotal, &file.DownloadsTotal, &file.UpdatesTotal, &file.Blocked)
		if err != nil {
			return files, err
		}
		file.BytesReadable = humanize.Bytes(file.Bytes)
		file.BytesTotalReadable = humanize.Bytes(file.BytesTotal)
		files = append(files, file)
	}
	if err = rows.Err(); err != nil {
		return files, err
	}
	return files, nil
}

func (d *FileDao) FileByChecksum(sha256 string) (files []ds.File, err error) {
	sqlStatement := "SELECT f.id, f.bin_id, f.filename, fc.mime, fc.bytes, fc.md5, f.sha256, f.downloads, f.updates, fc.in_storage, f.ip, f.headers, f.updated_at, f.created_at, f.deleted_at, b.deleted_at, b.expired_at, f.upload_duration_ms FROM file f JOIN file_content fc ON f.sha256 = fc.sha256 LEFT JOIN bin b ON f.bin_id = b.id WHERE f.sha256 = $1 ORDER BY f.created_at DESC"
	files, err = d.fileQuery(sqlStatement, sha256)
	return files, err
}

// CountBySHA256 returns the count of active file references with the given SHA256
// (active = file not deleted AND bin not deleted AND bin not expired)
func (d *FileDao) CountBySHA256(sha256 string) (int, error) {
	var count int
	sqlStatement := `SELECT COUNT(*) FROM file f
JOIN bin b ON f.bin_id = b.id
WHERE f.sha256 = $1 AND f.deleted_at IS NULL AND b.deleted_at IS NULL AND b.expired_at > NOW()`
	t0 := time.Now()
	err := d.db.QueryRow(sqlStatement, sha256).Scan(&count)
	observeQuery(d.metrics, "file_count_by_sha256", t0, err)
	if err != nil {
		return 0, err
	}
	return count, nil
}
