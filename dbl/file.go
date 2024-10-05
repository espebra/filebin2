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
	//"regexp"
	"unicode"
)

//var validCharacters = regexp.MustCompile("-_=+,.")

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
	// Trim whitespace before and after.
	file.Filename = strings.Trim(file.Filename, " ")

	// If the filename is empty, error out
	if len(file.Filename) == 0 {
		return errors.New("Filename not specified")
	}

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
		case strings.ContainsAny(string(r), "-_=+,."):
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

	// Truncate long filenames
	// XXX: The maximum length could be made configurable
	if len(n) > 120 {
		fmt.Printf("Truncating filename to 120 characters. Counting %d characters in %q.\n", len(n), n)
		n = n[:120]
	}

	if file.Filename != n {
		fmt.Printf("Modifying filename during upload from %q to %q\n", file.Filename, n)
	}

	file.Filename = n

	return nil
}

func (d *FileDao) GetByID(id int) (file ds.File, found bool, err error) {
	sqlStatement := "SELECT id, bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at FROM file WHERE id = $1 LIMIT 1"
	err = d.db.QueryRow(sqlStatement, id).Scan(&file.Id, &file.Bin, &file.Filename, &file.InStorage, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.IP, &file.ClientId, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
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
	sqlStatement := "SELECT id, bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at FROM file WHERE bin_id = $1 AND filename = $2 LIMIT 1"
	err = d.db.QueryRow(sqlStatement, bin, filename).Scan(&file.Id, &file.Bin, &file.Filename, &file.InStorage, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.IP, &file.ClientId, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
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

func (d *FileDao) Insert(file *ds.File) (err error) {
	if err := d.ValidateInput(file); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	inStorage := false
	downloads := 0
	updates := 0

	// Some kind of default value, not NULL
	if file.IP == "" {
		file.IP = "N/A"
	}
	if file.Headers == "" {
		file.Headers = "N/A"
	}

	sqlStatement := "INSERT INTO file (bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id"
	err = d.db.QueryRow(sqlStatement, file.Bin, file.Filename, file.InStorage, file.Mime, file.Bytes, file.MD5, file.SHA256, downloads, updates, file.IP, file.ClientId, file.Headers, now, now, file.DeletedAt).Scan(&file.Id)
	if err != nil {
		return err
	}
	file.InStorage = inStorage
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
	sqlStatement := "UPDATE file SET filename = $1, in_storage = $2, mime = $3, bytes = $4, md5 = $5, sha256 = $6, updates = $7, updated_at = $8, deleted_at = $9, ip = $10, headers = $11, client_id = $12 WHERE id = $13 RETURNING id"
	err = d.db.QueryRow(sqlStatement, file.Filename, file.InStorage, file.Mime, file.Bytes, file.MD5, file.SHA256, file.Updates, now, file.DeletedAt, file.IP, file.Headers, file.ClientId, file.Id).Scan(&id)
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
	sqlStatement := "SELECT id, bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at FROM file WHERE bin_id = $1 AND in_storage = $2 AND deleted_at IS NULL ORDER BY filename ASC"
	files, err = d.fileQuery(sqlStatement, id, inStorage)
	return files, err
}

func (d *FileDao) GetAll(available bool) (files []ds.File, err error) {
	sqlStatement := "SELECT id, bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at FROM file WHERE in_storage = $1 AND deleted_at IS NULL ORDER BY filename ASC"
	files, err = d.fileQuery(sqlStatement, available)
	return files, err
}

func (d *FileDao) GetPendingDelete() (files []ds.File, err error) {
	sqlStatement := "SELECT id, bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at FROM file WHERE in_storage = true AND deleted_at IS NOT NULL ORDER BY filename ASC"
	files, err = d.fileQuery(sqlStatement)
	return files, err
}

func (d *FileDao) GetTopDownloads(limit int) (files []ds.File, err error) {
	sqlStatement := "SELECT id, bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at FROM file WHERE in_storage = true AND deleted_at IS NULL ORDER BY downloads DESC LIMIT $1"
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
		err = rows.Scan(&file.Id, &file.Bin, &file.Filename, &file.InStorage, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.IP, &file.ClientId, &file.Headers, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
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
	sqlStatement := "SELECT sha256, COUNT(sha256) as c, mime, bytes, COUNT(sha256) * bytes AS bytes_total, SUM(downloads), SUM(updates) FROM file WHERE in_storage = true AND deleted_at IS NULL GROUP BY sha256, mime, bytes ORDER BY c DESC LIMIT $1"

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
	sqlStatement := "SELECT id, bin_id, filename, in_storage, mime, bytes, md5, sha256, downloads, updates, ip, client_id, headers, updated_at, created_at, deleted_at FROM file WHERE sha256 = $1 ORDER BY in_storage DESC NULLS LAST, downloads DESC, updates DESC"
	files, err = d.fileQuery(sqlStatement, sha256)
	return files, err
}
