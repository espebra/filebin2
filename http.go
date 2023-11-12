package main

import (
	"database/sql"
	"embed"
	"fmt"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/geoip"
	"github.com/espebra/filebin2/s3"
	"github.com/felixge/httpsnoop"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	//"strings"
	"time"
)

type funcHandler func(http.ResponseWriter, *http.Request)

type HTTP struct {
	router      *mux.Router
	templateBox *embed.FS
	staticBox   *embed.FS
	templates   *template.Template
	dao         *dbl.DAO
	s3          *s3.S3AO
	geodb       *geoip.DAO
	config      *ds.Config
	metrics     *ds.Metrics
}

func (h *HTTP) Init() (err error) {
	h.router = mux.NewRouter()
	h.templates = h.ParseTemplates()

	h.router.HandleFunc("/debug/pprof/cmdline", h.auth(pprof.Cmdline)).Methods(http.MethodGet)
	h.router.HandleFunc("/debug/pprof/profile", h.auth(pprof.Profile)).Methods(http.MethodGet)
	h.router.HandleFunc("/debug/pprof/symbol", h.auth(pprof.Symbol)).Methods(http.MethodGet)
	h.router.HandleFunc("/debug/pprof/trace", h.auth(pprof.Trace)).Methods(http.MethodGet)
	h.router.PathPrefix("/debug/pprof/").HandlerFunc(h.auth(pprof.Index))

	h.router.HandleFunc("/", h.banLookup(h.index)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/", h.banLookup(h.uploadFile)).Methods(http.MethodPost)
	h.router.HandleFunc("/filebin-status", h.filebinStatus).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/storage-status", h.storageStatus).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/metrics", h.viewMetrics).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/robots.txt", h.robots).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/about", h.banLookup(h.about)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api", h.banLookup(h.api)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api.yaml", h.banLookup(h.apiSpec)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/privacy", h.banLookup(h.privacy)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/terms", h.banLookup(h.terms)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/integration/slack", h.log(h.integrationSlack)).Methods(http.MethodPost)
	h.router.HandleFunc("/admin/log/{category:[a-z]+}/{filter:[A-Za-z0-9.:_-]+}", h.auth(h.viewAdminLog)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/bins", h.auth(h.viewAdminBins)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/bins/{limit:[0-9]+}", h.auth(h.viewAdminBins)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/bins/all", h.auth(h.viewAdminBinsAll)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/files", h.auth(h.viewAdminFiles)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/files/{limit:[0-9]+}", h.auth(h.viewAdminFiles)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/file/{sha256:[0-9a-z]+}", h.auth(h.viewAdminFile)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin", h.auth(h.viewAdminDashboard)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/approve/{bin:[A-Za-z0-9_-]+}", h.log(h.auth(h.approveBin))).Methods("PUT")
	//h.router.HandleFunc("/admin/cleanup", h.Auth(h.ViewAdminCleanup)).Methods(http.MethodHead, http.MethodGet)
	h.router.Handle("/static/{path:.*}", CacheControl(http.FileServer(http.FS(h.staticBox)))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/archive/{bin:[A-Za-z0-9_-]+}/{format:[a-z]+}", h.log(h.banLookup(h.archive))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/qr/{bin:[A-Za-z0-9_-]+}", h.banLookup(h.binQR)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/", h.banLookup(h.viewBinRedirect)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.banLookup(h.viewBin)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.banLookup(h.deleteBin))).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.banLookup(h.lockBin))).Methods("PUT")
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.banLookup(h.getFile))).Methods(http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.banLookup(h.deleteFile))).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.banLookup(h.uploadFile))).Methods(http.MethodPost, http.MethodPut)

	h.config.ExpirationDuration = time.Second * time.Duration(h.config.Expiration)
	return err
}

func CacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		h.ServeHTTP(w, r)
	})
}

func (h *HTTP) banLookup(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//ip := net.ParseIP(r.RemoteAddr)

		//client, err := h.geodb.Lookup(ip)
		//if err != nil {
		//	fmt.Printf("Unable to look up geoip details for %s: %s\n", r.RemoteAddr, err.Error())
		//}

		//// Check the client details against the ban filter here
		//fmt.Printf("Request: %s %s, client: %s\n", r.Method, r.URL.String(), client.String())
		fn(w, r)
	})
}

func (h *HTTP) log(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()

		params := mux.Vars(r)
		bin := params["bin"]
		filename := params["filename"]

		metrics := httpsnoop.CaptureMetricsFn(w, func(w http.ResponseWriter) {
			fn(w, r)
		})

		completed := t0.Add(metrics.Duration)

		// Only register transactions with a response status lower than 400, which
		// includes file uploads, archive downloads and presigned downloads from S3.
		// However, any client or server side errors are skipped to reduce the database
		// volume needed.
		if metrics.Code < 400 {
			_, err := h.dao.Transaction().Register(r, bin, filename, t0, completed, metrics.Code, metrics.Written)
			if err != nil {
				fmt.Printf("Unable to register the transaction: %s\n", err.Error())
			}
		}
	})
}

func (h *HTTP) auth(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Let the client know authentication is required
		w.Header().Set("WWW-Authenticate", "Basic realm='Filebin'")

		// Abort here if the admin username or password is not set
		if h.config.AdminUsername == "" || h.config.AdminPassword == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Read the authorization request header
		username, password, ok := r.BasicAuth()
		if ok == false {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if username != h.config.AdminUsername || password != h.config.AdminPassword {
			time.Sleep(3 * time.Second)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		fn(w, r)
	}
}

func (h *HTTP) Run() {
	fmt.Printf("Starting HTTP server on %s:%d\n", h.config.HttpHost, h.config.HttpPort)

	// Add gzip compression
	handler := handlers.CompressHandler(h.router)

	// Add access logging
	accessLog, err := os.OpenFile(h.config.HttpAccessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer accessLog.Close()
	if err != nil {
		fmt.Printf("Unable to open log file: %s\n", err.Error())
		os.Exit(2)
	}
	handler = handlers.CombinedLoggingHandler(accessLog, handler)

	// Add proxy header handling
	if h.config.HttpProxyHeaders {
		handler = handlers.ProxyHeaders(handler)
	}

	// Set up the server
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", h.config.HttpHost, h.config.HttpPort),
		Handler:           handler,
		ReadTimeout:       1 * time.Hour,
		WriteTimeout:      1 * time.Hour,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	// Start the server
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Failed to start HTTP server: %s\n", err.Error())
		os.Exit(2)
	}
}

func (h *HTTP) Error(w http.ResponseWriter, r *http.Request, internal string, external string, errno int, statusCode int) {
	if internal != "" {
		fmt.Printf("Errno %d: %q\n", errno, internal)
	}

	// Disregard any request body there is
	io.Copy(ioutil.Discard, r.Body)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	io.WriteString(w, external)
	return
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
		"isApproved": func(bin ds.Bin) bool {
			if bin.IsApproved() {
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
		"durationInSeconds": func(dur time.Duration) string {
			return fmt.Sprintf("%.3f", dur.Seconds())
		},
		"join": func(s ...string) string {
			return path.Join(s...)
		},
		"plus": func(a uint64, b uint64) uint64 {
			return a + b
		},
		"unescapeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	templ := template.New("").Funcs(fns)
	_, err := templ.ParseFS(h.templateBox, "templates/*.html")
	if err != nil {
		fmt.Printf("Unable to read templates directory: %s\n", err.Error())
		os.Exit(2)
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

func setRobotsPermissions(w http.ResponseWriter, allow bool) {
	if allow {
		w.Header().Set("X-Robots-Tag", "index, follow, noarchive")
	} else {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	}
}
