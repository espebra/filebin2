package dbl

import (
	"database/sql"
	"errors"
	//"fmt"
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

func (d *BinDao) ValidateInput(bin *ds.Bin) error {
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

func (d *BinDao) GetAll() (bins []ds.Bin, err error) {
	sqlStatement := "SELECT bin.id, bin.readonly, bin.status, bin.downloads, COALESCE(SUM(file.bytes), 0), bin.updated, bin.created, bin.expiration, bin.deleted FROM bin LEFT JOIN file ON bin.id = file.bin_id GROUP BY bin.id"
	rows, err := d.db.Query(sqlStatement)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Status, &bin.Downloads, &bin.Bytes, &bin.Updated, &bin.Created, &bin.Expiration, &bin.Deleted)
		if err != nil {
			return bins, err
		}
		// https://github.com/lib/pq/issues/329
		bin.Updated = bin.Updated.UTC()
		bin.Created = bin.Created.UTC()
		bin.Expiration = bin.Expiration.UTC()
		bin.Deleted = bin.Deleted.UTC()
		bin.UpdatedRelative = humanize.Time(bin.Updated)
		bin.CreatedRelative = humanize.Time(bin.Created)
		bin.ExpirationRelative = humanize.Time(bin.Expiration)
		bin.DeletedRelative = humanize.Time(bin.Deleted)
		bin.BytesReadable = humanize.Bytes(bin.Bytes)
		bins = append(bins, bin)
	}
	return bins, nil
}

func (d *BinDao) GetBinsPendingExpiration() (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.status, bin.downloads, COALESCE(SUM(file.bytes), 0), bin.updated, bin.created, bin.expiration, bin.deleted FROM bin LEFT JOIN file ON bin.id = file.bin_id WHERE bin.expiration <= $1 AND bin.status < 2 GROUP BY bin.id"
	rows, err := d.db.Query(sqlStatement, now)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Status, &bin.Downloads, &bin.Bytes, &bin.Updated, &bin.Created, &bin.Expiration, &bin.Deleted)
		if err != nil {
			return bins, err
		}
		// https://github.com/lib/pq/issues/329
		bin.Updated = bin.Updated.UTC()
		bin.Created = bin.Created.UTC()
		bin.Expiration = bin.Expiration.UTC()
		bin.Deleted = bin.Deleted.UTC()
		bin.UpdatedRelative = humanize.Time(bin.Updated)
		bin.CreatedRelative = humanize.Time(bin.Created)
		bin.ExpirationRelative = humanize.Time(bin.Expiration)
		bin.DeletedRelative = humanize.Time(bin.Deleted)
		bin.BytesReadable = humanize.Bytes(bin.Bytes)
		bins = append(bins, bin)
	}
	return bins, nil
}

func (d *BinDao) GetById(id string) (bin ds.Bin, found bool, err error) {
	// Get bin info
	sqlStatement := "SELECT bin.id, bin.readonly, bin.status, bin.downloads, COALESCE(SUM(file.bytes), 0), bin.updated, bin.created, bin.expiration, bin.deleted FROM bin LEFT JOIN file ON bin.id = file.bin_id WHERE bin.id = $1 GROUP BY bin.id LIMIT 1"
	err = d.db.QueryRow(sqlStatement, id).Scan(&bin.Id, &bin.Readonly, &bin.Status, &bin.Downloads, &bin.Bytes, &bin.Updated, &bin.Created, &bin.Expiration, &bin.Deleted)
	if err != nil {
		if err == sql.ErrNoRows {
			return bin, false, nil
		} else {
			return bin, false, err
		}
	}
	// https://github.com/lib/pq/issues/329
	bin.Updated = bin.Updated.UTC()
	bin.Created = bin.Created.UTC()
	bin.Expiration = bin.Expiration.UTC()
	bin.Deleted = bin.Deleted.UTC()
	bin.BytesReadable = humanize.Bytes(bin.Bytes)
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)
	bin.ExpirationRelative = humanize.Time(bin.Expiration)
	bin.DeletedRelative = humanize.Time(bin.Deleted)
	return bin, true, nil
}

func (d *BinDao) Insert(bin *ds.Bin) (err error) {
	if err := d.ValidateInput(bin); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	downloads := uint64(0)
	readonly := false
	status := 0
	bin.Expiration = bin.Expiration.UTC().Truncate(time.Microsecond)
	sqlStatement := "INSERT INTO bin (id, readonly, status, downloads, updated, created, expiration, deleted) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id"
	if err := d.db.QueryRow(sqlStatement, bin.Id, readonly, status, downloads, now, now, bin.Expiration, bin.Deleted).Scan(&bin.Id); err != nil {
		return err
	}
	bin.Updated = now
	bin.Created = now
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)
	bin.DeletedRelative = humanize.Time(bin.Deleted)
	bin.ExpirationRelative = humanize.Time(bin.Expiration)
	bin.Downloads = downloads
	bin.Readonly = readonly
	bin.Status = status
	return nil
}

func (d *BinDao) Update(bin *ds.Bin) (err error) {
	var id string
	now := time.Now().UTC().Truncate(time.Microsecond)
	bin.Expiration = bin.Expiration.UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET readonly = $1, status = $2, updated = $3, expiration = $4, deleted = $5 WHERE id = $6 RETURNING id"
	err = d.db.QueryRow(sqlStatement, bin.Readonly, bin.Status, now, bin.Expiration, bin.Deleted, bin.Id).Scan(&id)
	if err != nil {
		return err
	}
	bin.Updated = now
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.DeletedRelative = humanize.Time(bin.Deleted)
	return nil
}

func (d *BinDao) Delete(bin *ds.Bin) (err error) {
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

func (d *BinDao) RegisterDownload(bin *ds.Bin) (err error) {
	sqlStatement := "UPDATE bin SET downloads = downloads + 1 WHERE id = $1 RETURNING downloads"
	err = d.db.QueryRow(sqlStatement, bin.Id).Scan(&bin.Downloads)
	if err != nil {
		return err
	}
	return nil
}

func (d *BinDao) FlagRecentlyExpiredBins() (count int64, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET status = 1 WHERE status = 0 AND expiration <= $1"
	res, err := d.db.Exec(sqlStatement, now)
	if err != nil {
		return 0, err
	}
	count, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}
