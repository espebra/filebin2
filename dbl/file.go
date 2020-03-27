package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"path"
	"time"
)

type FileDao struct {
	db *sql.DB
}

func (d *FileDao) GetById(id int) (ds.File, error) {
	var file ds.File
	sqlStatement := "SELECT id, bin_id, filename, mime, bytes, md5, sha256, downloads, updated, created FROM file WHERE id = $1 LIMIT 1"
	err := d.db.QueryRow(sqlStatement, id).Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updated, &file.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, errors.New(fmt.Sprintf("No file found with id %d", id))
		}
	}

	// https://github.com/lib/pq/issues/329
	file.Updated = file.Updated.UTC()
	file.Created = file.Created.UTC()

	file.UpdatedRelative = humanize.Time(file.Updated)
	file.CreatedRelative = humanize.Time(file.Created)
	file.BytesReadable = humanize.Bytes(file.Bytes)

	return file, err
}

func (d *FileDao) GetByName(bin string, filename string) (ds.File, error) {
	var file ds.File
	sqlStatement := "SELECT id, bin_id, filename, mime, bytes, md5, sha256, downloads, updated, created FROM file WHERE bin_id = $1 AND filename = $2 LIMIT 1"
	err := d.db.QueryRow(sqlStatement, bin, filename).Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updated, &file.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return file, errors.New(fmt.Sprintf("No file found with filename %s in bin %s", filename, bin))
		}
	}

	// https://github.com/lib/pq/issues/329
	file.Updated = file.Updated.UTC()
	file.Created = file.Created.UTC()

	file.UpdatedRelative = humanize.Time(file.Updated)
	file.CreatedRelative = humanize.Time(file.Created)
	file.BytesReadable = humanize.Bytes(file.Bytes)

	return file, err
}

func (d *FileDao) Upsert(file *ds.File) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT id, bin_id, filename, mime, bytes, md5, sha256, downloads, updates, nonce, updated, created FROM file WHERE bin_id = $1 AND filename = $2 LIMIT 1"
	err := d.db.QueryRow(sqlStatement, file.Bin, file.Filename).Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.Nonce, &file.Updated, &file.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			downloads := 0
			updates := 0
			sqlStatement := "INSERT INTO file (bin_id, filename, mime, bytes, md5, sha256, downloads, updates, nonce, updated, created) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id"
			err := d.db.QueryRow(sqlStatement, file.Bin, file.Filename, file.Mime, file.Bytes, file.MD5, file.SHA256, downloads, updates, file.Nonce, now, now).Scan(&file.Id)
			if err != nil {
				return err
			}
			file.Updated = now
			file.Created = now
		} else {
			return err
		}
	}

	// https://github.com/lib/pq/issues/329
	file.Updated = file.Updated.UTC()
	file.Created = file.Created.UTC()

	file.UpdatedRelative = humanize.Time(file.Updated)
	file.CreatedRelative = humanize.Time(file.Created)
	file.BytesReadable = humanize.Bytes(file.Bytes)
	return nil
}

func (d *FileDao) Insert(file *ds.File) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	downloads := 0
	sqlStatement := "INSERT INTO file (bin_id, filename, mime, bytes, md5, sha256, downloads, nonce, updated, created) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id"
	err := d.db.QueryRow(sqlStatement, file.Bin, file.Filename, file.Mime, file.Bytes, file.MD5, file.SHA256, downloads, file.Nonce, now, now).Scan(&file.Id)
	if err != nil {
		return err
	}
	file.Updated = now
	file.Created = now
	file.UpdatedRelative = humanize.Time(file.Updated)
	file.CreatedRelative = humanize.Time(file.Created)
	file.BytesReadable = humanize.Bytes(file.Bytes)
	return nil
}

func (d *FileDao) GetByBin(id string) ([]ds.File, error) {
	var files []ds.File
	sqlStatement := "SELECT id, bin_id, filename, mime, bytes, md5, sha256, downloads, updates, nonce, updated, created FROM file WHERE bin_id = $1 ORDER BY filename ASC"
	rows, err := d.db.Query(sqlStatement, id)
	if err != nil {
		return files, err
	}
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.Nonce, &file.Updated, &file.Created)
		if err != nil {
			return files, err
		}

		// https://github.com/lib/pq/issues/329
		file.Updated = file.Updated.UTC()
		file.Created = file.Created.UTC()

		file.UpdatedRelative = humanize.Time(file.Updated)
		file.CreatedRelative = humanize.Time(file.Created)
		file.BytesReadable = humanize.Bytes(file.Bytes)

		file.URL = path.Join(file.Bin, file.Filename)

		files = append(files, file)
	}

	return files, err
}

func (d *FileDao) GetAll() ([]ds.File, error) {
	var files []ds.File
	sqlStatement := "SELECT id, bin_id, filename, mime, bytes, md5, sha256, downloads, updates, nonce, updated, created FROM file"
	rows, err := d.db.Query(sqlStatement)
	if err != nil {
		return files, err
	}
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.Bin, &file.Filename, &file.Mime, &file.Bytes, &file.MD5, &file.SHA256, &file.Downloads, &file.Updates, &file.Nonce, &file.Updated, &file.Created)
		if err != nil {
			return files, err
		}

		// https://github.com/lib/pq/issues/329
		file.Updated = file.Updated.UTC()
		file.Created = file.Created.UTC()

		file.UpdatedRelative = humanize.Time(file.Updated)
		file.CreatedRelative = humanize.Time(file.Created)
		file.BytesReadable = humanize.Bytes(file.Bytes)

		files = append(files, file)
	}
	return files, err
}

func (d *FileDao) Update(file *ds.File) error {
	var id int
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE file SET filename = $1, mime = $2, bytes = $3, md5 = $4, sha256 = $5, nonce = $6, updated = $7, updates = updates + 1 WHERE id = $8 RETURNING id"
	err := d.db.QueryRow(sqlStatement, file.Filename, file.Mime, file.Bytes, file.MD5, file.SHA256, file.Nonce, now, file.Id).Scan(&id)
	if err != nil {
		//if err == sql.ErrNoRows {
		//	return errors.New(fmt.Sprintf("Unable to update file id %d", file.Id))
		//}
		return err
	}
	file.Updated = now
	file.UpdatedRelative = humanize.Time(file.Updated)
	file.BytesReadable = humanize.Bytes(file.Bytes)
	return nil
}

func (d *FileDao) Delete(file *ds.File) error {
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

func (d *FileDao) RegisterDownload(file *ds.File) error {
	sqlStatement := "UPDATE file SET downloads = downloads + 1 WHERE id = $1 RETURNING downloads"
	err := d.db.QueryRow(sqlStatement, file.Id).Scan(&file.Downloads)
	if err != nil {
		return err
	}
	return nil
}
