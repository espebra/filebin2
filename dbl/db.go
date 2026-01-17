package dbl

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

//go:embed schema.sql
var schemaSQL string

type DAO struct {
	db             *sql.DB
	ConnStr        string
	binDao         *BinDao
	fileDao        *FileDao
	fileContentDao *FileContentDao
	metricsDao     *MetricsDao
	transactionDao *TransactionDao
	clientDao      *ClientDao
}

// Init a database connection given
// a database name and user.
func Init(dbHost string, dbPort int, dbName, dbUser, dbPassword string, maxOpenConns, maxIdleConns int) (DAO, error) {
	var dao DAO
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return dao, err
	}
	if err := db.Ping(); err != nil {
		return dao, errors.New(fmt.Sprintf("Unable to ping the database: %s:%d\n", dbHost, dbPort))
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

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
			fmt.Printf("Error in reset: %s\n", err.Error())
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
		fmt.Printf("Database status check returned: %s\n", err.Error())
		return false
	}
	return true
}

func (dao DAO) Stats() sql.DBStats {
	return dao.db.Stats()
}
