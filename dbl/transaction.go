package dbl

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"time"

	"github.com/espebra/filebin2/ds"

	"github.com/dustin/go-humanize"
	//"github.com/gorilla/mux"
)

type TransactionDao struct {
	db *sql.DB
}

func (d *TransactionDao) Register(r *http.Request, timestamp time.Time, status int, size int) (transaction *ds.Transaction, err error) {
	// Clean up before logging
	inputBin := r.Header.Get("bin")
	inputFilename := r.Header.Get("filename")

	if inputBin == "" {
		fmt.Printf("Skip logging\n")
		// No need to log a transaction that is not related to a bin
		return
	}

	if inputFilename == "" {
		fmt.Printf("Skip logging\n")
		// No need to log a transaction that is not related to a bin
		return
	}

	//u, err := url.Parse(r.RequestURI)
	//if err != nil {
	//	fmt.Printf("Unable to parse path: %s: %s\n", t.Path, err.Error())
	//}

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
	tr.Timestamp = timestamp
	tr.Status = status
	tr.Bytes = size
	err = d.Insert(tr)
	return tr, err
}

//func (d *TransactionDao) Finish(tr *ds.Transaction) (err error) {
//	var id string
//	now := time.Now().UTC().Truncate(time.Microsecond)
//	sqlStatement := "UPDATE transaction SET finished_at = $1 WHERE id = $2 RETURNING id"
//	err = d.db.QueryRow(sqlStatement, now, tr.Id).Scan(&id)
//	if err != nil {
//		return err
//	}
//	tr.FinishedAt.Time = now
//	tr.FinishedAtRelative = humanize.Time(tr.FinishedAt.Time)
//	return nil
//}

func (d *TransactionDao) GetByBin(bin string) (transactions []ds.Transaction, err error) {
	sqlStatement := "SELECT id, bin_id, filename, method, path, ip, trace, timestamp, bytes, status FROM transaction WHERE bin_id = $1 ORDER BY timestamp DESC"
	rows, err := d.db.Query(sqlStatement, bin)
	if err != nil {
		return transactions, err
	}
	for rows.Next() {
		var t ds.Transaction
		err = rows.Scan(&t.Id, &t.BinId, &t.Filename, &t.Method, &t.Path, &t.IP, &t.Trace, &t.Timestamp, &t.Bytes, &t.Status)
		if err != nil {
			return transactions, err
		}
		t.TimestampRelative = humanize.Time(t.Timestamp)

		u, err := url.Parse(t.Path)
		if err != nil {
			fmt.Printf("Unable to parse path: %s: %s\n", t.Path, err.Error())
		}

		// Ignore these since they are not actually downloading or uploading file content
		if t.Method == "HEAD" {
			continue
		}

		if t.Method == "POST" && u.Path == "/" {
			t.Type = "file-upload"
		} else if t.Method == "GET" && u.Path == path.Join("/", t.BinId, t.Filename) {
			t.Type = "file-download"
		} else if u.Path == path.Join("/archive", t.BinId, "zip") {
			t.Type = "zip-download"
		} else if u.Path == path.Join("/archive", t.BinId, "tar") {
			t.Type = "tar-download"
		} else if t.Method == "DELETE" && u.Path == path.Join("/", t.BinId) && t.Filename == "" {
			t.Type = "bin-delete"
		} else if t.Method == "DELETE" && u.Path == path.Join("/", t.BinId, t.Filename) && t.Filename != "" {
			t.Type = "file-delete"
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (d *TransactionDao) Insert(t *ds.Transaction) (err error) {
	//now := time.Now().UTC().Truncate(time.Microsecond)
	//t.FinishedAt.Time = now
	sqlStatement := "INSERT INTO transaction (bin_id, filename, method, path, ip, trace, timestamp, status, bytes) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id"
	if err := d.db.QueryRow(sqlStatement, t.BinId, t.Filename, t.Method, t.Path, t.IP, t.Trace, t.Timestamp, t.Status, t.Bytes).Scan(&t.Id); err != nil {
		return err
	}
	t.TimestampRelative = humanize.Time(t.Timestamp)
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

func (d *TransactionDao) Cleanup() (count int64, err error) {
	sqlStatement := "DELETE FROM transaction USING bin WHERE transaction.bin_id=bin.id AND bin.deleted_at IS NOT NULL AND bin.deleted_at < NOW() - INTERVAL '30 days'"
	res, err := d.db.Exec(sqlStatement)
	if err != nil {
		return count, err
	}
	n, err := res.RowsAffected()
	count = n
	if err != nil {
		return count, err
	}

	sqlStatement = "DELETE FROM transaction WHERE NOT bin_id IN (SELECT id FROM bin)"
	res, err = d.db.Exec(sqlStatement)
	if err != nil {
		return count, err
	}
	n, err = res.RowsAffected()
	count = count + n
	if err != nil {
		return count, err
	}
	return count, nil
}
