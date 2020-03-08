package main

import (
	"fmt"
	//"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/dbl"
)


const (
        DbName     = "db"
        DbUser     = "username"
        DbPassword = "changeme"
        DbHost     = "db"
        DbPort     = 5432
)

func main() {
        _, err := dbl.Init(DbHost, DbPort, DbName, DbUser, DbPassword)
        if err != nil {
                fmt.Printf("%s\n", err)
        }
}
