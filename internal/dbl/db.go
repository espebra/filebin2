package dbl

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

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
	ConnStr        string
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
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Name)

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

		db.Close()
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

	dao = DAO{db: db}
	dao.ConnStr = connStr
	dao.binDao = &BinDao{db: db}
	dao.fileDao = &FileDao{db: db}
	dao.fileContentDao = &FileContentDao{db: db}
	dao.metricsDao = &MetricsDao{db: db}
	dao.transactionDao = &TransactionDao{db: db}
	dao.clientDao = &ClientDao{db: db}

	// Create schema if it doesn't exist
	if err := dao.CreateSchema(); err != nil {
		return dao, fmt.Errorf("failed to create schema: %w", err)
	}

	return dao, nil
}

func (dao DAO) Close() error {
	return dao.db.Close()
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
