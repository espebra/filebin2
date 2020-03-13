package main

import (
	"log"
	"os"
	//"fmt"
	"flag"
	//"github.com/espebra/filebin2/ds"
	"github.com/GeertJohan/go.rice"
	"github.com/espebra/filebin2/dbl"
)

var (
	// HTTP
	listenHostFlag = flag.String("listen-host", "127.0.0.1", "Listen host")
	listenPortFlag = flag.Int("listen-port", 8080, "Listen port")

	// Database
	dbHostFlag     = flag.String("db-host", "127.0.0.1", "Database host")
	dbPortFlag     = flag.Int("db-port", 5432, "Database port")
	dbNameFlag     = flag.String("db-name", os.Getenv("DATABASE_NAME"), "Name of the database")
	dbUsernameFlag = flag.String("db-username", os.Getenv("DATABASE_USERNAME"), "Database username")
	dbPasswordFlag = flag.String("db-password", os.Getenv("DATABASE_PASSWORD"), "Database password")
)

func main() {
	flag.Parse()

	daoconn, err := dbl.Init(*dbHostFlag, *dbPortFlag, *dbNameFlag, *dbUsernameFlag, *dbPasswordFlag)
	if err != nil {
		log.Fatal("Unable to connect to the database: ", err)
	}

	if err := daoconn.CreateSchema(); err != nil {
		log.Fatal("Unable to create Schema:", err)
	}

	staticBox := rice.MustFindBox("static")
	templateBox := rice.MustFindBox("templates")

	h := &HTTP{
		httpHost:    *listenHostFlag,
		httpPort:    *listenPortFlag,
		staticBox:   staticBox,
		templateBox: templateBox,
		dao:         &daoconn,
	}

	if err := h.Init(); err != nil {
		log.Fatal("Unable to start the HTTP server:", err)
	}

	h.Run()
}
