package dbl

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// escapeLikePattern escapes the LIKE/ILIKE wildcard characters in s so it
// can be embedded in a pattern and matched literally.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// DBMetricsObserver allows DAO to report metrics without importing the ds package.
type DBMetricsObserver interface {
	ObserveDBQuery(operation string, duration time.Duration)
	IncrDBQueryError(operation string)
}

// observeQuery records query duration and errors. Safe to call with nil metrics.
func observeQuery(m DBMetricsObserver, operation string, t0 time.Time, err error) {
	if m == nil {
		return
	}
	m.ObserveDBQuery(operation, time.Since(t0))
	if err != nil && err != sql.ErrNoRows {
		m.IncrDBQueryError(operation)
	}
}

//go:embed schema.sql
var schemaSQL string

type DAO struct {
	db             *sql.DB
	lockDB         *sql.DB
	asyncDB        *sql.DB
	metrics        DBMetricsObserver
	binDao         *BinDao
	fileDao        *FileDao
	fileContentDao *FileContentDao
	metricsDao     *MetricsDao
	transactionDao *TransactionDao
	clientDao      *ClientDao
}

type DBConfig struct {
	Host            string
	Port            int
	Name            string
	Username        string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func Init(cfg DBConfig) (DAO, error) {
	var dao DAO
	// binary_parameters makes the driver send a query with parameters in a
	// single round trip, instead of preparing it in one round trip and
	// executing it in another.
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable binary_parameters=yes", cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Name)

	// Retry logic for database connection with 30 second timeout
	retryTimeout := 30 * time.Second
	retryInterval := 2 * time.Second
	startTime := time.Now()

	var db *sql.DB
	var err error

	for {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			elapsed := time.Since(startTime)
			if elapsed >= retryTimeout {
				return dao, fmt.Errorf("unable to open database connection after %.0fs: %s", elapsed.Seconds(), err.Error())
			}
			slog.Warn("database not available yet, retrying",
				"elapsed_seconds", elapsed.Seconds(),
				"retry_interval_seconds", retryInterval.Seconds(),
				"error", err)
			time.Sleep(retryInterval)
			continue
		}

		err = db.Ping()
		if err == nil {
			slog.Info("connected to database", "host", cfg.Host, "port", cfg.Port)
			break
		}

		_ = db.Close()
		elapsed := time.Since(startTime)
		if elapsed >= retryTimeout {
			return dao, fmt.Errorf("unable to ping database after %.0fs: %s:%d", elapsed.Seconds(), cfg.Host, cfg.Port)
		}
		slog.Warn("database not available yet, retrying",
			"elapsed_seconds", elapsed.Seconds(),
			"retry_interval_seconds", retryInterval.Seconds(),
			"error", err)
		time.Sleep(retryInterval)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Dedicated pool for session-scoped advisory locks. Lock connections are
	// pinned for the duration of a lock, and a lock holder issues regular
	// queries while holding it. If lock connections came from the main pool,
	// enough concurrent lock holders would pin every slot and their own
	// queries would wait forever for a free connection, deadlocking the
	// process. Keeping the pools separate makes that impossible.
	lockDB, err := sql.Open("postgres", connStr)
	if err != nil {
		_ = db.Close()
		return dao, fmt.Errorf("unable to open lock connection pool: %w", err)
	}
	lockDB.SetMaxOpenConns(cfg.MaxOpenConns)
	lockDB.SetMaxIdleConns(2)
	lockDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	lockDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Dedicated pool for the bookkeeping writes that happen on every
	// request: the transaction log, the client activity and the download
	// counters. Its sessions commit without waiting for the commit to be
	// flushed to disk, which lets PostgreSQL flush many of these commits
	// at once instead of one by one. The writes are visible immediately,
	// but a crash of the database server can lose the ones from the last
	// fraction of a second. Writes that must not be lost, such as uploads,
	// use the main pool.
	asyncDB, err := sql.Open("postgres", connStr+" synchronous_commit=off")
	if err != nil {
		_ = db.Close()
		_ = lockDB.Close()
		return dao, fmt.Errorf("unable to open async commit connection pool: %w", err)
	}
	asyncDB.SetMaxOpenConns(cfg.MaxOpenConns)
	asyncDB.SetMaxIdleConns(cfg.MaxIdleConns)
	asyncDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	asyncDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	dao = DAO{db: db, lockDB: lockDB, asyncDB: asyncDB}
	dao.binDao = &BinDao{db: db}
	dao.fileDao = &FileDao{db: db, asyncDB: asyncDB}
	dao.fileContentDao = &FileContentDao{db: db, lockDB: lockDB}
	dao.metricsDao = &MetricsDao{db: db}
	dao.transactionDao = &TransactionDao{db: asyncDB}
	dao.clientDao = &ClientDao{db: db, asyncDB: asyncDB}

	// Create schema if it doesn't exist
	if err := dao.CreateSchema(); err != nil {
		return dao, fmt.Errorf("failed to create schema: %w", err)
	}

	return dao, nil
}

func (dao DAO) Close() error {
	var lockErr, asyncErr error
	if dao.lockDB != nil {
		lockErr = dao.lockDB.Close()
	}
	if dao.asyncDB != nil {
		asyncErr = dao.asyncDB.Close()
	}
	if err := dao.db.Close(); err != nil {
		return err
	}
	return errors.Join(lockErr, asyncErr)
}

func (dao DAO) CreateSchema() error {
	// Execute the embedded schema SQL
	if _, err := dao.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	return nil
}

func (dao DAO) ResetDB() error {
	sqlStatements := []string{
		"DELETE FROM file",
		"DELETE FROM file_content",
		"DELETE FROM bin",
		"DELETE FROM client",
		"DELETE FROM transaction"}

	for _, s := range sqlStatements {
		if _, err := dao.db.Exec(s); err != nil {
			slog.Error("error in database reset", "statement", s, "error", err)
			return err
		}
	}
	return nil
}

func (dao DAO) Bin() *BinDao {
	return dao.binDao
}

func (dao DAO) File() *FileDao {
	return dao.fileDao
}

func (dao DAO) FileContent() *FileContentDao {
	return dao.fileContentDao
}

func (dao DAO) Metrics() *MetricsDao {
	return dao.metricsDao
}

func (dao DAO) Transaction() *TransactionDao {
	return dao.transactionDao
}

func (dao DAO) Client() *ClientDao {
	return dao.clientDao
}

func (dao DAO) Status() bool {
	if err := dao.db.Ping(); err != nil {
		slog.Warn("database status check failed", "error", err)
		return false
	}
	return true
}

func (dao DAO) Stats() sql.DBStats {
	return dao.db.Stats()
}

// SetMetrics sets the metrics observer for database operations.
func (dao *DAO) SetMetrics(m DBMetricsObserver) {
	dao.metrics = m
	dao.binDao.metrics = m
	dao.fileDao.metrics = m
	dao.fileContentDao.metrics = m
	dao.metricsDao.metrics = m
	dao.transactionDao.metrics = m
	dao.clientDao.metrics = m
}
