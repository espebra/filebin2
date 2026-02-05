package dbl

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
)

type PostgresStats struct {
	Version               string  `json:"version"`
	DatabaseSize          uint64  `json:"database_size"`
	DatabaseSizeReadable  string  `json:"database_size_readable"`
	ActiveConnections     int     `json:"active_connections"`
	TxCommitted           int64   `json:"tx_committed"`
	TxRolledBack          int64   `json:"tx_rolled_back"`
	CacheHitRatio         float64 `json:"cache_hit_ratio"`
	CacheHitRatioReadable string  `json:"cache_hit_ratio_readable"`
	TotalDeadTuples       int64   `json:"total_dead_tuples"`
}

func (d *MetricsDao) PostgresStats() (PostgresStats, error) {
	var stats PostgresStats

	// Version
	if err := d.db.QueryRow("SELECT version()").Scan(&stats.Version); err != nil {
		return stats, fmt.Errorf("query version: %w", err)
	}

	// Database size
	if err := d.db.QueryRow("SELECT pg_database_size(current_database())").Scan(&stats.DatabaseSize); err != nil {
		return stats, fmt.Errorf("query database size: %w", err)
	}
	stats.DatabaseSizeReadable = humanize.Bytes(stats.DatabaseSize)

	// Active connections
	if err := d.db.QueryRow("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()").Scan(&stats.ActiveConnections); err != nil {
		return stats, fmt.Errorf("query active connections: %w", err)
	}

	// Transaction and cache stats
	var blksRead, blksHit int64
	if err := d.db.QueryRow("SELECT xact_commit, xact_rollback, blks_read, blks_hit FROM pg_stat_database WHERE datname = current_database()").Scan(&stats.TxCommitted, &stats.TxRolledBack, &blksRead, &blksHit); err != nil {
		return stats, fmt.Errorf("query database stats: %w", err)
	}
	if blksHit+blksRead > 0 {
		stats.CacheHitRatio = float64(blksHit) / float64(blksHit+blksRead)
	}
	stats.CacheHitRatioReadable = fmt.Sprintf("%.2f%%", stats.CacheHitRatio*100)

	// Dead tuples
	if err := d.db.QueryRow("SELECT COALESCE(SUM(n_dead_tup), 0) FROM pg_stat_user_tables").Scan(&stats.TotalDeadTuples); err != nil {
		return stats, fmt.Errorf("query dead tuples: %w", err)
	}

	return stats, nil
}

type MetricsDao struct {
	db *sql.DB
}

func (d *MetricsDao) StorageBytesAllocated() (totalBytes uint64) {
	// Assume that each file consumes at least 256KB, to align with minimum billable object size in AWS.
	minBytes := 262144
	sqlStatement := "SELECT COALESCE((SELECT SUM(GREATEST(file_content.bytes, $1)) FROM file JOIN file_content ON file.sha256 = file_content.sha256 WHERE file_content.in_storage = true AND file.deleted_at IS NULL), 0)"
	if err := d.db.QueryRow(sqlStatement, minBytes).Scan(&totalBytes); err != nil {
		slog.Error("unable to calculate total storage bytes allocated", "error", err)
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
		sqlStatement = "SELECT SUM(file_content.bytes) FROM file JOIN file_content ON file.sha256 = file_content.sha256 WHERE file_content.in_storage = true AND file.deleted_at IS NULL"
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
