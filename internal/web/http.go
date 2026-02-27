package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/geoip"
	"github.com/espebra/filebin2/internal/s3"
	"github.com/espebra/filebin2/internal/workspace"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

//go:embed templates
var templateBox embed.FS

//go:embed static
var staticBox embed.FS

type AdminLogin struct {
	IP                 string
	Hostname           string
	LastActive         time.Time
	LastActiveReadable string
}

type HTTP struct {
	router           *mux.Router
	templateBox      *embed.FS
	staticBox        *embed.FS
	templates        *template.Template
	dao              *dbl.DAO
	s3               *s3.S3AO
	geodb            *geoip.DAO
	workspace        *workspace.Manager
	config           *ds.Config
	metrics          *ds.Metrics
	metricsRegistry  *prometheus.Registry
	adminLogins      []AdminLogin
	adminLoginsMutex sync.Mutex
	siteMessage      ds.SiteMessage
	siteMessageMutex sync.RWMutex
	startedAt        time.Time

	// Cache for storage bytes to avoid expensive DB query on every request
	storageBytesCache uint64
	storageBytesMutex sync.RWMutex

	// Stop channel for graceful shutdown of background goroutines
	stopChan chan struct{}
}

// New creates a new HTTP server instance
func New(dao *dbl.DAO, s3ao *s3.S3AO, geodb *geoip.DAO, wm *workspace.Manager, config *ds.Config, metrics *ds.Metrics, metricsRegistry *prometheus.Registry) *HTTP {
	return &HTTP{
		templateBox:     &templateBox,
		staticBox:       &staticBox,
		dao:             dao,
		s3:              s3ao,
		geodb:           geodb,
		workspace:       wm,
		config:          config,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
		startedAt:       time.Now(),
	}
}

// getCachedStorageBytes returns the cached storage bytes value
func (h *HTTP) getCachedStorageBytes() uint64 {
	h.storageBytesMutex.RLock()
	defer h.storageBytesMutex.RUnlock()
	return h.storageBytesCache
}

// startStorageBytesUpdater starts a background goroutine that updates the storage bytes cache every minute
func (h *HTTP) startStorageBytesUpdater() {
	// Initialize stop channel
	h.stopChan = make(chan struct{})

	// Initial update
	h.storageBytesMutex.Lock()
	h.storageBytesCache = h.dao.Metrics().StorageBytesAllocated()
	h.storageBytesMutex.Unlock()

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				bytes := h.dao.Metrics().StorageBytesAllocated()
				h.storageBytesMutex.Lock()
				h.storageBytesCache = bytes
				h.storageBytesMutex.Unlock()
			case <-h.stopChan:
				return
			}
		}
	}()
}

// Stop gracefully shuts down background goroutines
func (h *HTTP) Stop() {
	if h.stopChan != nil {
		close(h.stopChan)
	}
}

func (h *HTTP) Init() error {
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
	h.router.HandleFunc("/admin/bins/all", h.auth(h.viewAdminBinsAll)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/clients", h.auth(h.viewAdminClients)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/clients/all", h.auth(h.viewAdminClientsAll)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/files", h.auth(h.viewAdminFiles)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/filecontent", h.auth(h.viewAdminFileContent)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/file/{sha256:[0-9a-z]+}", h.auth(h.viewAdminFile)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/file/{sha256:[0-9a-z]+}/block", h.log(h.auth(h.blockFileContent))).Methods("POST")
	h.router.HandleFunc("/admin/file/{sha256:[0-9a-z]+}/unblock", h.log(h.auth(h.unblockFileContent))).Methods("POST")
	h.router.HandleFunc("/admin/file/{sha256:[0-9a-z]+}/delete", h.log(h.auth(h.deleteFileContent))).Methods("POST")
	h.router.HandleFunc("/admin/message", h.auth(h.viewAdminSiteMessage)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/message", h.log(h.auth(h.updateSiteMessage))).Methods("POST")
	h.router.HandleFunc("/admin", h.auth(h.viewAdminDashboard)).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/admin/approve/{bin:[A-Za-z0-9_-]+}", h.log(h.auth(h.approveBin))).Methods("PUT")
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

	// Start background updater for storage bytes cache
	h.startStorageBytesUpdater()

	return nil
}

func CacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		h.ServeHTTP(w, r)
	})
}

func (h *HTTP) clientLookup(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, _, err := h.dao.Client().GetByRemoteAddr(r.RemoteAddr)
		if err != nil {
			h.Error(w, r, fmt.Sprintf("Failed to select client with remote addr %s: %s", r.RemoteAddr, err.Error()), "Database error", 991, http.StatusInternalServerError)
			return
		}

		if client.IsBanned() {
			slog.Warn("rejecting request from banned client", "ip", client.IP, "banned_at", client.BannedAt.Time.Format("2006-01-02 15:04:05 UTC"), "banned_by", client.BannedBy)
			http.Error(w, "This client IP address has been banned.", http.StatusForbidden)
			return
		}

		if err := h.geodb.Lookup(r.RemoteAddr, &client); err != nil {
			slog.Debug("unable to look up geoip details", "remote_addr", r.RemoteAddr, "error", err)
		}

		// Check the client details against the ban filter here
		fn(w, r)

		_ = h.dao.Client().Update(&client)
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

		// Derive a handler name from the matched route template
		handler := "unknown"
		if route := mux.CurrentRoute(r); route != nil {
			if tmpl, err := route.GetPathTemplate(); err == nil {
				handler = tmpl
			}
		}
		h.metrics.ObserveHTTPRequest(r.Method, handler, metrics.Duration, metrics.Code)

		// Only register transactions with a response status lower than 400, which
		// includes file uploads, archive downloads and presigned downloads from S3.
		// However, any client or server side errors are skipped to reduce the database
		// volume needed.
		if metrics.Code < 400 {
			_, err := h.dao.Transaction().Register(r, bin, filename, t0, completed, metrics.Code, metrics.Written)
			if err != nil {
				slog.Error("unable to register transaction", "error", err)
			}
		}
	})
}

func (h *HTTP) auth(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Abort here if the admin username or password is not set
		if h.config.AdminUsername == "" || h.config.AdminPassword == "" {
			w.Header().Set("WWW-Authenticate", "Basic realm='Filebin'")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Read the authorization request header
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", "Basic realm='Filebin'")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Use constant-time comparison to prevent timing attacks
		usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(h.config.AdminUsername)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(h.config.AdminPassword)) == 1

		if !usernameMatch || !passwordMatch {
			time.Sleep(3 * time.Second)
			w.Header().Set("WWW-Authenticate", "Basic realm='Filebin'")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		h.trackAdminLogin(r.RemoteAddr)
		fn(w, r)
	}
}

func (h *HTTP) trackAdminLogin(remoteAddr string) {
	// Extract IP from remoteAddr (format: "IP:port")
	host, _, err := net.SplitHostPort(remoteAddr)
	var ip string
	if err == nil {
		ip = host
	} else {
		ip = remoteAddr
	}

	h.adminLoginsMutex.Lock()
	defer h.adminLoginsMutex.Unlock()

	now := time.Now().UTC()

	// Check if IP already exists and update it
	for i := range h.adminLogins {
		if h.adminLogins[i].IP == ip {
			h.adminLogins[i].LastActive = now
			// Move to front (most recent)
			login := h.adminLogins[i]
			copy(h.adminLogins[1:i+1], h.adminLogins[0:i])
			h.adminLogins[0] = login
			return
		}
	}

	// Add new IP at the front
	newLogin := AdminLogin{
		IP:         ip,
		LastActive: now,
	}

	// Do reverse DNS lookup for new IP (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resolver := net.Resolver{}
	names, err := resolver.LookupAddr(ctx, ip)
	if err == nil && len(names) > 0 {
		newLogin.Hostname = names[0]
	}

	h.adminLogins = append([]AdminLogin{newLogin}, h.adminLogins...)

	// Keep only the last 10
	if len(h.adminLogins) > 10 {
		h.adminLogins = h.adminLogins[:10]
	}
}

func (h *HTTP) Run() {
	slog.Info("starting HTTP server", "host", h.config.HttpHost, "port", h.config.HttpPort)

	// Add gzip compression, but exclude /archive endpoints (they're already compressed)
	compressedRouter := handlers.CompressHandler(h.router)
	var handler http.Handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/archive/") {
			// Skip compression for archive endpoints
			h.router.ServeHTTP(w, r)
		} else {
			// Apply compression for all other endpoints
			compressedRouter.ServeHTTP(w, r)
		}
	})

	// Add access logging
	accessLog, err := os.OpenFile(h.config.HttpAccessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("unable to open log file", "path", h.config.HttpAccessLog, "error", err)
		os.Exit(2)
	}
	defer func() { _ = accessLog.Close() }()
	handler = handlers.CombinedLoggingHandler(accessLog, handler)

	// Add proxy header handling
	if h.config.HttpProxyHeaders {
		next := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := r.Header.Get("X-Real-IP"); ip != "" {
				if net.ParseIP(ip) != nil {
					r.RemoteAddr = ip
				}
			}
			next.ServeHTTP(w, r)
		})
	}

	slog.Info("HTTP server timeouts configured",
		"read_timeout_seconds", h.config.ReadTimeout.Seconds(),
		"read_header_timeout_seconds", h.config.ReadHeaderTimeout.Seconds(),
		"idle_timeout_seconds", h.config.IdleTimeout.Seconds(),
		"write_timeout_seconds", h.config.WriteTimeout.Seconds())

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
		slog.Error("failed to start HTTP server", "error", err)
		os.Exit(2)
	}
}

func (h *HTTP) Error(w http.ResponseWriter, r *http.Request, internal string, external string, errno int, statusCode int) {
	w.Header().Set("Cache-Control", "max-age=1")
	w.Header().Set("X-Robots-Tag", "noindex")

	if internal != "" {
		slog.Warn("request error", "errno", errno, "message", internal)
	}

	// Disregard any request body there is
	_, _ = io.Copy(io.Discard, r.Body)

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
		var buf bytes.Buffer
		if err := h.templates.ExecuteTemplate(&buf, "error_page", data); err != nil {
			slog.Error("failed to execute template", "template", "error_page", "error", err)
			// Don't call http.Error here since WriteHeader was already called
			return
		}
		_, _ = buf.WriteTo(w)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, external)
	}
}

// renderTemplate executes a template to a buffer first, then writes to the response.
// This prevents "superfluous response.WriteHeader" errors when template execution fails.
func (h *HTTP) renderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	_, _ = buf.WriteTo(w)
	return nil
}

// Parse all templates
func (h *HTTP) ParseTemplates() *template.Template {

	// Functions that are available from within templates
	var fns = template.FuncMap{
		"isAvailable": func(bin ds.Bin) bool {
			return bin.IsReadable()
		},
		"isApproved": func(bin ds.Bin) bool {
			return bin.IsApproved()
		},
		"elapsed": func(t0, t1 time.Time) string {
			elapsed := t1.Sub(t0)
			return elapsed.String()
		},
		"finished": func(t sql.NullTime) bool {
			return t.Valid && !t.Time.IsZero()
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
			return client.IsBanned()
		},
	}

	templ := template.New("").Funcs(fns)
	_, err := templ.ParseFS(h.templateBox, "templates/*.html")
	if err != nil {
		slog.Error("unable to read templates directory", "error", err)
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
	// Try to parse the addr directly
	ip := net.ParseIP(addr)
	if ip != nil {
		return ip.String(), nil
	}

	// If parsing directly failed, try to split host from port
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("unable to parse IP from %q: %w", addr, err)
	}

	ip = net.ParseIP(host)
	if ip == nil {
		return "", fmt.Errorf("unable to parse IP from host %q", host)
	}

	return ip.String(), nil
}

func setRobotsPermissions(w http.ResponseWriter, allow bool) {
	if allow {
		w.Header().Set("X-Robots-Tag", "index, follow, noarchive")
	} else {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	}
}
