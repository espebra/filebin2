package dbl

import (
	"testing"
)

const (
	testDbName     = "db"
	testDbUser     = "username"
	testDbPassword = "changeme"
	testDbHost     = "db"
	testDbPort     = 5432
)

func tearUp() (DAO, error) {
	dao, err := Init(testDbHost, testDbPort, testDbName, testDbUser, testDbPassword)
	if err != nil {
		return dao, err
	}
	if err := dao.ResetDB(); err != nil {
		return dao, err
	}
	return dao, nil
}

func tearDown(dao DAO) error {
	if err := dao.ResetDB(); err != nil {
		return err
	}
	err := dao.Close()
	return err
}

func TestDbInit(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	err = tearDown(dao)
	if err != nil {
		t.Error(err)
	}
}

func TestFailingInit(t *testing.T) {
	_, err := Init("", 0, "", "", "")
	if err == nil {
		t.Error("Was expecting to fail here, invalid user and db name were provided.")
	}
}

func TestPing(t *testing.T) {
	dao, err := tearUp()
	if err != nil {
		t.Error(err)
	}
	status := dao.Status()
	if status == false {
		t.Error("Was expecting the database status check to return true")
	}
	err = tearDown(dao)
	if err != nil {
		t.Error(err)
	}
}
