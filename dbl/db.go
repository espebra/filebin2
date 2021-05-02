package dbl

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
)

type DAO struct {
	db             *sql.DB
	ConnStr        string
	binDao         *BinDao
	fileDao        *FileDao
	infoDao        *InfoDao
	transactionDao *TransactionDao
}

// Init a database connection given
// a database name and user.
func Init(dbHost string, dbPort int, dbName, dbUser, dbPassword string) (DAO, error) {
	var dao DAO
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return dao, err
	}
	if err != nil {
		return dao, err
	}
	if err := db.Ping(); err != nil {
		return dao, errors.New(fmt.Sprintf("Unable to ping the database: %s:%d\n", dbHost, dbPort))
	}
	dao = DAO{db: db}
	dao.ConnStr = connStr
	dao.binDao = &BinDao{db: db}
	dao.fileDao = &FileDao{db: db}
	dao.infoDao = &InfoDao{db: db}
	dao.transactionDao = &TransactionDao{db: db}
	return dao, nil
}

func (dao DAO) Close() error {
	return dao.db.Close()
}

func (dao DAO) CreateSchema() error {
	// Not implemented
	return nil
}

func (dao DAO) ResetDB() error {
	sqlStatements := []string{
		"DELETE FROM file",
		"DELETE FROM bin",
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

func (dao DAO) Info() *InfoDao {
	return dao.infoDao
}

func (dao DAO) Transaction() *TransactionDao {
	return dao.transactionDao
}

func (dao DAO) Status() bool {
	if err := dao.db.Ping(); err != nil {
		fmt.Printf("Database status check returned: %s\n", err.Error())
		return false
	}
	return true
}
