package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"time"
)

type FileDao struct {
	db *sql.DB
}

//func (d *FileDao) validateInput(file *ds.File) error {
//	u, err := url.ParseRequestURI(file.Url)
//	if err != nil {
//		return errors.New("The URL needs to be a valid HTTP/HTTPS URL.")
//	}
//
//	if u.Scheme != "http" && u.Scheme != "https" {
//		return errors.New("The URL has an unknown scheme: " + u.Scheme)
//	}
//
//	u, err = url.ParseRequestURI(file.ProbeUrl)
//	if err != nil {
//		return errors.New("The health probe URL needs to be a valid HTTP/HTTPS URL.")
//	}
//
//	if u.Scheme != "http" && u.Scheme != "https" {
//		return errors.New("The URL has an unknown scheme: " + u.Scheme)
//	}
//
//	if strings.TrimSpace(file.Name) == "" {
//		return errors.New("The name cannot be empty.")
//	}
//
//	// Probe expect should allow valid HTTP response status codes
//	expect_min := 100
//	expect_max := 599
//	if file.ProbeExpect < expect_min || file.ProbeExpect > expect_max {
//		return errors.New(fmt.Sprintf("The probe expect needs to be between %d and %d. Was %d", expect_min, expect_max, file.ProbeExpect))
//	}
//
//	timeout_min := 100
//	timeout_max := 10000
//	if file.ProbeTimeout < timeout_min || file.ProbeTimeout > timeout_max {
//		return errors.New(fmt.Sprintf("The probe timeout needs to be between %d and %d (milliseconds). Was %d.", timeout_min, timeout_max, file.ProbeTimeout))
//	}
//
//	// Probe interval in number of seconds between each health probe.
//	interval_min := 1
//	interval_max := 60
//	if file.ProbeInterval < interval_min || file.ProbeInterval > interval_max {
//		return errors.New(fmt.Sprintf("The probe interval needs to be between %d and %d (seconds). Was %d.", interval_min, interval_max, file.ProbeInterval))
//	}
//
//	// Window length in number of probes to consider.
//	window_min := 1
//	window_max := 20
//	if file.ProbeWindow < window_min || file.ProbeWindow > window_max {
//		return errors.New(fmt.Sprintf("The probe window needs to be between %d and %d. Was %d", window_min, window_max, file.ProbeWindow))
//	}
//
//	// Threshold in number of probes that needs to be health within the window
//	// length for the file to be considered healthy.
//	threshold_min := 1                    // Need to have at least one successfull probe.
//	threshold_max := file.ProbeWindow // The threshold can't be higher than the probe window.
//	if file.ProbeThreshold < threshold_min || file.ProbeThreshold > threshold_max {
//		return errors.New(fmt.Sprintf("The probe threshold needs to be between %d and %d. Was %d.", threshold_min, threshold_max, file.ProbeThreshold))
//	}
//
//	// Initial is the number of health probes to assume healthy at startup
//	// or without enough history.
//	initial_min := 0
//	initial_max := file.ProbeWindow // The threshold can't be higher than the probe window.
//	if file.ProbeInitial < initial_min || file.ProbeInitial > initial_max {
//		return errors.New(fmt.Sprintf("The probe initial needs to be between %d and %d. Was %d.", initial_min, initial_max, file.ProbeInitial))
//	}
//
//	valid_methods := []string{
//		"GET",
//		"HEAD",
//	}
//	if _, found := stringInSlice(file.ProbeMethod, valid_methods); !found {
//		return errors.New("The probe method is not valid")
//	}
//
//	// Input validation passed
//	return nil
//}

func (d *FileDao) GetById(id int) (ds.File, error) {
	var file ds.File
	sqlStatement := "SELECT id, bin_id, filename, size, checksum, updated, created FROM file WHERE id = $1 LIMIT 1"
	err := d.db.QueryRow(sqlStatement, id).Scan(&file.Id, &file.BinId, &file.Filename, &file.Size, &file.Checksum, &file.Updated, &file.Created)
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

	return file, err
}

func (d *FileDao) Upsert(file *ds.File) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT id, bin_id, filename, size, checksum, updated, created FROM file WHERE bin_id = $1 AND filename = $2 LIMIT 1"
	err := d.db.QueryRow(sqlStatement, file.BinId, file.Filename).Scan(&file.Id, &file.BinId, &file.Filename, &file.Size, &file.Checksum, &file.Updated, &file.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			sqlStatement := "INSERT INTO file (bin_id, filename, size, checksum, updated, created) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id"
			err := d.db.QueryRow(sqlStatement, file.BinId, file.Filename, file.Size, file.Checksum, now, now).Scan(&file.Id)
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
	return nil
}

func (d *FileDao) Insert(file *ds.File) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "INSERT INTO file (bin_id, filename, size, checksum, updated, created) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id"
	err := d.db.QueryRow(sqlStatement, file.BinId, file.Filename, file.Size, file.Checksum, now, now).Scan(&file.Id)
	if err != nil {
		return err
	}
	file.Updated = now
	file.Created = now
	file.UpdatedRelative = humanize.Time(file.Updated)
	file.CreatedRelative = humanize.Time(file.Created)
	return nil
}

func (d *FileDao) GetByBinId(id string) ([]ds.File, error) {
	var files []ds.File
	sqlStatement := "SELECT id, bin_id, filename, size, checksum, updated, created FROM file WHERE bin_id = $1 ORDER BY filename ASC"
	rows, err := d.db.Query(sqlStatement, id)
	if err != nil {
		return files, err
	}
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.BinId, &file.Filename, &file.Size, &file.Checksum, &file.Updated, &file.Created)
		if err != nil {
			return files, err
		}

		// https://github.com/lib/pq/issues/329
		file.Updated = file.Updated.UTC()
		file.Created = file.Created.UTC()

		file.UpdatedRelative = humanize.Time(file.Updated)
		file.CreatedRelative = humanize.Time(file.Created)

		files = append(files, file)
	}

	return files, err
}

func (d *FileDao) GetAll() ([]ds.File, error) {
	var files []ds.File
	sqlStatement := "SELECT id, bin_id, filename, size, checksum, updated, created FROM file"
	rows, err := d.db.Query(sqlStatement)
	if err != nil {
		return files, err
	}
	for rows.Next() {
		var file ds.File
		err = rows.Scan(&file.Id, &file.BinId, &file.Filename, &file.Size, &file.Checksum, &file.Updated, &file.Created)
		if err != nil {
			return files, err
		}

		// https://github.com/lib/pq/issues/329
		file.Updated = file.Updated.UTC()
		file.Created = file.Created.UTC()

		file.UpdatedRelative = humanize.Time(file.Updated)
		file.CreatedRelative = humanize.Time(file.Created)

		files = append(files, file)
	}
	return files, err
}

func (d *FileDao) Update(file *ds.File) error {
	var id int
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE file SET filename = $1, size = $2, checksum = $3, updated = $4 WHERE id = $5 RETURNING id"
	err := d.db.QueryRow(sqlStatement, file.Filename, file.Size, file.Checksum, now, file.Id).Scan(&id)
	if err != nil {
		//if err == sql.ErrNoRows {
		//	return errors.New(fmt.Sprintf("Unable to update file id %d", file.Id))
		//}
		return err
	}
	file.Updated = now
	file.UpdatedRelative = humanize.Time(file.Updated)
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
