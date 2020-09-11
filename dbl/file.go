package dbl

import (
	"database/sql"
	"errors"
	//"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"path"
	"regexp"
	"strings"
	"time"
)

var invalidFilename = regexp.MustCompile("[^A-Za-z0-9-_=+,.]")

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
	// Replace all invalid characters with _
	file.Filename = invalidFilename.ReplaceAllString(file.Filename, "_")

	// . is not allowed as the first character
	if strings.HasPrefix(file.Filename, ".") {
		file.Filename = strings.Replace(file.Filename, ".", "_", 1)
	}

	// If the filename is empty, set it to _
	if len(file.Filename) == 0 {
		return errors.New("Filename not specified")
	}

	return nil
}

func (d *FileDao) GetById(id int) (file ds.File, found bool, err error) {
	sqlStatement := "SELECT id, bin_id, filename, status, mime, bytes, md5, sha256, downloads, updates, ip, trace, updated_at, created_at, deleted_at FROM file WHERE id = $1 LIMIT 1"
	err = d.db.QueryRow(sqlStatement, id).Scan(&file.Id, &file.Bin, &file.Filename, &file.Status, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.IP, &file.Trace, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, false, nil
		} else {
			return file, false, err
		}
	}
	// https://github.com/lib/pq/issues/329
	file.UpdatedAt = file.UpdatedAt.UTC()
	file.CreatedAt = file.CreatedAt.UTC()
	file.DeletedAt = file.DeletedAt.UTC()
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.CreatedAtRelative = humanize.Time(file.CreatedAt)
	file.DeletedAtRelative = humanize.Time(file.DeletedAt)
	file.BytesReadable = humanize.Bytes(file.Bytes)
	return file, true, nil
}

func (d *FileDao) GetByName(bin string, filename string) (file ds.File, found bool, err error) {
	sqlStatement := "SELECT id, bin_id, filename, status, mime, bytes, md5, sha256, downloads, updates, ip, trace, updated_at, created_at, deleted_at FROM file WHERE bin_id = $1 AND filename = $2 LIMIT 1"
	err = d.db.QueryRow(sqlStatement, bin, filename).Scan(&file.Id, &file.Bin, &file.Filename, &file.Status, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.IP, &file.Trace, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, false, nil
		} else {
			return file, false, err
		}
	}
	// https://github.com/lib/pq/issues/329
	file.UpdatedAt = file.UpdatedAt.UTC()
	file.CreatedAt = file.CreatedAt.UTC()
	file.DeletedAt = file.DeletedAt.UTC()
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.CreatedAtRelative = humanize.Time(file.CreatedAt)
	file.DeletedAtRelative = humanize.Time(file.DeletedAt)
	file.BytesReadable = humanize.Bytes(file.Bytes)
	setCategory(&file)
	return file, true, nil
}

func (d *FileDao) Insert(file *ds.File) (err error) {
	if err := d.ValidateInput(file); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	status := 0
	downloads := 0
	updates := 0

	// Some kind of default value, not NULL
	if file.IP == "" {
		file.IP = "N/A"
	}
	if file.Trace == "" {
		file.Trace = "N/A"
	}

	sqlStatement := "INSERT INTO file (bin_id, filename, status, mime, bytes, md5, sha256, downloads, updates, ip, trace, nonce, updated_at, created_at, deleted_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id"
	err = d.db.QueryRow(sqlStatement, file.Bin, file.Filename, file.Status, file.Mime, file.Bytes, file.MD5, file.SHA256, downloads, updates, file.IP, file.Trace, file.Nonce, now, now, file.DeletedAt).Scan(&file.Id)
	if err != nil {
		return err
	}
	file.Status = status
	file.Downloads = uint64(downloads)
	file.Updates = uint64(updates)
	file.UpdatedAt = now
	file.CreatedAt = now
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.CreatedAtRelative = humanize.Time(file.CreatedAt)
	file.DeletedAtRelative = humanize.Time(file.DeletedAt)
	file.BytesReadable = humanize.Bytes(file.Bytes)
	setCategory(file)
	return nil
}

func (d *FileDao) GetByBin(id string, status int) (files []ds.File, err error) {
	sqlStatement := "SELECT id, bin_id, filename, status, mime, bytes, md5, sha256, downloads, updates, ip, trace, nonce, updated_at, created_at, deleted_at FROM file WHERE bin_id = $1 AND status = $2 ORDER BY filename ASC"
	rows, err := d.db.Query(sqlStatement, id, status)
	if err != nil {
		return files, err
	}
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.Bin, &file.Filename, &file.Status, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.IP, &file.Trace, &file.Nonce, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
		if err != nil {
			return files, err
		}
		// https://github.com/lib/pq/issues/329
		file.UpdatedAt = file.UpdatedAt.UTC()
		file.CreatedAt = file.CreatedAt.UTC()
		file.DeletedAt = file.DeletedAt.UTC()
		file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
		file.CreatedAtRelative = humanize.Time(file.CreatedAt)
		file.DeletedAtRelative = humanize.Time(file.DeletedAt)
		file.BytesReadable = humanize.Bytes(file.Bytes)
		file.URL = path.Join(file.Bin, file.Filename)
		setCategory(&file)
		files = append(files, file)
	}
	return files, nil
}

func (d *FileDao) GetAll(status int) (files []ds.File, err error) {
	sqlStatement := "SELECT id, bin_id, filename, status, mime, bytes, md5, sha256, downloads, updates, ip, trace, nonce, updated_at, created_at, deleted_at FROM file WHERE status = $1 ORDER BY filename ASC"
	rows, err := d.db.Query(sqlStatement, status)
	if err != nil {
		return files, err
	}
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.Bin, &file.Filename, &file.Status, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.IP, &file.Trace, &file.Nonce, &file.UpdatedAt, &file.CreatedAt, &file.DeletedAt)
		if err != nil {
			return files, err
		}
		// https://github.com/lib/pq/issues/329
		file.UpdatedAt = file.UpdatedAt.UTC()
		file.CreatedAt = file.CreatedAt.UTC()
		file.DeletedAt = file.DeletedAt.UTC()
		file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
		file.CreatedAtRelative = humanize.Time(file.CreatedAt)
		file.DeletedAtRelative = humanize.Time(file.DeletedAt)
		file.BytesReadable = humanize.Bytes(file.Bytes)
		setCategory(&file)
		files = append(files, file)
	}
	return files, nil
}

func (d *FileDao) Update(file *ds.File) (err error) {
	var id int
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE file SET filename = $1, status = $2, mime = $3, bytes = $4, md5 = $5, sha256 = $6, nonce = $7, updates = $8, updated_at = $9, deleted_at = $10, ip = $11, trace = $12 WHERE id = $13 RETURNING id"
	err = d.db.QueryRow(sqlStatement, file.Filename, file.Status, file.Mime, file.Bytes, file.MD5, file.SHA256, file.Nonce, file.Updates, now, file.DeletedAt, file.IP, file.Trace, file.Id).Scan(&id)
	if err != nil {
		return err
	}
	file.UpdatedAt = now
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.DeletedAtRelative = humanize.Time(file.DeletedAt)
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
	} else {
		return nil
	}
}

func (d *FileDao) RegisterDownload(file *ds.File) (err error) {
	sqlStatement := "UPDATE file SET downloads = downloads + 1 WHERE id = $1 RETURNING downloads"
	err = d.db.QueryRow(sqlStatement, file.Id).Scan(&file.Downloads)
	if err != nil {
		return err
	}
	return nil
}
