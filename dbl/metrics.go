package dbl

import (
	"database/sql"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"time"
)

type MetricsDao struct {
	db *sql.DB
}

func (d *MetricsDao) StorageBytesAllocated() (totalBytes uint64) {
	// Assume that each file consumes at least 256KB, to align with minimum billable object size in AWS.
	minBytes := 262144
	sqlStatement := "SELECT COALESCE((SELECT SUM(GREATEST(bytes, $1)) FROM file WHERE in_storage = true AND deleted_at IS NULL), 0)"
	if err := d.db.QueryRow(sqlStatement, minBytes).Scan(&totalBytes); err != nil {
		fmt.Printf("Unable to calculate total storage bytes allocated: %s\n", err.Error())
	}
	return totalBytes
}

func (d *MetricsDao) UpdateMetrics(metrics *ds.Metrics) (err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)

	tmp := ds.Metrics{}

	// Number of log entries
	sqlStatement := "SELECT COUNT(Id) FROM transaction"
	if err := d.db.QueryRow(sqlStatement).Scan(&tmp.CurrentLogEntries); err != nil {
		return err
	}

	// Number of current files
	sqlStatement = "SELECT COUNT(Id) FROM file WHERE in_storage = true AND deleted_at IS NULL"
	if err := d.db.QueryRow(sqlStatement).Scan(&tmp.CurrentFiles); err != nil {
		return err
	}

	// Total number of bytes of current files
	if metrics.CurrentFiles > 0 {
		sqlStatement = "SELECT SUM(bytes) FROM file WHERE in_storage = true AND deleted_at IS NULL"
		if err := d.db.QueryRow(sqlStatement).Scan(&tmp.CurrentBytes); err != nil {
			return err
		}
	}

	// Number of current bins
	sqlStatement = "SELECT COUNT(Id) FROM bin WHERE expired_at > $1 AND deleted_at IS NULL"
	if err := d.db.QueryRow(sqlStatement, now).Scan(&tmp.CurrentBins); err != nil {
		return err
	}

	metrics.Lock()
	metrics.CurrentBins = tmp.CurrentBins
	metrics.CurrentLogEntries = tmp.CurrentLogEntries
	metrics.CurrentFiles = tmp.CurrentFiles
	metrics.CurrentBytes = tmp.CurrentBytes

	metrics.CurrentFilesReadable = humanize.Comma(metrics.CurrentFiles)
	metrics.CurrentBinsReadable = humanize.Comma(metrics.CurrentBins)
	metrics.CurrentBytesReadable = humanize.Bytes(uint64(metrics.CurrentBytes))

	metrics.TotalFilesReadable = humanize.Comma(metrics.TotalFiles)
	metrics.TotalBinsReadable = humanize.Comma(metrics.TotalBins)
	metrics.TotalBytesReadable = humanize.Bytes(uint64(metrics.TotalBytes))
	metrics.Unlock()

	return nil
}
