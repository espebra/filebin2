package dbl

import (
	"database/sql"
	//"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"time"
)

type InfoDao struct {
	db *sql.DB
}

func (d *InfoDao) GetInfo() (info ds.Info, err error) {
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Number of log entries
	sqlStatement := "SELECT COUNT(Id) FROM transaction"
	if err := d.db.QueryRow(sqlStatement).Scan(&info.CurrentLogEntries); err != nil {
		return info, err
	}

	// Number of current files
	sqlStatement = "SELECT COUNT(Id) FROM file WHERE in_storage = true AND deleted_at IS NULL"
	if err := d.db.QueryRow(sqlStatement).Scan(&info.CurrentFiles); err != nil {
		return info, err
	}

	// Total number of bytes of current files
	if info.CurrentFiles > 0 {
		sqlStatement = "SELECT SUM(bytes) FROM file WHERE in_storage = true AND deleted_at IS NULL"
		if err := d.db.QueryRow(sqlStatement).Scan(&info.CurrentBytes); err != nil {
			return info, err
		}
	}

	// Number of current bins
	sqlStatement = "SELECT COUNT(Id) FROM bin WHERE expired_at > $1 AND deleted_at IS NULL"
	if err := d.db.QueryRow(sqlStatement, now).Scan(&info.CurrentBins); err != nil {
		return info, err
	}

	// Number of total files since day 1
	sqlStatement = "SELECT COUNT(Id) FROM file"
	if err := d.db.QueryRow(sqlStatement).Scan(&info.TotalFiles); err != nil {
		return info, err
	}

	// Total number of bytes of all files since day 1
	if info.TotalFiles > 0 {
		sqlStatement = "SELECT SUM(bytes) FROM file"
		if err := d.db.QueryRow(sqlStatement).Scan(&info.TotalBytes); err != nil {
			return info, err
		}
	}

	// Total number of bins since day 1
	sqlStatement = "SELECT COUNT(Id) FROM bin"
	if err := d.db.QueryRow(sqlStatement).Scan(&info.TotalBins); err != nil {
		return info, err
	}

	info.CurrentFilesReadable = humanize.Comma(info.CurrentFiles)
	info.CurrentBinsReadable = humanize.Comma(info.CurrentBins)
	info.CurrentBytesReadable = humanize.Bytes(uint64(info.CurrentBytes))

	info.TotalFilesReadable = humanize.Comma(info.TotalFiles)
	info.TotalBinsReadable = humanize.Comma(info.TotalBins)
	info.TotalBytesReadable = humanize.Bytes(uint64(info.TotalBytes))

	return info, nil
}
