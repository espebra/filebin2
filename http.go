package main

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"strings"
	"time"

	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/geoip"
	"github.com/espebra/filebin2/s3"
	"github.com/espebra/filebin2/workspace"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

type funcHandler func(http.ResponseWriter, *http.Request)

type HTTP struct {
	router          *mux.Router
	templateBox     *embed.FS
	staticBox       *embed.FS
	templates       *template.Template
	dao             *dbl.DAO
	s3              *s3.S3AO
	geodb           *geoip.DAO
	workspace       *workspace.Manager
	config          *ds.Config
	metrics         *ds.Metrics
	metricsRegistry *prometheus.Registry
}

func (h *HTTP) Init() (err error) {
	h.router = mux.NewRouter()
	h.templates = h.ParseTemplates()

	h.router.HandleFunc("/debug/pprof/cmdline", h.auth(pprof.Cmdline)).Methods(http.MethodGet)
	h.router.HandleFunc("/debug/pprof/profile", h.auth(pprof.Profile)).Methods(http.MethodGet)
	h.router.HandleFunc("/debug/pprof/symbol", h.auth(pprof.Symbol)).Methods(http.MethodGet)
	h.router.HandleFunc("/debug/pprof/trace", h.auth(pprof.Trace)).Methods(http.MethodGet)
	h.router.PathPrefix("/debug/pprof/").HandlerFunc(h.auth(pprof.Index))

	h.router.HandleFunc("/", h.index).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/", h.clientLookup(h.uploadFile)).Methods(http.MethodPost)
	h.router.HandleFunc("/filebin-status", h.filebinStatus).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/storage-status", h.storageStatus).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/metrics", h.viewMetrics).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/robots.txt", h.robots).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/about", h.about).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api", h.api).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api.yaml", h.apiSpec).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/privacy", h.privacy).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/contact", h.contact).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/terms", h.terms).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/integration/slack", h.integrationSlack).Methods(http.MethodPost)
	h.router.HandleFunc("/admin/log/{category:[a-z]+}/{filter:[A-Za-z0-9.:_-]+}", h.auth(h.viewAdminLog)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/bins", h.auth(h.viewAdminBins)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/bins/{limit:[0-9]+}", h.auth(h.viewAdminBins)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/bins/all", h.auth(h.viewAdminBinsAll)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/clients", h.auth(h.viewAdminClients)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/clients/all", h.auth(h.viewAdminClientsAll)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/files", h.auth(h.viewAdminFiles)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/files/{limit:[0-9]+}", h.auth(h.viewAdminFiles)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/file/{sha256:[0-9a-z]+}", h.auth(h.viewAdminFile)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/file/{sha256:[0-9a-z]+}/block", h.log(h.auth(h.blockFileContent))).Methods("POST")
	h.router.HandleFunc("/admin", h.auth(h.viewAdminDashboard)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/approve/{bin:[A-Za-z0-9_-]+}", h.log(h.auth(h.approveBin))).Methods("PUT")
	//h.router.HandleFunc("/admin/cleanup", h.Auth(h.ViewAdminCleanup)).Methods(http.MethodHead, http.MethodGet)
	h.router.Handle("/static/{path:.*}", CacheControl(http.FileServer(http.FS(h.staticBox)))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/archive/{bin:[A-Za-z0-9_-]+}/{format:[a-z]+}", h.log(h.clientLookup(h.archive))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}.txt", h.viewBinPlainText).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/qr/{bin:[A-Za-z0-9_-]+}", h.binQR).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/", h.viewBinRedirect).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.viewBin).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.clientLookup(h.deleteBin))).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.clientLookup(h.lockBin))).Methods("PUT")
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.clientLookup(h.banBin))).Methods("BAN")
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.clientLookup(h.getFile))).Methods(http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.clientLookup(h.deleteFile))).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.clientLookup(h.uploadFile))).Methods(http.MethodPost, http.MethodPut)

	h.config.ExpirationDuration = time.Second * time.Duration(h.config.Expiration)
	return err
}

func CacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		h.ServeHTTP(w, r)
	})
}

func (h *HTTP) clientLookup(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, found, err := h.dao.Client().GetByRemoteAddr(r.RemoteAddr)
		if err != nil {
			h.Error(w, r, fmt.Sprintf("Failed to select client with remote addr %s: %s", r.RemoteAddr, err.Error()), "Database error", 991, http.StatusInternalServerError)
			return
		}
		if found {
			// Existing client IP to be updated
			//fmt.Printf("Existing IP address: %s\n", client.IP)
		} else {
			// New client IP to be registered
			//fmt.Printf("New IP address: %s\n", client.IP)

		}

		if client.IsBanned() {
			//fmt.Printf("Client IP %s is banned.\n", client.IP)
			fmt.Printf("Rejecting request from client ip %s due to ban %s (%s) by %s.\n", client.IP, client.BannedAtRelative, client.BannedAt.Time.Format("2006-01-02 15:04:05 UTC"), client.BannedBy)
			http.Error(w, "This client IP address has been banned.", http.StatusForbidden)
			return
		}

		if err := h.geodb.Lookup(r.RemoteAddr, &client); err != nil {
			fmt.Printf("Unable to look up geoip details for %s: %s\n", r.RemoteAddr, err.Error())
		}

		// Check the client details against the ban filter here
		fn(w, r)

		h.dao.Client().Update(&client)
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

	// Add gzip compression, but exclude /archive endpoints (they're already compressed)
	var handler http.Handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/archive/") {
			// Skip compression for archive endpoints
			h.router.ServeHTTP(w, r)
		} else {
			// Apply compression for all other endpoints
			handlers.CompressHandler(h.router).ServeHTTP(w, r)
		}
	})

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

	fmt.Printf("HTTP server read timeout: %s\n", h.config.ReadTimeout)
	fmt.Printf("HTTP server read header timeout: %s\n", h.config.ReadHeaderTimeout)
	fmt.Printf("HTTP server idle timeout: %s\n", h.config.IdleTimeout)
	fmt.Printf("HTTP server write timeout: %s\n", h.config.WriteTimeout)

	// Set up the server
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", h.config.HttpHost, h.config.HttpPort),
		Handler:           handler,
		ReadTimeout:       h.config.ReadTimeout,
		WriteTimeout:      h.config.WriteTimeout,
		IdleTimeout:       h.config.IdleTimeout,
		ReadHeaderTimeout: h.config.ReadHeaderTimeout,
	}

	// Start the server
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Failed to start HTTP server: %s\n", err.Error())
		os.Exit(2)
	}
}

func (h *HTTP) Error(w http.ResponseWriter, r *http.Request, internal string, external string, errno int, statusCode int) {
	w.Header().Set("Cache-Control", "max-age=1")
	w.Header().Set("X-Robots-Tag", "noindex")

	if internal != "" {
		fmt.Printf("Errno %d: %q\n", errno, internal)
	}

	// Disregard any request body there is
	io.Copy(ioutil.Discard, r.Body)

	type Data struct {
		ds.Common
		Text       string
		ErrNo      int
		StatusCode int
	}

	var data Data
	data.Text = external
	data.ErrNo = errno
	data.StatusCode = statusCode

	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		h.metrics.IncrErrorPageViewCount()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(statusCode)
		if err := h.templates.ExecuteTemplate(w, "error", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 914", http.StatusInternalServerError)
			return
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(statusCode)
		io.WriteString(w, external)
	}
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
		"lowercase": func(s string) string {
			return strings.ToLower(s)
		},
		"isBanned": func(client ds.Client) bool {
			if client.IsBanned() {
				return true
			}
			return false
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

func (h *HTTP) cookieVerify(w http.ResponseWriter, r *http.Request) bool {

	// Skip cookie verification for certain user agents.
	agent := r.Header.Get("user-agent")
	filter := []string{"Wget", "curl", "VLC"}
	for _, f := range filter {
		if strings.Contains(agent, f) {
			// Skip cookie verification if the user-agent match the filter
			return true
		}
	}

	// Check if cookie exists
	if cookie, err := r.Cookie("verified"); err == nil {
		// Yes, the cookie exists.
		// Check if the cookie has the expected value
		if cookie.Value == h.config.ExpectedCookieValue {
			// Yes, the value is correct.
			// Cookie exists and contains the expected value. Ok to continue.
			return true
		}
	}

	// false indicates that the client didn't provide a proper cookie. Should show the warning.
	return false
}

func (h *HTTP) setVerificationCookie(w http.ResponseWriter, r *http.Request) {
	// The cookie does not exist or its value was wrong
	cookie := http.Cookie{}
	cookie.Name = "verified"
	cookie.Value = h.config.ExpectedCookieValue
	cookie.Expires = time.Now().Add(time.Duration(h.config.CookieLifetime) * 24 * time.Hour)
	cookie.Secure = true
	cookie.HttpOnly = true
	cookie.Path = "/"
	http.SetCookie(w, &cookie)
}

func extractIP(addr string) (string, error) {
	var ip net.IP
	var err error
	var host string

	// Try to parse the addr directly
	ip = net.ParseIP(addr)

	// If parsing directly failed, try to split host from port
	if ip == nil {
		host, _, err = net.SplitHostPort(addr)
		if err == nil {
			ip = net.ParseIP(host)
		}
		if err != nil {
			fmt.Printf("Unable to split host port for %s, returning %s: %s\n", addr, ip.String(), err.Error())
		}
	}
	return ip.String(), err
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
