package main

import (
	//"encoding/json"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
	//"encoding/json"
	"github.com/GeertJohan/go.rice"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/s3"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type funcHandler func(http.ResponseWriter, *http.Request)

type HTTP struct {
	expiration         int
	expirationDuration time.Duration
	limitFileDownloads uint64
	limitStorage       uint64
	httpPort           int
	httpHost           string
	httpProxyHeaders   bool
	httpAccessLog      string
	adminUsername      string
	adminPassword      string
	tmpdir             string
	baseUrl            url.URL
	router             *mux.Router
	templateBox        *rice.Box
	staticBox          *rice.Box
	templates          *template.Template
	dao                *dbl.DAO
	s3                 *s3.S3AO
}

func (h *HTTP) Init() (err error) {
	h.router = mux.NewRouter()
	h.templates = h.ParseTemplates()

	h.router.HandleFunc("/", h.Index).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/", h.Log(h.Upload)).Methods(http.MethodPost)
	h.router.HandleFunc("/filebin-status", h.FilebinStatus).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/about", h.About).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api", h.API).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api.yaml", h.APISpec).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/privacy", h.Privacy).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/terms", h.Terms).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/archive/{bin:[A-Za-z0-9_-]+}/{format:[a-z]+}", h.Log(h.Archive)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/qr/{bin:[A-Za-z0-9_-]+}", h.BinQR).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/log/{bin:[A-Za-z0-9_-]+}", h.Auth(h.ViewAdminLog)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin", h.Auth(h.ViewAdminDashboard)).Methods(http.MethodHead, http.MethodGet)
	//h.router.HandleFunc("/admin/cleanup", h.Auth(h.ViewAdminCleanup)).Methods(http.MethodHead, http.MethodGet)
	h.router.Handle("/static/{path:.*}", http.StripPrefix("/static/", http.FileServer(h.staticBox.HTTPBox()))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.ViewBin).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.Log(h.DeleteBin)).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.Log(h.LockBin)).Methods("PUT")
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.Log(h.GetFile)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.Log(h.DeleteFile)).Methods(http.MethodDelete)

	h.expirationDuration = time.Second * time.Duration(h.expiration)
	return err
}

func (h *HTTP) Auth(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Let the client know authentication is required
		w.Header().Set("WWW-Authenticate", "Basic realm='Filebin'")

		// Abort here if the admin username or password is not set
		if h.adminUsername == "" || h.adminPassword == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Read the authorization request header
		username, password, ok := r.BasicAuth()
		if ok == false {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if username != h.adminUsername || password != h.adminPassword {
			time.Sleep(3 * time.Second)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		fn(w, r)
	}
}

func (h *HTTP) Log(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//tr, err := h.dao.Transaction().Start(r)
		//if err != nil {
		//	fmt.Printf("Unable to register the transaction: %s\n", err.Error())
		//}
		//defer h.dao.Transaction().Finish(tr)

		fn(w, r)
	}
}

func (h *HTTP) Run() {
	fmt.Printf("Starting HTTP server on %s:%d\n", h.httpHost, h.httpPort)

	// Add gzip compression
	handler := handlers.CompressHandler(h.router)

	// Add access logging
	accessLog, err := os.OpenFile(h.httpAccessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer accessLog.Close()
	if err != nil {
		fmt.Printf("Unable to open log file: %s\n", err.Error())
		os.Exit(2)
	}
	handler = handlers.CombinedLoggingHandler(accessLog, handler)

	// Add proxy header handling
	if h.httpProxyHeaders {
		handler = handlers.ProxyHeaders(handler)
	}

	// Set up the server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", h.httpHost, h.httpPort),
		Handler:      handler,
		ReadTimeout:  2 * time.Hour,
		WriteTimeout: 2 * time.Hour,
	}

	// Start the server
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Failed to start HTTP server: %s\n", err.Error())
		os.Exit(2)
	}
}

func (h *HTTP) Error(w http.ResponseWriter, r *http.Request, internal string, external string, errno int, statusCode int) {
	if internal != "" {
		fmt.Printf("Errno %d: %s\n", errno, internal)
	}

	//var msg ds.Message
	//msg.Id = errno
	//msg.Text = external

	w.WriteHeader(statusCode)
	io.WriteString(w, external)

	//if r.Header.Get("accept") == "application/json" {
	//	w.Header().Set("Content-Type", "application/json")
	//	out, err := json.MarshalIndent(msg, "", "    ")
	//	if err != nil {
	//		fmt.Printf("Failed to parse json: %s\n", err.Error())
	//		http.Error(w, "Errno 1000", http.StatusInternalServerError)
	//		return
	//	}
	//	io.WriteString(w, string(out))
	//} else {
	//	if err := h.templates.ExecuteTemplate(w, "message", msg); err != nil {
	//		fmt.Printf("Failed to execute template: %s\n", err.Error())
	//		http.Error(w, "Errno 1001", http.StatusInternalServerError)
	//		return
	//	}
	//}
}

// Parse all templates
func (h *HTTP) ParseTemplates() *template.Template {

	// Functions that are available from within templates
	var fns = template.FuncMap{
		"isAvailable": func(bin ds.Bin) bool {
			if bin.IsReadable() {
				return true
			}
			return false
		},
		"elapsed": func(t0, t1 time.Time) string {
			elapsed := t1.Sub(t0)
			return elapsed.String()
		},
		"finished": func(t sql.NullTime) bool {
			if t.Valid {
				if t.Time.IsZero() == false {
					return true
				}
			}
			return false
		},
		"join": func(s ...string) string {
			return path.Join(s...)
		},
	}

	templ := template.New("").Funcs(fns)
	err := h.templateBox.Walk("/", func(filepath string, info os.FileInfo, err error) error {
		if strings.HasSuffix(filepath, ".tpl") {
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

func extractIP(addr string) (ip string, err error) {
	host, _, _ := net.SplitHostPort(addr)
	//if err != nil {
	//	fmt.Printf("Error 1: %s\n", err.Error())
	//	return ip, err
	//}
	ipRaw := net.ParseIP(host)
	return ipRaw.String(), nil
}

func inStringSlice(needle string, haystack []string) bool {
	for i := range haystack {
		if haystack[i] == needle {
			return true
		}
	}
	return false
}
