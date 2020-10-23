package dbl

import (
	"database/sql"
	"fmt"
	"net"
	"path"
	"net/http"
	"net/url"
	"net/http/httputil"
	"time"

	"github.com/espebra/filebin2/ds"

	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
)

type TransactionDao struct {
	db *sql.DB
}

func (d *TransactionDao) Start(r *http.Request) (transaction *ds.Transaction, err error) {
	// Clean up before logging
	r.Header.Del("Authorization")

	params := mux.Vars(r)
	inputBin := params["bin"]
	if inputBin == "" {
		inputBin = r.Header.Get("bin")
	}
	inputFilename := params["filename"]
	if inputFilename == "" {
		inputFilename = r.Header.Get("filename")
	}

	tr := &ds.Transaction{}
	tr.BinId = inputBin
	tr.Filename = inputFilename
	tr.Method = r.Method
	tr.Path = r.URL.String()

	tr.IP = r.RemoteAddr

	// Remove the port if it's part of RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		tr.IP = host
	}

	reqTrace, err := httputil.DumpRequest(r, false)
	if err != nil {
		fmt.Printf("Unable to parse request: %s\n", err.Error())
	}
	tr.Trace = string(reqTrace)
	tr.StartedAt = time.Now().UTC().Truncate(time.Microsecond)
	err = d.Insert(tr)
	return tr, err
}

func (d *TransactionDao) Finish(tr *ds.Transaction) (err error) {
	var id string
	now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE transaction SET finished_at = $1 WHERE id = $2 RETURNING id"
	err = d.db.QueryRow(sqlStatement, now, tr.Id).Scan(&id)
	if err != nil {
		return err
	}
	tr.FinishedAt.Time = now
	tr.FinishedAtRelative = humanize.Time(tr.FinishedAt.Time)
	return nil
}

func (d *TransactionDao) GetByBin(bin string) (transactions []ds.Transaction, err error) {
	sqlStatement := "SELECT id, bin_id, filename, method, path, ip, trace, started_at, finished_at FROM transaction WHERE bin_id = $1 ORDER BY started_at DESC"
	rows, err := d.db.Query(sqlStatement, bin)
	if err != nil {
		return transactions, err
	}
	for rows.Next() {
		var t ds.Transaction
		err = rows.Scan(&t.Id, &t.BinId, &t.Filename, &t.Method, &t.Path, &t.IP, &t.Trace, &t.StartedAt, &t.FinishedAt)
		if err != nil {
			return transactions, err
		}
		t.StartedAtRelative = humanize.Time(t.StartedAt)
		t.FinishedAtRelative = humanize.Time(t.FinishedAt.Time)

		u, err := url.Parse(t.Path)
		if err != nil {
			fmt.Printf("Unable to parse path: %s: %s\n", t.Path, err.Error())
		}

		if u.Path == "/" {
			t.Type = "file-upload"
		} else if u.Path == path.Join("/archive", t.BinId, "zip") {
			t.Type = "zip-download"
		} else if u.Path == path.Join("/archive", t.BinId, "tar") {
			t.Type = "tar-download"
		} else if u.Path == path.Join("/", t.BinId, t.Filename) {
			t.Type = "file-download"
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (d *TransactionDao) Insert(t *ds.Transaction) (err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	t.FinishedAt.Time = now
	sqlStatement := "INSERT INTO transaction (bin_id, filename, method, path, ip, trace, started_at, finished_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id"
	if err := d.db.QueryRow(sqlStatement, t.BinId, t.Filename, t.Method, t.Path, t.IP, t.Trace, t.StartedAt, t.FinishedAt).Scan(&t.Id); err != nil {
		return err
	}
	t.StartedAtRelative = humanize.Time(t.StartedAt)
	t.FinishedAtRelative = humanize.Time(t.FinishedAt.Time)
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
