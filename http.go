package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"strings"
	//"encoding/json"
	"github.com/GeertJohan/go.rice"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/s3"
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
	s3          *s3.S3AO
}

func (h *HTTP) Init() (err error) {
	h.router = mux.NewRouter()
	h.templates = h.ParseTemplates()

	h.router.HandleFunc("/", h.Index).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/", h.Upload).Methods(http.MethodPost)
	h.router.Handle("/static/{path:.*}", http.StripPrefix("/static/", http.FileServer(h.staticBox.HTTPBox()))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.ViewBin).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.DeleteBin).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.LockBin).Methods("LOCK")
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.GetFile).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.DeleteFile).Methods(http.MethodDelete)
	return err
}

func (h *HTTP) Run() {
	fmt.Printf("Starting HTTP server on %s:%d\n", h.httpHost, h.httpPort)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", h.httpHost, h.httpPort), handlers.CompressHandler(h.router))
	if err != nil {
		fmt.Errorf("Failed to start HTTP server: %s", err.Error())
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
				fmt.Errorf("%s", err.Error())
			}
			// Parse the template
			_, err = templ.Parse(content)
			if err != nil {
				fmt.Errorf("%s", err.Error())
			}
		}
		return err
	})
	if err != nil {
		fmt.Errorf("%s", err.Error())
	}
	return templ
}
