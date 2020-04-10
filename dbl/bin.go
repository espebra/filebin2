package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

var invalidBin = regexp.MustCompile("[^A-Za-z0-9-_.]")

type BinDao struct {
	db *sql.DB
}

func (d *BinDao) validateInput(bin *ds.Bin) error {
	// Generate the bin if it is not set
	if bin.Id == "" {
		bin.Id = d.GenerateId()
	}

	// Reject invalid bins
	if invalidBin.MatchString(bin.Id) {
		return errors.New("The bin contains invalid characters.")
	}

	// Ensure decent length
	if len(bin.Id) < 8 {
		return errors.New("The bin is too short.")
	}

	// Do not allow the bin to start with .
	if strings.HasPrefix(bin.Id, ".") {
		return errors.New("Invalid bin specified.")
	}
	return nil
}

func (d *BinDao) GenerateId() string {
	// TODO: Make sure the ID is unique
	characters := []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	length := 10

	id := make([]rune, length)
	for i := range id {
		id[i] = characters[rand.Intn(len(characters))]
	}
	return string(id)
}

func (d *BinDao) GetAll() ([]ds.Bin, error) {
	bins := []ds.Bin{}

	sqlStatement := "SELECT bin.id, bin.readonly, bin.deleted, bin.downloads, COALESCE(SUM(file.bytes), 0), bin.updated, bin.created, bin.expiration FROM bin LEFT JOIN file ON bin.id = file.bin_id GROUP BY bin.id"
	rows, err := d.db.Query(sqlStatement)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Deleted, &bin.Downloads, &bin.Bytes, &bin.Updated, &bin.Created, &bin.Expiration)
		if err != nil {
			return bins, err
		}

		// https://github.com/lib/pq/issues/329
		bin.Updated = bin.Updated.UTC()
		bin.Created = bin.Created.UTC()
		bin.Expiration = bin.Expiration.UTC()

		bin.UpdatedRelative = humanize.Time(bin.Updated)
		bin.CreatedRelative = humanize.Time(bin.Created)
		bin.ExpirationRelative = humanize.Time(bin.Expiration)
		bin.BytesReadable = humanize.Bytes(bin.Bytes)

		bins = append(bins, bin)
	}

	return bins, nil
}

func (d *BinDao) GetById(id string) (ds.Bin, error) {
	var bin ds.Bin

	// Get bin info
	sqlStatement := "SELECT bin.id, bin.readonly, bin.deleted, bin.downloads, COALESCE(SUM(file.bytes), 0), bin.updated, bin.created, bin.expiration FROM bin LEFT JOIN file ON bin.id = file.bin_id WHERE bin.id = $1 GROUP BY bin.id LIMIT 1"
	err := d.db.QueryRow(sqlStatement, id).Scan(&bin.Id, &bin.Readonly, &bin.Deleted, &bin.Downloads, &bin.Bytes, &bin.Updated, &bin.Created, &bin.Expiration)
	if err != nil {
		if err == sql.ErrNoRows {
			return bin, errors.New(fmt.Sprintf("No bin found with id %s", id))
		} else {
			fmt.Printf("Unable to query the database: %s\n", err.Error())
		}
	}

	// https://github.com/lib/pq/issues/329
	bin.Updated = bin.Updated.UTC()
	bin.Created = bin.Created.UTC()
	bin.Expiration = bin.Expiration.UTC()

	bin.BytesReadable = humanize.Bytes(bin.Bytes)
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)
	bin.ExpirationRelative = humanize.Time(bin.Expiration)

	return bin, err
}

func (d *BinDao) Upsert(bin *ds.Bin) error {
	if err := d.validateInput(bin); err != nil {
		return err
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.deleted, bin.downloads, COALESCE(SUM(file.bytes), 0), bin.updated, bin.created, bin.expiration FROM bin LEFT JOIN file ON bin.id = file.bin_id WHERE bin.id = $1 GROUP BY bin.id LIMIT 1"
	err := d.db.QueryRow(sqlStatement, bin.Id).Scan(&bin.Id, &bin.Readonly, &bin.Deleted, &bin.Downloads, &bin.Bytes, &bin.Updated, &bin.Created, &bin.Expiration)
	if err != nil {
		if err == sql.ErrNoRows {
			expiration := now.Add(time.Hour * 24 * 7)
			downloads := 0
			deleted := 0
			readonly := false
			sqlStatement := "INSERT INTO bin (id, readonly, deleted, downloads, updated, created, expiration) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id"
			err := d.db.QueryRow(sqlStatement, bin.Id, readonly, deleted, downloads, now, now, expiration).Scan(&bin.Id)
			if err != nil {
				return err
			}
			bin.Updated = now
			bin.Created = now
			bin.Expiration = expiration
			bin.Deleted = deleted
			bin.Readonly = readonly
		} else {
			return err
		}
	}
	// https://github.com/lib/pq/issues/329
	bin.Updated = bin.Updated.UTC()
	bin.Created = bin.Created.UTC()
	bin.Expiration = bin.Expiration.UTC()
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)
	bin.ExpirationRelative = humanize.Time(bin.Expiration)
	bin.BytesReadable = humanize.Bytes(bin.Bytes)
	return nil
}

func (d *BinDao) Insert(bin *ds.Bin) error {
	if err := d.validateInput(bin); err != nil {
		return err
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	expiration := now.Add(time.Hour * 24 * 7)
	downloads := uint64(0)
	readonly := false
	deleted := 0
	sqlStatement := "INSERT INTO bin (id, readonly, deleted, downloads, updated, created, expiration) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id"
	err := d.db.QueryRow(sqlStatement, bin.Id, readonly, deleted, downloads, now, now, expiration).Scan(&bin.Id)
	if err != nil {
		return err
	}
	bin.Updated = now
	bin.Created = now
	bin.Expiration = expiration
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)
	bin.ExpirationRelative = humanize.Time(bin.Expiration)
	bin.Downloads = downloads
	bin.Readonly = readonly
	bin.Deleted = deleted
	return nil
}

func (d *BinDao) Update(bin *ds.Bin) error {
	var id string
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET readonly = $1, deleted = $2, updated = $3 WHERE id = $4 RETURNING id"
	err := d.db.QueryRow(sqlStatement, bin.Readonly, bin.Deleted, now, bin.Id).Scan(&id)
	if err != nil {
		return err
	}
	bin.Updated = now
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	return nil
}

func (d *BinDao) Delete(bin *ds.Bin) error {
	sqlStatement := "DELETE FROM bin WHERE id = $1"
	res, err := d.db.Exec(sqlStatement, bin.Id)
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

func (d *BinDao) RegisterDownload(bin *ds.Bin) error {
	sqlStatement := "UPDATE bin SET downloads = downloads + 1 WHERE id = $1 RETURNING downloads"
	err := d.db.QueryRow(sqlStatement, bin.Id).Scan(&bin.Downloads)
	if err != nil {
		return err
	}
	return nil
}
