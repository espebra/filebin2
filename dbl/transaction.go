package dbl

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"path"
	"strconv"
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

	//u, err := url.Parse(r.RequestURI)
	//if err != nil {
	//	fmt.Printf("Unable to parse path: %s: %s\n", t.Path, err.Error())
	//}

	tr := &ds.Transaction{}
	//tr.BinId = inputBin
	//tr.Filename = inputFilename
	tr.Method = r.Method
	tr.Path = r.URL.String()
	tr.IP = r.RemoteAddr

	// Remove the port if it's part of RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		tr.IP = host
	}

	reqHeaders, err := httputil.DumpRequest(r, false)
	if err != nil {
		fmt.Printf("Unable to parse request: %s\n", err.Error())
	}
	tr.Headers = string(reqHeaders)
	tr.Timestamp = timestamp
	tr.Status = status
	tr.RespBytes = size

	// XXX: It would be nice to count request body bytes instead
	if r.Header.Get("content-length") != "" {
		i, err := strconv.Atoi(r.Header.Get("content-length"))
		if err != nil {
			// Did not get a valid content-length header, so
			// set the log entry to -1 bytes to show that
			// something is wrong. The request should be
			// aborted, but the logging should not.
			tr.ReqBytes = -1
		}
		tr.ReqBytes = i
	}

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
	sqlStatement := "SELECT id, bin_id, filename, operation, method, path, ip, headers, timestamp, req_bytes, resp_bytes, status FROM transaction WHERE bin_id = $1 ORDER BY timestamp DESC"
	rows, err := d.db.Query(sqlStatement, bin)
	if err != nil {
		return transactions, err
	}
	for rows.Next() {
		var t ds.Transaction
		err = rows.Scan(&t.Id, &t.BinId, &t.Filename, &t.Operation, &t.Method, &t.Path, &t.IP, &t.Headers, &t.Timestamp, &t.ReqBytes, &t.RespBytes, &t.Status)
		if err != nil {
			return transactions, err
		}
		t.TimestampRelative = humanize.Time(t.Timestamp)

		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (d *TransactionDao) Insert(t *ds.Transaction) (err error) {
	//now := time.Now().UTC().Truncate(time.Microsecond)
	//t.FinishedAt.Time = now

	// Ignore these since they are not actually downloading or uploading file content
	if t.Method == "HEAD" {
		return nil
	}

	if t.Method == "POST" && t.Path == path.Join("/", t.BinId, t.Filename) {
		t.Operation = "file-upload"
	} else if t.Method == "GET" && t.Path == path.Join("/", t.BinId, t.Filename) {
		t.Operation = "file-download"
	} else if t.Path == path.Join("/archive", t.BinId, "zip") {
		t.Operation = "zip-download"
	} else if t.Path == path.Join("/archive", t.BinId, "tar") {
		t.Operation = "tar-download"
	} else if t.Method == "DELETE" && t.Path == path.Join("/", t.BinId) && t.Filename == "" {
		t.Operation = "bin-delete"
	} else if t.Method == "DELETE" && t.Path == path.Join("/", t.BinId, t.Filename) && t.Filename != "" {
		t.Operation = "file-delete"
	} else if t.Method == "PUT" && t.Path == path.Join("/", t.BinId) && t.Filename == "" {
		t.Operation = "bin-lock"
	}

	sqlStatement := "INSERT INTO transaction (bin_id, filename, operation, method, path, ip, headers, timestamp, status, req_bytes, resp_bytes) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id"
	if err := d.db.QueryRow(sqlStatement, t.BinId, t.Filename, t.Operation, t.Method, t.Path, t.IP, t.Headers, t.Timestamp, t.Status, t.ReqBytes, t.RespBytes).Scan(&t.Id); err != nil {
		return err
	}
	t.TimestampRelative = humanize.Time(t.Timestamp)
	return nil
}

func (d *TransactionDao) Update(t *ds.Transaction) (err error) {
	var id string
	//now := time.Now().UTC().Truncate(time.Microsecond)
	sqlStatement := "UPDATE transaction SET headers = $1 WHERE id = $2 RETURNING id"
	err = d.db.QueryRow(sqlStatement, t.Headers, t.Id).Scan(&id)
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
