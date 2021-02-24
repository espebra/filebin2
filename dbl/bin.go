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
	if len(bin.Id) > 60 {
		return errors.New("The bin is too long.")
	}
	// Do not allow the bin to start with .
	if strings.HasPrefix(bin.Id, ".") {
		return errors.New("Invalid bin specified.")
	}
	if bin.UpdatedAt.After(bin.ExpiredAt) {
		return errors.New("The bin cannot be updated when it has expired.")
	}
	return nil
}

func (d *BinDao) GenerateId() string {
	// TODO: Make sure the ID is unique
	characters := []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	length := 16

	id := make([]rune, length)
	for i := range id {
		id[i] = characters[rand.Intn(len(characters))]
	}
	return string(id)
}

func (d *BinDao) GetAll() (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.bytes), 0), COUNT(file.filename), bin.updates, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id=file.bin_id AND file.deleted_at IS NULL AND file.in_storage = true WHERE bin.expired_at > $1 AND bin.deleted_at IS NULL GROUP BY bin.id ORDER BY bin.updated_at DESC"
	rows, err := d.db.Query(sqlStatement, now)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Downloads, &bin.Bytes, &bin.Files, &bin.Updates, &bin.UpdatedAt, &bin.CreatedAt, &bin.ApprovedAt, &bin.ExpiredAt, &bin.DeletedAt)
		if err != nil {
			return bins, err
		}
		// https://github.com/lib/pq/issues/329
		bin.UpdatedAt = bin.UpdatedAt.UTC()
		bin.CreatedAt = bin.CreatedAt.UTC()
		bin.ExpiredAt = bin.ExpiredAt.UTC()
		bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
		bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
		if bin.IsApproved() {
			bin.ApprovedAt.Time = bin.ApprovedAt.Time.UTC()
			bin.ApprovedAtRelative = humanize.Time(bin.ApprovedAt.Time)
		}
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
		if bin.IsDeleted() {
			bin.DeletedAt.Time = bin.DeletedAt.Time.UTC()
			bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
		}
		bin.BytesReadable = humanize.Bytes(bin.Bytes)
		bin.URL = path.Join(bin.Id)
		bins = append(bins, bin)
	}
	return bins, nil
}

func (d *BinDao) GetPendingDelete() (bins []ds.Bin, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.bytes), 0), COUNT(filename) AS files, bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin INNER JOIN file ON bin.id = file.bin_id AND file.in_storage=true WHERE bin.expired_at < $1 OR bin.deleted_at IS NOT NULL GROUP BY bin.id"
	rows, err := d.db.Query(sqlStatement, now)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Readonly, &bin.Downloads, &bin.Bytes, &bin.Files, &bin.UpdatedAt, &bin.CreatedAt, &bin.ApprovedAt, &bin.ExpiredAt, &bin.DeletedAt)
		if err != nil {
			return bins, err
		}
		// https://github.com/lib/pq/issues/329
		bin.UpdatedAt = bin.UpdatedAt.UTC()
		bin.CreatedAt = bin.CreatedAt.UTC()
		bin.ExpiredAt = bin.ExpiredAt.UTC()
		bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
		bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
		if bin.IsApproved() {
			bin.ApprovedAt.Time = bin.ApprovedAt.Time.UTC()
			bin.ApprovedAtRelative = humanize.Time(bin.ApprovedAt.Time)
		}
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
		if bin.IsDeleted() {
			bin.DeletedAt.Time = bin.DeletedAt.Time.UTC()
			bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
		}
		bin.BytesReadable = humanize.Bytes(bin.Bytes)
		bin.URL = path.Join(bin.Id)
		bins = append(bins, bin)
	}
	return bins, nil
}

func (d *BinDao) GetById(id string) (bin ds.Bin, found bool, err error) {
	// Get bin info
	sqlStatement := "SELECT bin.id, bin.readonly, bin.downloads, COALESCE(SUM(file.bytes), 0), COUNT(file.filename), bin.updated_at, bin.created_at, bin.approved_at, bin.expired_at, bin.deleted_at FROM bin LEFT JOIN file ON bin.id = file.bin_id AND file.in_storage=true AND file.deleted_at IS NULL WHERE bin.id = $1 GROUP BY bin.id LIMIT 1"
	err = d.db.QueryRow(sqlStatement, id).Scan(&bin.Id, &bin.Readonly, &bin.Downloads, &bin.Bytes, &bin.Files, &bin.UpdatedAt, &bin.CreatedAt, &bin.ApprovedAt, &bin.ExpiredAt, &bin.DeletedAt)
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
	bin.BytesReadable = humanize.Bytes(bin.Bytes)
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
	if bin.IsApproved() {
		bin.ApprovedAt.Time = bin.ApprovedAt.Time.UTC()
		bin.ApprovedAtRelative = humanize.Time(bin.ApprovedAt.Time)
	}
	bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
	if bin.IsDeleted() {
		bin.DeletedAt.Time = bin.DeletedAt.Time.UTC()
		bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
	}
	bin.URL = path.Join(bin.Id)
	return bin, true, nil
}

func (d *BinDao) Insert(bin *ds.Bin) (err error) {
	if err := d.ValidateInput(bin); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	downloads := uint64(0)
	updates := uint64(0)
	readonly := false
	bin.ExpiredAt = bin.ExpiredAt.UTC().Truncate(time.Microsecond)
	sqlStatement := "INSERT INTO bin (id, readonly, downloads, updates, updated_at, created_at, expired_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id"
	if err := d.db.QueryRow(sqlStatement, bin.Id, readonly, downloads, updates, now, now, bin.ExpiredAt).Scan(&bin.Id); err != nil {
		return err
	}
	bin.UpdatedAt = now
	bin.CreatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
	bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
	if bin.IsDeleted() {
		bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
	}
	bin.Downloads = downloads
	bin.Readonly = readonly
	return nil
}

func (d *BinDao) Upsert(bin *ds.Bin) (err error) {
	if err := d.ValidateInput(bin); err != nil {
		return err
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	downloads := uint64(0)
	updates := uint64(0)
	readonly := false
	bin.ExpiredAt = bin.ExpiredAt.UTC().Truncate(time.Microsecond)
	sqlStatement := "INSERT INTO bin (id, readonly, downloads, updates, updated_at, created_at, approved_at, expired_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id"
	err = d.db.QueryRow(sqlStatement, bin.Id, readonly, downloads, updates, now, now, bin.ApprovedAt, bin.ExpiredAt).Scan(&bin.Id)
	if err == nil {
		bin.UpdatedAt = now
		bin.CreatedAt = now
		bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
		bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
		if bin.IsApproved() {
			bin.ApprovedAt.Time = bin.ApprovedAt.Time.UTC()
			bin.ApprovedAtRelative = humanize.Time(bin.ApprovedAt.Time)
		}
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
		if bin.IsDeleted() {
			bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
		}
		bin.Downloads = downloads
		bin.Readonly = readonly
	}

	b, found, err := d.GetById(bin.Id)
	if err != nil {
		return err
	}

	if found {
		bin = &b
	}

	return nil
}

func (d *BinDao) Update(bin *ds.Bin) (err error) {
	var id string
	now := time.Now().UTC().Truncate(time.Microsecond)
	bin.ExpiredAt = bin.ExpiredAt.UTC().Truncate(time.Microsecond)
	if err := d.ValidateInput(bin); err != nil {
		return err
	}
	sqlStatement := "UPDATE bin SET readonly = $1, updated_at = $2, approved_at = $3, expired_at = $4, deleted_at = $5, updates = $6 WHERE id = $7 RETURNING id"
	err = d.db.QueryRow(sqlStatement, bin.Readonly, now, bin.ApprovedAt, bin.ExpiredAt, bin.DeletedAt, bin.Updates, bin.Id).Scan(&id)
	if err != nil {
		return err
	}
	bin.UpdatedAt = now
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	if bin.IsApproved() {
		bin.ApprovedAt.Time = bin.ApprovedAt.Time.UTC()
		bin.ApprovedAtRelative = humanize.Time(bin.ApprovedAt.Time)
	}
	if bin.IsDeleted() {
		bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
	}
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

func (d *BinDao) RegisterUpdate(bin *ds.Bin) (err error) {
	bin.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET updates = udates + 1, updated_at = $1 WHERE id = $2 RETURNING updates"
	err = d.db.QueryRow(sqlStatement, bin.UpdatedAt, bin.Id).Scan(&bin.Updates)
	if err != nil {
		return err
	}
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return nil
}
