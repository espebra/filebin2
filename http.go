package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	//"encoding/json"
	"github.com/GeertJohan/go.rice"
	"github.com/espebra/filebin2/dbl"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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
	err := h.templateBox.Walk("/", func(filepath string, info os.FileInfo, err error) error {
		if strings.HasSuffix(filepath, ".html") {
			// Read the template
			f := path.Base(filepath)
			//log.Println("Loading template: " + f)
			content, err := h.templateBox.String(f)
			if err != nil {
				log.Fatal(err)
			}
			// Parse the template
			_, err = templ.Parse(content)
			if err != nil {
				log.Fatal(err)
			}
		}
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
	return templ
}
