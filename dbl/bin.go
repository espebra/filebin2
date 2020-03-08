package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"time"
)

type BinDao struct {
	db *sql.DB
}

func (d *BinDao) GetAll() ([]ds.Bin, error) {
	bins := []ds.Bin{}

	sqlStatement := "SELECT bin.id AS id, bin.bid AS bid, bin.updated AS updated, bin.created AS created FROM bin ORDER BY updated ASC"
	rows, err := d.db.Query(sqlStatement)
	if err != nil {
		return bins, err
	}
	for rows.Next() {
		var bin ds.Bin
		err = rows.Scan(&bin.Id, &bin.Bid, &bin.Updated, &bin.Created)
		if err != nil {
			return bins, err
		}

		// https://github.com/lib/pq/issues/329
		bin.Updated = bin.Updated.UTC()
		bin.Created = bin.Created.UTC()

		bin.UpdatedRelative = humanize.Time(bin.Updated)
		bin.CreatedRelative = humanize.Time(bin.Created)

		bins = append(bins, bin)
	}

	return bins, nil
}

func (d *BinDao) GetById(id int) (ds.Bin, error) {
	var bin ds.Bin

	// Get bin info
	sqlStatement := "SELECT id, bid, updated, created FROM bin WHERE id = $1 LIMIT 1"
	err := d.db.QueryRow(sqlStatement, id).Scan(&bin.Id, &bin.Bid, &bin.Updated, &bin.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return bin, errors.New(fmt.Sprintf("No bin found with id %d", id))
		}
	}
	// https://github.com/lib/pq/issues/329
	bin.Updated = bin.Updated.UTC()
	bin.Created = bin.Created.UTC()

	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)

	return bin, err
}

func (d *BinDao) GetByBid(bid string) (ds.Bin, error) {
	var bin ds.Bin

	// Get bin info
	sqlStatement := "SELECT id, bid, updated, created FROM bin WHERE bid = $1 LIMIT 1"
	err := d.db.QueryRow(sqlStatement, bid).Scan(&bin.Id, &bin.Bid, &bin.Updated, &bin.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return bin, errors.New(fmt.Sprintf("No bin found with bid %s", bid))
		}
	}
	// https://github.com/lib/pq/issues/329
	bin.Updated = bin.Updated.UTC()
	bin.Created = bin.Created.UTC()

	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)

	return bin, err
}

func (d *BinDao) Insert(bin *ds.Bin) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "INSERT INTO bin (bid, updated, created) VALUES ($1, $2, $3) RETURNING id"
	err := d.db.QueryRow(sqlStatement, bin.Bid, now, now).Scan(&bin.Id)
	if err != nil {
		return err
	}
	bin.Updated = now
	bin.Created = now
	bin.UpdatedRelative = humanize.Time(bin.Updated)
	bin.CreatedRelative = humanize.Time(bin.Created)
	return nil
}

func (d *BinDao) Update(bin *ds.Bin) error {
	var id int
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE bin SET updated = $1 WHERE id = $2 RETURNING id"
	err := d.db.QueryRow(sqlStatement, now, bin.Id).Scan(&id)
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