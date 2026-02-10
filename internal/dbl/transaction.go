package dbl

import (
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
)

type TransactionDao struct {
	db      *sql.DB
	metrics DBMetricsObserver
}

func (d *TransactionDao) Register(r *http.Request, bin string, filename string, timestamp time.Time, completed time.Time, status int, size int64) (transaction *ds.Transaction, err error) {
	// Clean up before logging
	r.Header.Del("authorization")

	tr := &ds.Transaction{}
	tr.BinId = bin
	tr.Filename = filename
	tr.Method = r.Method
	tr.Path = r.URL.Path
	tr.IP = r.RemoteAddr

	// Remove the port if it's part of RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		tr.IP = host
	}

	reqHeaders, err := httputil.DumpRequest(r, false)
	if err != nil {
		slog.Error("unable to dump request", "error", err)
	}

	tr.Headers = string(reqHeaders)
	tr.Timestamp = timestamp
	tr.CompletedAt = completed
	tr.Status = status
	tr.RespBytes = size

	// XXX: It would be nice to count request body bytes instead
	if r.Header.Get("content-length") != "" {
		i, err := strconv.ParseInt(r.Header.Get("content-length"), 10, 0)
		if err != nil {
			// Did not get a valid content-length header, so
			// set the log entry to -1 bytes to show that
			// something is wrong. The request should be
			// aborted, but the logging should not.
			tr.ReqBytes = -1
		} else {
			tr.ReqBytes = i
		}
	}

	err = d.Insert(tr)
	return tr, err
}

func (d *TransactionDao) GetByIP(ip string) (transactions []ds.Transaction, err error) {
	sqlStatement := "SELECT id, bin_id, filename, operation, method, path, ip, headers, timestamp, req_bytes, resp_bytes, status, completed FROM transaction WHERE ip = $1 ORDER BY timestamp DESC"
	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, ip)
	observeQuery(d.metrics, "transaction_get_by_ip", t0, err)
	if err != nil {
		return transactions, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var t ds.Transaction
		err = rows.Scan(&t.Id, &t.BinId, &t.Filename, &t.Operation, &t.Method, &t.Path, &t.IP, &t.Headers, &t.Timestamp, &t.ReqBytes, &t.RespBytes, &t.Status, &t.CompletedAt)
		if err != nil {
			return transactions, err
		}
		t.TimestampRelative = humanize.Time(t.Timestamp)
		if t.ReqBytes >= 0 {
			t.ReqBytesReadable = humanize.Bytes(uint64(t.ReqBytes))
		}
		t.RespBytesReadable = humanize.Bytes(uint64(t.RespBytes))
		t.Duration = t.CompletedAt.Sub(t.Timestamp)

		transactions = append(transactions, t)
	}
	if err = rows.Err(); err != nil {
		return transactions, err
	}
	return transactions, nil
}

func (d *TransactionDao) GetByBin(bin string) (transactions []ds.Transaction, err error) {
	sqlStatement := "SELECT id, bin_id, filename, operation, method, path, ip, headers, timestamp, req_bytes, resp_bytes, status, completed FROM transaction WHERE bin_id = $1 ORDER BY timestamp DESC"
	t0 := time.Now()
	rows, err := d.db.Query(sqlStatement, bin)
	observeQuery(d.metrics, "transaction_get_by_bin", t0, err)
	if err != nil {
		return transactions, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var t ds.Transaction
		err = rows.Scan(&t.Id, &t.BinId, &t.Filename, &t.Operation, &t.Method, &t.Path, &t.IP, &t.Headers, &t.Timestamp, &t.ReqBytes, &t.RespBytes, &t.Status, &t.CompletedAt)
		if err != nil {
			return transactions, err
		}
		t.TimestampRelative = humanize.Time(t.Timestamp)
		if t.ReqBytes >= 0 {
			t.ReqBytesReadable = humanize.Bytes(uint64(t.ReqBytes))
		}
		t.RespBytesReadable = humanize.Bytes(uint64(t.RespBytes))
		t.Duration = t.CompletedAt.Sub(t.Timestamp)

		transactions = append(transactions, t)
	}
	if err = rows.Err(); err != nil {
		return transactions, err
	}
	return transactions, nil
}

func (d *TransactionDao) Insert(t *ds.Transaction) (err error) {
	sqlStatement := "INSERT INTO transaction (bin_id, filename, operation, method, path, ip, headers, timestamp, status, req_bytes, resp_bytes, completed) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, t.BinId, t.Filename, t.Operation, t.Method, t.Path, t.IP, t.Headers, t.Timestamp, t.Status, t.ReqBytes, t.RespBytes, t.CompletedAt).Scan(&t.Id)
	observeQuery(d.metrics, "transaction_insert", t0, err)
	if err != nil {
		return err
	}
	t.TimestampRelative = humanize.Time(t.Timestamp)
	return nil
}

func (d *TransactionDao) Update(t *ds.Transaction) (err error) {
	var id string
	sqlStatement := "UPDATE transaction SET headers = $1 WHERE id = $2 RETURNING id"
	t0 := time.Now()
	err = d.db.QueryRow(sqlStatement, t.Headers, t.Id).Scan(&id)
	observeQuery(d.metrics, "transaction_update", t0, err)
	if err != nil {
		return err
	}
	return nil
}

func (d *TransactionDao) Cleanup(retention uint64) (count int64, err error) {
	sqlStatement := "DELETE FROM transaction WHERE timestamp < NOW() - ($1 || ' days')::interval"
	t0 := time.Now()
	res, err := d.db.Exec(sqlStatement, retention)
	observeQuery(d.metrics, "transaction_cleanup", t0, err)
	if err != nil {
		return count, err
	}
	n, err := res.RowsAffected()
	count = n
	if err != nil {
		return count, err
	}
	return count, nil
}
