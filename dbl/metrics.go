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
	sqlStatement := "SELECT COALESCE((SELECT SUM(GREATEST(file.bytes, $1)) FROM file JOIN file_content ON file.sha256 = file_content.sha256 WHERE file_content.in_storage = true AND file.deleted_at IS NULL), 0)"
	if err := d.db.QueryRow(sqlStatement, minBytes).Scan(&totalBytes); err != nil {
		fmt.Printf("Unable to calculate total storage bytes allocated: %s\n", err.Error())
	}
	return totalBytes
}

func (d *MetricsDao) UpdateMetrics(metrics *ds.Metrics) (err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)

	var currentLogEntries, currentFiles, currentBytes, currentBins int64

	// Number of log entries
	sqlStatement := "SELECT COUNT(*) FROM transaction"
	if err := d.db.QueryRow(sqlStatement).Scan(&currentLogEntries); err != nil {
		return err
	}

	// Number of current files
	sqlStatement = "SELECT COUNT(*) FROM file JOIN file_content ON file.sha256 = file_content.sha256 WHERE file_content.in_storage = true AND file.deleted_at IS NULL"
	if err := d.db.QueryRow(sqlStatement).Scan(&currentFiles); err != nil {
		return err
	}

	// Total number of bytes of current files
	if currentFiles > 0 {
		sqlStatement = "SELECT SUM(file.bytes) FROM file JOIN file_content ON file.sha256 = file_content.sha256 WHERE file_content.in_storage = true AND file.deleted_at IS NULL"
		if err := d.db.QueryRow(sqlStatement).Scan(&currentBytes); err != nil {
			return err
		}
	}

	// Number of current bins
	sqlStatement = "SELECT COUNT(*) FROM bin WHERE expired_at > $1 AND deleted_at IS NULL"
	if err := d.db.QueryRow(sqlStatement, now).Scan(&currentBins); err != nil {
		return err
	}

	// Update metrics struct fields (no lock needed for Prometheus metrics as they are thread-safe)
	metrics.CurrentBins = currentBins
	metrics.CurrentLogEntries = currentLogEntries
	metrics.CurrentFiles = currentFiles
	metrics.CurrentBytes = currentBytes

	metrics.CurrentFilesReadable = humanize.Comma(metrics.CurrentFiles)
	metrics.CurrentBinsReadable = humanize.Comma(metrics.CurrentBins)
	metrics.CurrentBytesReadable = humanize.Bytes(uint64(metrics.CurrentBytes))

	metrics.TotalFilesReadable = humanize.Comma(metrics.TotalFiles)
	metrics.TotalBinsReadable = humanize.Comma(metrics.TotalBins)
	metrics.TotalBytesReadable = humanize.Bytes(uint64(metrics.TotalBytes))

	return nil
}
