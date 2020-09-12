package dbl

import (
	"database/sql"
	"errors"
	//"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"math/rand"
	"path"
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

func (d *BinDao) GetAll(hidden bool) (bins []ds.Bin, err error) {
	sqlStatement := "SELECT bin.id, bin.readonly, bin.hidden, bin.deleted, bin.downloads, COALESCE(SUM(file.bytes), 0), COUNT(file.filename), bin.updated_at, bin.created_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.hidden=$1 WHERE bin.hidden=$1 AND bin.deleted=false GROUP BY bin.id ORDER BY bin.updated_at DESC"
	rows, err := d.db.Query(sqlStatement, hidden)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Hidden, &bin.Deleted, &bin.Downloads, &bin.Bytes, &bin.Files, &bin.UpdatedAt, &bin.CreatedAt, &bin.ExpiredAt, &bin.DeletedAt)
		if err != nil {
			return bins, err
		}
		// https://github.com/lib/pq/issues/329
		bin.UpdatedAt = bin.UpdatedAt.UTC()
		bin.CreatedAt = bin.CreatedAt.UTC()
		bin.ExpiredAt = bin.ExpiredAt.UTC()
		bin.DeletedAt = bin.DeletedAt.UTC()
		bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
		bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
		bin.DeletedAtRelative = humanize.Time(bin.DeletedAt)
		bin.BytesReadable = humanize.Bytes(bin.Bytes)
		bin.URL = path.Join(bin.Id)
		bins = append(bins, bin)
	}
	return bins, nil
}

func (d *BinDao) GetPendingDelete() (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.hidden, bin.deleted, bin.downloads, COALESCE(SUM(file.bytes), 0), COUNT(filename) AS files, bin.updated_at, bin.created_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id = file.bin_id WHERE (bin.expired_at <= $1 OR bin.hidden = true) AND bin.deleted = false GROUP BY bin.id"
	rows, err := d.db.Query(sqlStatement, now)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Hidden, &bin.Deleted, &bin.Downloads, &bin.Bytes, &bin.Files, &bin.UpdatedAt, &bin.CreatedAt, &bin.ExpiredAt, &bin.DeletedAt)
		if err != nil {
			return bins, err
		}
		// https://github.com/lib/pq/issues/329
		bin.UpdatedAt = bin.UpdatedAt.UTC()
		bin.CreatedAt = bin.CreatedAt.UTC()
		bin.ExpiredAt = bin.ExpiredAt.UTC()
		bin.DeletedAt = bin.DeletedAt.UTC()
		bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
		bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
		bin.DeletedAtRelative = humanize.Time(bin.DeletedAt)
		bin.BytesReadable = humanize.Bytes(bin.Bytes)
		bin.URL = path.Join(bin.Id)
		bins = append(bins, bin)
	}
	return bins, nil
}

func (d *BinDao) GetById(id string) (bin ds.Bin, found bool, err error) {
	// Get bin info
	sqlStatement := "SELECT bin.id, bin.readonly, bin.hidden, bin.deleted, bin.downloads, COALESCE(SUM(file.bytes), 0), COUNT(file.filename), bin.updated_at, bin.created_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id = file.bin_id AND file.hidden=false WHERE bin.id = $1 GROUP BY bin.id LIMIT 1"
	err = d.db.QueryRow(sqlStatement, id).Scan(&bin.Id, &bin.Readonly, &bin.Hidden, &bin.Deleted, &bin.Downloads, &bin.Bytes, &bin.Files, &bin.UpdatedAt, &bin.CreatedAt, &bin.ExpiredAt, &bin.DeletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return bin, false, nil
		} else {
			return bin, false, err
		}
	}
	// https://github.com/lib/pq/issues/329
	bin.UpdatedAt = bin.UpdatedAt.UTC()
	bin.CreatedAt = bin.CreatedAt.UTC()
	bin.ExpiredAt = bin.ExpiredAt.UTC()
	bin.DeletedAt = bin.DeletedAt.UTC()
	bin.BytesReadable = humanize.Bytes(bin.Bytes)
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
	bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
	bin.DeletedAtRelative = humanize.Time(bin.DeletedAt)
	bin.URL = path.Join(bin.Id)
	return bin, true, nil
}

func (d *BinDao) Insert(bin *ds.Bin) (err error) {
	if err := d.ValidateInput(bin); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	downloads := uint64(0)
	readonly := false
	hidden := false
	deleted := false
	bin.ExpiredAt = bin.ExpiredAt.UTC().Truncate(time.Microsecond)
	sqlStatement := "INSERT INTO bin (id, readonly, hidden, deleted, downloads, updated_at, created_at, expired_at, deleted_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id"
	if err := d.db.QueryRow(sqlStatement, bin.Id, readonly, hidden, deleted, downloads, now, now, bin.ExpiredAt, bin.DeletedAt).Scan(&bin.Id); err != nil {
		return err
	}
	bin.UpdatedAt = now
	bin.CreatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
	bin.DeletedAtRelative = humanize.Time(bin.DeletedAt)
	bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
	bin.Downloads = downloads
	bin.Readonly = readonly
	bin.Hidden = hidden
	return nil
}

func (d *BinDao) Update(bin *ds.Bin) (err error) {
	var id string
	now := time.Now().UTC().Truncate(time.Microsecond)
	bin.ExpiredAt = bin.ExpiredAt.UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET readonly = $1, hidden = $2, deleted = $3, updated_at = $4, expired_at = $5, deleted_at = $6 WHERE id = $7 RETURNING id"
	err = d.db.QueryRow(sqlStatement, bin.Readonly, bin.Hidden, bin.Deleted, now, bin.ExpiredAt, bin.DeletedAt, bin.Id).Scan(&id)
	if err != nil {
		return err
	}
	bin.UpdatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	bin.DeletedAtRelative = humanize.Time(bin.DeletedAt)
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

func (d *BinDao) HideRecentlyExpiredBins() (count int64, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET hidden = true WHERE hidden = false AND expired_at <= $1"
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

func (d *BinDao) HideEmptyBins() (count int64, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	limit := now.Add(-5 * time.Minute)

	// Hide empty bins that are older than limit
	sqlStatement := "UPDATE bin SET hidden = true WHERE bin.id IN (SELECT bin.id FROM bin LEFT JOIN file ON bin.id = file.bin_id WHERE bin.hidden = false GROUP BY bin.id HAVING COUNT(filename) = 0 AND bin.created_at < $1)"
	res, err := d.db.Exec(sqlStatement, limit)
	if err != nil {
		return 0, err
	}
	count, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}
