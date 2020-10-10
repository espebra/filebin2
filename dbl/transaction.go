package dbl

import (
	"database/sql"
	//"errors"
	//"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	//"math/rand"
	//"path"
	//"regexp"
	//"strings"
	"time"
)

type TransactionDao struct {
	db *sql.DB
}

func (d *TransactionDao) GetByBin(bin string) (transactions []ds.Transaction, err error) {
	sqlStatement := "SELECT id, bin_id, method, path, ip, trace, started_at, finished_at FROM transaction WHERE bin_id = $1 ORDER BY started_at DESC"
	rows, err := d.db.Query(sqlStatement, bin)
	if err != nil {
		return transactions, err
	}
	for rows.Next() {
		var t ds.Transaction
		err = rows.Scan(&t.Id, &t.Bin, &t.Method, &t.Path, &t.IP, &t.Trace, &t.StartedAt, &t.FinishedAt)
		if err != nil {
			return transactions, err
		}
		t.StartedAtRelative = humanize.Time(t.StartedAt)
		t.FinishedAtRelative = humanize.Time(t.FinishedAt)
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (d *TransactionDao) Insert(t *ds.Transaction) (err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	t.FinishedAt = now
	sqlStatement := "INSERT INTO transaction (bin_id, method, path, ip, trace, started_at, finished_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id"
	if err := d.db.QueryRow(sqlStatement, t.Bin, t.Method, t.Path, t.IP, t.Trace, t.StartedAt, t.FinishedAt).Scan(&t.Id); err != nil {
		return err
	}
	t.StartedAtRelative = humanize.Time(t.StartedAt)
	t.FinishedAtRelative = humanize.Time(t.FinishedAt)
	return nil
}

func (d *TransactionDao) Update(t *ds.Transaction) (err error) {
	var id string
	//now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE transaction SET trace = $1 WHERE id = $2 RETURNING id"
	err = d.db.QueryRow(sqlStatement, t.Trace, t.Id).Scan(&id)
	if err != nil {
		return err
	}
	//bin.UpdatedAt = now
	//bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	return nil
}
