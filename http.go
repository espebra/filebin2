package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	//"strings"
	//"path/filepath"
	//"encoding/json"
	"github.com/espebra/filebin2/dbl"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	//"github.com/espebra/filebin2/ds"
	"github.com/GeertJohan/go.rice"
)

type funcHandler func(http.ResponseWriter, *http.Request)

type HTTP struct {
	httpPort    int
	httpHost    string
	router      *mux.Router
	templateBox *rice.Box
	staticBox   *rice.Box
	templates   *template.Template
	dao         *dbl.DAO
}

func (h *HTTP) Init() (err error) {
	h.router = mux.NewRouter()
	h.templates = h.ParseTemplates()

	h.router.HandleFunc("/", h.Index).Methods(http.MethodGet, http.MethodHead)
	h.router.Handle("/static/{path:.*}", http.StripPrefix("/static/", http.FileServer(h.staticBox.HTTPBox()))).Methods("GET", "HEAD")
	return err
}

func (h *HTTP) Run() {
	log.Println("Starting HTTP server on " + h.httpHost + ":" + fmt.Sprintf("%d", h.httpPort))
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", h.httpHost, h.httpPort), handlers.CompressHandler(h.router))
	if err != nil {
		log.Fatal("Failed to start HTTP server:", err)
	}
}

// Parse all templates
func (h *HTTP) ParseTemplates() *template.Template {

	// Functions that are available from within templates
	var fns = template.FuncMap{}

	templ := template.New("").Funcs(fns)
	//err := filepath.Walk(*templateDirFlag, func(path string, info os.FileInfo, err error) error {
	//	if strings.HasSuffix(path, ".html") {
	//		_, err = templ.ParseFiles(path)
	//		if err != nil {
	//			log.Println(err)
	//		}
	//	}

	//	return err
	//})

	//if err != nil {
	//	log.Fatal(err)
	//}

	return templ
}
