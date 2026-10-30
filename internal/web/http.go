package web

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
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

	"github.com/espebra/filebin2/internal/audit"
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
	UserAgent          string
	Requests           uint64
	LastActive         time.Time
	LastActiveReadable string
}

type HTTP struct {
	// router serves the public endpoints, none of which require
	// authentication. adminRouter serves the admin pages, the profiling
	// endpoints and the metrics, and is served on a separate listener that
	// is not meant to be exposed publicly.
	router           *mux.Router
	adminRouter      *mux.Router
	templateBox      *embed.FS
	staticBox        *embed.FS
	templates        *template.Template
	dao              *dbl.DAO
	s3               *s3.S3AO
	geodb            *geoip.DAO
	audit            *audit.Logger
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

	// Bounded in-memory ring buffers of recent client-reported upload
	// telemetry. Failures and successes are kept separately so failures
	// can be retained longer while high-volume successes rotate out
	// faster.
	clientUploadFailures       []ds.ClientUploadFailure
	clientUploadFailuresMutex  sync.Mutex
	clientUploadSuccesses      []ds.ClientUploadSuccess
	clientUploadSuccessesMutex sync.Mutex

	// Stop channel for graceful shutdown of background goroutines
	stopChan chan struct{}

	// The running HTTP servers, used by Shutdown for graceful shutdown
	httpServers      []*http.Server
	httpServersMutex sync.Mutex
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

// SetAuditLogger sets the audit log to write transfers and other events of
// bins and files to. Events are not written if it is not set.
func (h *HTTP) SetAuditLogger(a *audit.Logger) {
	h.audit = a
}

// getCachedStorageBytes returns the cached storage bytes value
func (h *HTTP) getCachedStorageBytes() uint64 {
	h.storageBytesMutex.RLock()
	defer h.storageBytesMutex.RUnlock()
	return h.storageBytesCache
}

// refreshDatabaseMetrics queries the database-sourced metrics, then updates
// the storage bytes cache and the Prometheus gauges. The queries scan large
// tables, so they run on an interval in the background instead of on every
// /metrics scrape.
func (h *HTTP) refreshDatabaseMetrics() {
	if err := h.dao.Metrics().UpdateMetrics(h.metrics); err != nil {
		slog.Error("unable to update metrics", "error", err)
		return
	}
	h.storageBytesMutex.Lock()
	h.storageBytesCache = uint64(h.metrics.CurrentBytes)
	h.storageBytesMutex.Unlock()
	h.metrics.UpdateGauges()
}

// startMetricsUpdater starts a background goroutine that refreshes the
// database-sourced metrics every minute
func (h *HTTP) startMetricsUpdater() {
	// Initialize stop channel
	h.stopChan = make(chan struct{})

	// Initial update
	h.refreshDatabaseMetrics()

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.refreshDatabaseMetrics()
			case <-h.stopChan:
				return
			}
		}
	}()
}

// recordClientUploadFailure appends a terminal failure event to the
// in-memory ring buffer, evicting the oldest entry when the buffer is
// full. A non-positive ClientUploadFailuresCap disables in-memory
// retention entirely.
func (h *HTTP) recordClientUploadFailure(ev ds.ClientUploadFailure) {
	cap := h.config.ClientUploadFailuresCap
	if cap <= 0 {
		return
	}

	h.clientUploadFailuresMutex.Lock()
	defer h.clientUploadFailuresMutex.Unlock()

	if len(h.clientUploadFailures) >= cap {
		copy(h.clientUploadFailures, h.clientUploadFailures[1:])
		h.clientUploadFailures = h.clientUploadFailures[:cap-1]
	}
	h.clientUploadFailures = append(h.clientUploadFailures, ev)
}

// recentClientUploadFailures returns a copy of the failure ring buffer
// in newest-first order.
func (h *HTTP) recentClientUploadFailures() []ds.ClientUploadFailure {
	h.clientUploadFailuresMutex.Lock()
	defer h.clientUploadFailuresMutex.Unlock()

	n := len(h.clientUploadFailures)
	out := make([]ds.ClientUploadFailure, n)
	for i, ev := range h.clientUploadFailures {
		out[n-1-i] = ev
	}
	return out
}

// recordClientUploadSuccess appends a successful upload event to the
// in-memory ring buffer. A non-positive ClientUploadSuccessesCap
// disables in-memory retention entirely.
func (h *HTTP) recordClientUploadSuccess(ev ds.ClientUploadSuccess) {
	cap := h.config.ClientUploadSuccessesCap
	if cap <= 0 {
		return
	}

	h.clientUploadSuccessesMutex.Lock()
	defer h.clientUploadSuccessesMutex.Unlock()

	if len(h.clientUploadSuccesses) >= cap {
		copy(h.clientUploadSuccesses, h.clientUploadSuccesses[1:])
		h.clientUploadSuccesses = h.clientUploadSuccesses[:cap-1]
	}
	h.clientUploadSuccesses = append(h.clientUploadSuccesses, ev)
}

// recentClientUploadSuccesses returns a copy of the success ring buffer
// in newest-first order.
func (h *HTTP) recentClientUploadSuccesses() []ds.ClientUploadSuccess {
	h.clientUploadSuccessesMutex.Lock()
	defer h.clientUploadSuccessesMutex.Unlock()

	n := len(h.clientUploadSuccesses)
	out := make([]ds.ClientUploadSuccess, n)
	for i, ev := range h.clientUploadSuccesses {
		out[n-1-i] = ev
	}
	return out
}

// Stop gracefully shuts down background goroutines
func (h *HTTP) Stop() {
	if h.stopChan != nil {
		close(h.stopChan)
	}
}

func (h *HTTP) Init() error {
	h.router = mux.NewRouter()
	h.adminRouter = mux.NewRouter()
	h.templates = h.ParseTemplates()

	h.initAdminRoutes()
	h.initPublicRoutes()

	h.config.ExpirationDuration = time.Second * time.Duration(h.config.Expiration)

	// Start background updater for the database-sourced metrics
	h.startMetricsUpdater()

	return nil
}

// initAdminRoutes registers the routes of the admin listener. Nothing
// registered here is reachable from the public listener.
func (h *HTTP) initAdminRoutes() {
	h.adminRouter.HandleFunc("/debug/pprof/cmdline", h.auth(pprof.Cmdline)).Methods(http.MethodGet)
	h.adminRouter.HandleFunc("/debug/pprof/profile", h.auth(pprof.Profile)).Methods(http.MethodGet)
	h.adminRouter.HandleFunc("/debug/pprof/symbol", h.auth(pprof.Symbol)).Methods(http.MethodGet)
	h.adminRouter.HandleFunc("/debug/pprof/trace", h.auth(pprof.Trace)).Methods(http.MethodGet)
	h.adminRouter.PathPrefix("/debug/pprof/").HandlerFunc(h.auth(pprof.Index))
	h.adminRouter.HandleFunc("/metrics", h.viewMetrics).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/log/bin/{bin:[A-Za-z0-9_-]+}", h.auth(h.viewAdminLogBin)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/log/ip/{ip:[A-Za-z0-9.:_-]+}", h.auth(h.viewAdminLogIP)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/ip/{ip:[A-Za-z0-9.:_-]+}", h.auth(h.viewAdminIP)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/ip/{ip:[A-Za-z0-9.:_-]+}/ban", h.log(h.auth(h.banIP))).Methods("POST")
	h.adminRouter.HandleFunc("/admin/ip/{ip:[A-Za-z0-9.:_-]+}/unban", h.log(h.auth(h.unbanIP))).Methods("POST")
	// Each top list has its own page. The sort keys are part of the route
	// pattern, so they cannot clash with /admin/bins/all and /admin/clients/all.
	for _, section := range h.adminTopListSections() {
		h.adminRouter.HandleFunc(section.path, h.auth(h.redirectAdminTopList(section))).Methods(http.MethodHead, http.MethodGet)
		h.adminRouter.HandleFunc(section.path+"/{sort:"+section.sortPattern()+"}", h.auth(h.viewAdminTopList(section))).Methods(http.MethodHead, http.MethodGet)
	}
	h.adminRouter.HandleFunc("/admin/bins/all", h.auth(h.viewAdminBinsAll)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/clients/all", h.auth(h.viewAdminClientsAll)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/bin/{bin:[A-Za-z0-9_-]+}", h.auth(h.viewAdminBin)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/bin/{bin:[A-Za-z0-9_-]+}/revive", h.log(h.auth(h.reviveBin))).Methods("POST")
	h.adminRouter.HandleFunc("/admin/bin/{bin:[A-Za-z0-9_-]+}/ban-uploaders", h.log(h.auth(h.banBinUploaders))).Methods("POST")
	h.adminRouter.HandleFunc("/admin/bin/{bin:[A-Za-z0-9_-]+}/ban-downloaders", h.log(h.auth(h.banBinDownloaders))).Methods("POST")
	h.adminRouter.HandleFunc("/admin/file/{sha256:[0-9a-z]+}", h.auth(h.viewAdminFile)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/file/{sha256:[0-9a-z]+}/block", h.log(h.auth(h.blockFileContent))).Methods("POST")
	h.adminRouter.HandleFunc("/admin/file/{sha256:[0-9a-z]+}/unblock", h.log(h.auth(h.unblockFileContent))).Methods("POST")
	h.adminRouter.HandleFunc("/admin/file/{sha256:[0-9a-z]+}/delete", h.log(h.auth(h.deleteFileContent))).Methods("POST")
	h.adminRouter.HandleFunc("/admin/recent/uploads.txt", h.auth(h.viewAdminRecentUploadsText)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/recent/uploads", h.auth(h.viewAdminRecentUploads)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/telemetry/upload-failures", h.auth(h.viewAdminClientUploadFailures)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/telemetry/upload-successes", h.auth(h.viewAdminClientUploadSuccesses)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/search", h.auth(h.redirectAdminSearch)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/search/{kind:bins|files}", h.auth(h.viewAdminSearch)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/message", h.auth(h.viewAdminSiteMessage)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/message", h.log(h.auth(h.updateSiteMessage))).Methods("POST")
	h.adminRouter.HandleFunc("/admin", h.auth(h.viewAdminDashboard)).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/admin/approve/{bin:[A-Za-z0-9_-]+}", h.log(h.auth(h.approveBin))).Methods("PUT")
	h.adminRouter.HandleFunc("/filebin-status", h.filebinStatus).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.HandleFunc("/storage-status", h.storageStatus).Methods(http.MethodHead, http.MethodGet)
	h.adminRouter.Handle("/static/{path:.*}", CacheControl(http.FileServer(http.FS(h.staticBox)))).Methods(http.MethodHead, http.MethodGet)
}

// initPublicRoutes registers the routes of the public listener. None of
// them require authentication, so the public listener is safe to expose.
func (h *HTTP) initPublicRoutes() {
	// Routes are matched in registration order and the first match wins, so
	// the fixed paths must be registered before the /{bin} and
	// /{bin}/{filename} catch-alls below. The fixed first path segments
	// (archive, md5, qr, sha1, sha256, static, and so on) can never
	// clash with a bin name, since bin ids must be at least 8 characters
	// long (see BinDao.ValidateInput) and every reserved segment is shorter.
	// Keep it that way when adding routes or changing the minimum length.
	h.router.HandleFunc("/", h.index).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/", h.log(h.clientLookup(h.handle(h.uploadFile)))).Methods(http.MethodPost)
	h.router.HandleFunc("/filebin-status", h.filebinStatus).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/storage-status", h.storageStatus).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/robots.txt", h.robots).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/about", h.about).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api", h.api).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/api.yaml", h.apiSpec).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/privacy", h.privacy).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/contact", h.contact).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/terms", h.terms).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/integration/slack", h.integrationSlack).Methods(http.MethodPost)
	h.router.HandleFunc("/api/telemetry/failure", h.telemetryFailure).Methods(http.MethodPost)
	h.router.HandleFunc("/api/telemetry/success", h.telemetrySuccess).Methods(http.MethodPost)
	h.router.Handle("/static/{path:.*}", CacheControl(http.FileServer(http.FS(h.staticBox)))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/archive/{bin:[A-Za-z0-9_-]+}/{format:[a-z]+}", h.log(h.clientLookup(h.archive))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/sha256/{bin:[A-Za-z0-9_-]+}", h.handle(h.viewBinChecksums(func(f ds.File) string { return f.SHA256 }))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/sha1/{bin:[A-Za-z0-9_-]+}", h.handle(h.viewBinChecksums(func(f ds.File) string { return f.SHA1 }))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/md5/{bin:[A-Za-z0-9_-]+}", h.handle(h.viewBinChecksums(func(f ds.File) string { return md5Hex(f.MD5) }))).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/qr/{bin:[A-Za-z0-9_-]+}", h.binQR).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}.txt", h.viewBinPlainText).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/", h.viewBinRedirect).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.viewBin).Methods(http.MethodHead, http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.clientLookup(h.deleteBin))).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.clientLookup(h.lockBin))).Methods("PUT")
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", h.log(h.clientLookup(h.banBin))).Methods("BAN")
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.clientLookup(h.getFile))).Methods(http.MethodGet)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.clientLookup(h.deleteFile))).Methods(http.MethodDelete)
	h.router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", h.log(h.clientLookup(h.handle(h.uploadFile)))).Methods(http.MethodPost, http.MethodPut)
}

func CacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		h.ServeHTTP(w, r)
	})
}

func (h *HTTP) clientLookup(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := extractIP(r.RemoteAddr)
		if err != nil {
			h.Error(w, r, fmt.Sprintf("Failed to parse remote addr %s: %s", r.RemoteAddr, err.Error()), "Unable to identify the client", 991, http.StatusInternalServerError)
			return
		}
		client := ds.Client{IP: ip}
		if err := h.geodb.Lookup(r.RemoteAddr, &client); err != nil {
			slog.Debug("unable to look up geoip details", "remote_addr", r.RemoteAddr, "error", err)
		}

		// Check for a ban and count the request in one query.
		banned, err := h.dao.Client().RecordActivity(&client)
		if err != nil {
			h.Error(w, r, fmt.Sprintf("Failed to record activity of client %s: %s", ip, err.Error()), "Database error", 991, http.StatusInternalServerError)
			return
		}
		if banned {
			// Look up the ban details for the log. Bans are rare, so the
			// extra query does not matter.
			if client, _, err := h.dao.Client().GetByRemoteAddr(r.RemoteAddr); err == nil {
				slog.Warn("rejecting request from banned client", "ip", client.IP, "banned_at", client.BannedAt.Time.Format("2006-01-02 15:04:05 UTC"), "banned_by", client.BannedBy)
			}
			http.Error(w, "This client IP address has been banned.", http.StatusForbidden)
			return
		}

		fn(w, r)
	})
}

type transactionChecksumsKey struct{}

type requestIDKey struct{}

// proxyHeadersKey marks requests received on a listener that trusts the
// proxy headers, so that handlers shared by the listeners can tell them
// apart.
type proxyHeadersKey struct{}

// requestIDHeader carries the request id. Varnish sets it on requests to the
// backend and the upload client reads it from the response.
const requestIDHeader = "X-Request-Id"

// newRequestID returns the id of the request. The id set by the proxy is
// used if the listener trusts proxy headers and the id is well formed, so
// that the proxy and filebin log the same id. Otherwise a random id is
// generated.
func (h *HTTP) newRequestID(r *http.Request) string {
	if proxyHeadersTrusted(r) {
		if id := r.Header.Get(requestIDHeader); validRequestID(id) {
			return id
		}
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// proxyHeadersTrusted reports whether the request was received on a
// listener that trusts the proxy headers.
func proxyHeadersTrusted(r *http.Request) bool {
	trusted, _ := r.Context().Value(proxyHeadersKey{}).(bool)
	return trusted
}

// validRequestID accepts ids of up to 128 characters from a limited
// character set, so that they are safe to log and to put in URLs.
func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == ':':
		default:
			return false
		}
	}
	return true
}

// requestID returns the id of the request as set by log(), or an empty
// string for requests that are not wrapped by log().
func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	return id
}

// setTransactionChecksums records the checksums of the file content
// transferred by the request, to be written to the audit log by log().
// It is a no-op for requests that are not wrapped by log().
func setTransactionChecksums(r *http.Request, md5, sha1, sha256, phash string) {
	if c, ok := r.Context().Value(transactionChecksumsKey{}).(*ds.TransactionChecksums); ok {
		c.MD5 = md5
		c.SHA1 = sha1
		c.SHA256 = sha256
		c.PHash = phash
	}
}

// setTransactionBin records the bin of an upload, to be written to the
// audit log and the transaction table by log() when the URL has no bin, as
// for legacy uploads to /. It is a no-op for requests that are not wrapped
// by log().
func setTransactionBin(r *http.Request, bin string) {
	if c, ok := r.Context().Value(transactionChecksumsKey{}).(*ds.TransactionChecksums); ok {
		c.Bin = bin
	}
}

// setTransactionFilename records the name the file is stored as, to be
// written to the audit log by log() in place of the name in the request,
// which may have been sanitized. It is a no-op for requests that are not
// wrapped by log().
func setTransactionFilename(r *http.Request, filename string) {
	if c, ok := r.Context().Value(transactionChecksumsKey{}).(*ds.TransactionChecksums); ok {
		c.Filename = filename
	}
}

// setTransactionContentType records the content type detected from an
// uploaded file, to be written to the audit log by log() in place of the
// content type sent by the client. It is a no-op for requests that are not
// wrapped by log().
func setTransactionContentType(r *http.Request, contentType string) {
	if c, ok := r.Context().Value(transactionChecksumsKey{}).(*ds.TransactionChecksums); ok {
		c.ContentType = contentType
	}
}

func (h *HTTP) log(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()

		params := mux.Vars(r)
		bin := params["bin"]
		filename := params["filename"]

		// Handlers that transfer a file record its checksums here to have
		// them included in the audit log.
		checksums := &ds.TransactionChecksums{}
		ctx := context.WithValue(r.Context(), transactionChecksumsKey{}, checksums)

		reqID := h.newRequestID(r)
		ctx = context.WithValue(ctx, requestIDKey{}, reqID)
		w.Header().Set(requestIDHeader, reqID)

		r = r.WithContext(ctx)

		// Derive a handler name from the matched route template
		handler := "unknown"
		if route := mux.CurrentRoute(r); route != nil {
			if tmpl, err := route.GetPathTemplate(); err == nil {
				handler = tmpl
			}
		}

		// The metrics are updated as the response is written, so they are
		// available if the handler aborts the response by panicking.
		metrics := httpsnoop.Metrics{Code: http.StatusOK}

		// A handler aborts a response it can no longer complete, such as an
		// archive download that fails midway, with a panic. Write it to the
		// audit log as aborted and continue the panic, so that the server
		// still resets the connection. The transaction table has no way to
		// mark a request as aborted, so it is left out of it as before.
		defer func() {
			if rec := recover(); rec != nil {
				h.logAuditEvent(r, handler, t0, time.Now(), metrics.Code, metrics.Written, reqID, *checksums, audit.OutcomeAborted)
				panic(rec)
			}
		}()

		metrics.CaptureMetrics(w, func(w http.ResponseWriter) {
			fn(w, r)
		})

		completed := time.Now()
		duration := completed.Sub(t0)
		h.metrics.ObserveHTTPRequest(r.Method, handler, duration, metrics.Code)

		h.logAuditEvent(r, handler, t0, completed, metrics.Code, metrics.Written, reqID, *checksums, audit.OutcomeComplete)

		// Only register transactions with a response status lower than 400, which
		// includes file uploads, archive downloads and presigned downloads from S3.
		// However, any client or server side errors are skipped to reduce the database
		// volume needed.
		if metrics.Code < 400 {
			// Legacy uploads to / have the bin and filename in request
			// headers instead of the URL.
			if bin == "" {
				bin = checksums.Bin
			}
			if filename == "" {
				filename = checksums.Filename
			}
			_, err := h.dao.Transaction().Register(r, bin, filename, t0, completed, metrics.Code, metrics.Written)
			if err != nil {
				slog.Error("unable to register transaction", "error", err)
			}
		}
	})
}

// logAuditEvent writes the request to the audit log. Requests for bins and
// files that do not exist are left out, as they are mostly scanners and
// stale links. Other client errors are kept, as attempts that were rejected,
// for instance by a ban or a block, can be relevant.
func (h *HTTP) logAuditEvent(r *http.Request, routeTemplate string, t0 time.Time, completed time.Time, status int, written int64, reqID string, checksums ds.TransactionChecksums, outcome string) {
	if h.audit == nil || status == http.StatusNotFound {
		return
	}
	params := mux.Vars(r)
	e := audit.Event{
		Time:            t0,
		DurationSeconds: completed.Sub(t0).Seconds(),
		Event:           auditEventName(r.Method, routeTemplate),
		Actor:           audit.ActorClient,
		RequestID:       reqID,
		Method:          r.Method,
		Host:            auditValue(r.Host),
		Path:            r.URL.Path,
		Status:          status,
		Outcome:         outcome,
		RespBytes:       written,
		Bin:             params["bin"],
		Filename:        params["filename"],
		MD5:             md5Hex(checksums.MD5),
		SHA1:            checksums.SHA1,
		SHA256:          checksums.SHA256,
		PHash:           checksums.PHash,
		IP:              r.RemoteAddr,
		UserAgent:       auditHeader(r, "User-Agent"),
		Referer:         auditHeader(r, "Referer"),
		XForwardedFor:   auditHeader(r, "X-Forwarded-For"),
		Forwarded:       auditHeader(r, "Forwarded"),
		Via:             auditHeader(r, "Via"),
		AcceptLanguage:  auditHeader(r, "Accept-Language"),
		SecCHUA:         auditHeader(r, "Sec-CH-UA"),
		SecCHUAPlatform: auditHeader(r, "Sec-CH-UA-Platform"),
		SecCHUAMobile:   auditHeader(r, "Sec-CH-UA-Mobile"),
		Origin:          auditHeader(r, "Origin"),
		SecFetchSite:    auditHeader(r, "Sec-Fetch-Site"),
		ContentType:     auditHeader(r, "Content-Type"),
	}
	if strings.HasPrefix(r.URL.Path, "/admin/") {
		e.Actor = audit.ActorAdmin
	}
	if checksums.Bin != "" {
		e.Bin = checksums.Bin
	}
	if checksums.Filename != "" {
		e.Filename = checksums.Filename
	}
	if checksums.ContentType != "" {
		e.ContentType = checksums.ContentType
	}
	if e.SHA256 == "" {
		// Admin actions on content are addressed by checksum
		e.SHA256 = params["sha256"]
	}
	if r.ContentLength > 0 {
		e.ReqBytes = r.ContentLength
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		e.IP = host
	}
	if h.geodb != nil {
		var client ds.Client
		if err := h.geodb.Lookup(e.IP, &client); err == nil {
			e.ASN = client.ASN
			e.ASNOrganization = client.ASNOrganization
			e.Network = client.Network
			e.Country = client.Country
			e.City = client.City
			e.Continent = client.Continent
		}
	}
	h.audit.Log(e)
}

// auditHeaderMaxBytes limits the size of each header value in the audit log,
// as clients can send arbitrarily large headers.
const auditHeaderMaxBytes = 512

// auditHeader returns the values of a request header for the audit log,
// joined if the header is repeated and truncated to auditHeaderMaxBytes.
func auditHeader(r *http.Request, name string) string {
	return auditValue(strings.Join(r.Header.Values(name), ", "))
}

// auditValue truncates a client supplied value to auditHeaderMaxBytes.
func auditValue(v string) string {
	if len(v) > auditHeaderMaxBytes {
		v = strings.ToValidUTF8(v[:auditHeaderMaxBytes], "")
	}
	return v
}

// auditEventName names the event of a request in the audit log by the route
// it matched.
func auditEventName(method string, routeTemplate string) string {
	switch {
	case routeTemplate == "/" && method == http.MethodPost:
		// Legacy upload with the bin and filename in request headers
		return "file_upload"
	case strings.HasPrefix(routeTemplate, "/archive/"):
		return "archive_download"
	case strings.HasPrefix(routeTemplate, "/admin/"):
		// Name admin actions by the fixed segments of the route, for
		// instance admin_bin_revive for /admin/bin/{bin}/revive
		var parts []string
		for _, segment := range strings.Split(routeTemplate, "/") {
			if segment == "" || strings.HasPrefix(segment, "{") {
				continue
			}
			parts = append(parts, strings.ReplaceAll(segment, "-", "_"))
		}
		return strings.Join(parts, "_")
	case strings.HasPrefix(routeTemplate, "/{bin") && strings.Contains(routeTemplate, "{filename"):
		switch method {
		case http.MethodGet, http.MethodHead:
			// The client is redirected to a presigned URL and fetches
			// the file from S3, so filebin only sees the request to
			// download, not the transfer itself.
			return "file_download_requested"
		case http.MethodPost, http.MethodPut:
			return "file_upload"
		case http.MethodDelete:
			return "file_delete"
		}
	case strings.HasPrefix(routeTemplate, "/{bin"):
		switch method {
		case http.MethodDelete:
			return "bin_delete"
		case http.MethodPut:
			return "bin_lock"
		case "BAN":
			return "bin_ban"
		}
	}
	return "http_request"
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

		h.trackAdminLogin(r.RemoteAddr, r.UserAgent())
		fn(w, r)
	}
}

func (h *HTTP) trackAdminLogin(remoteAddr string, userAgent string) {
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
			h.adminLogins[i].UserAgent = userAgent
			h.adminLogins[i].Requests++
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
		UserAgent:  userAgent,
		Requests:   1,
		LastActive: now,
	}

	h.adminLogins = append([]AdminLogin{newLogin}, h.adminLogins...)

	// Keep only the last 10
	if len(h.adminLogins) > 10 {
		h.adminLogins = h.adminLogins[:10]
	}

	// Resolve the hostname asynchronously to avoid stalling admin requests
	// behind the mutex while the reverse DNS lookup is in flight.
	go h.resolveAdminLoginHostname(ip)
}

func (h *HTTP) resolveAdminLoginHostname(ip string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resolver := net.Resolver{}
	names, err := resolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return
	}

	h.adminLoginsMutex.Lock()
	defer h.adminLoginsMutex.Unlock()
	for i := range h.adminLogins {
		if h.adminLogins[i].IP == ip {
			h.adminLogins[i].Hostname = names[0]
			return
		}
	}
}

// Run starts the public listener and, unless it is disabled, the admin
// listener. It returns as soon as a shutdown is initiated.
func (h *HTTP) Run() {
	// Open the access log if enabled. It is shared by both listeners. The
	// file is deliberately not closed when Run returns: during a graceful
	// shutdown, Run returns as soon as the shutdown is initiated while
	// draining request handlers still write to the access log. The file
	// stays open for the lifetime of the process.
	var accessLog io.Writer
	if h.config.HttpAccessLog != "" {
		f, err := os.OpenFile(h.config.HttpAccessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Error("unable to open log file", "path", h.config.HttpAccessLog, "error", err)
			os.Exit(2)
		}
		accessLog = f
		slog.Info("access log enabled", "path", h.config.HttpAccessLog)
	} else {
		slog.Info("access log disabled")
	}

	slog.Info("HTTP server timeouts configured",
		"read_timeout_seconds", h.config.ReadTimeout.Seconds(),
		"read_header_timeout_seconds", h.config.ReadHeaderTimeout.Seconds(),
		"idle_timeout_seconds", h.config.IdleTimeout.Seconds(),
		"write_timeout_seconds", h.config.WriteTimeout.Seconds())

	// The proxy headers are trusted per listener, since the public and the
	// admin listeners do not necessarily run behind the same proxy, or
	// behind a proxy at all. Trusting them on a listener that is reachable
	// without a proxy would let the clients spoof their IP address.
	servers := []*http.Server{
		h.newServer("public", h.config.HttpHost, h.config.HttpPort, h.router, accessLog, h.config.HttpProxyHeaders),
	}
	if h.config.AdminHttpPort > 0 {
		servers = append(servers, h.newServer("admin", h.config.AdminHttpHost, h.config.AdminHttpPort, h.adminRouter, accessLog, h.config.AdminHttpProxyHeaders))
	} else {
		slog.Info("admin HTTP listener disabled")
	}
	h.httpServersMutex.Lock()
	h.httpServers = servers
	h.httpServersMutex.Unlock()

	// Start the servers. ListenAndServe blocks until Shutdown is called (in
	// which case it returns ErrServerClosed) or the server fails.
	var wg sync.WaitGroup
	for _, srv := range servers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("failed to start HTTP server", "addr", srv.Addr, "error", err)
				os.Exit(2)
			}
		}()
	}
	wg.Wait()
}

// newServer sets up an HTTP server for one of the listeners
func (h *HTTP) newServer(name string, host string, port int, router *mux.Router, accessLog io.Writer, proxyHeaders bool) *http.Server {
	slog.Info("starting HTTP server", "listener", name, "host", host, "port", port)

	// Add gzip compression, but exclude /archive endpoints (they're already
	// compressed). The responses are small HTML and JSON documents, for
	// which the fastest compression level costs a fraction of the CPU time
	// of the default level for a slightly larger response.
	compressedRouter := handlers.CompressHandlerLevel(router, gzip.BestSpeed)
	var handler http.Handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/archive/") {
			// Skip compression for archive endpoints
			router.ServeHTTP(w, r)
		} else {
			// Apply compression for all other endpoints
			compressedRouter.ServeHTTP(w, r)
		}
	})

	// Add access logging if enabled
	if accessLog != nil {
		handler = handlers.CombinedLoggingHandler(accessLog, handler)
	}

	// Add proxy header handling
	if proxyHeaders {
		slog.Info("trusting proxy headers", "listener", name)
		next := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := r.Header.Get("X-Real-IP"); ip != "" {
				if net.ParseIP(ip) != nil {
					r.RemoteAddr = ip
				}
			}
			r = r.WithContext(context.WithValue(r.Context(), proxyHeadersKey{}, true))
			next.ServeHTTP(w, r)
		})
	}

	return &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, port),
		Handler:           handler,
		ReadTimeout:       h.config.ReadTimeout,
		WriteTimeout:      h.config.WriteTimeout,
		IdleTimeout:       h.config.IdleTimeout,
		ReadHeaderTimeout: h.config.ReadHeaderTimeout,
	}
}

// Shutdown gracefully shuts down the HTTP servers: they stop accepting new
// connections and wait for in-flight requests to complete until the context
// is cancelled, at which point the remaining connections are closed. Draining
// in-flight uploads matters in particular, as killing an upload between its
// S3 write and its database insert would leak the S3 object.
func (h *HTTP) Shutdown(ctx context.Context) error {
	h.httpServersMutex.Lock()
	servers := h.httpServers
	h.httpServersMutex.Unlock()

	errs := make([]error, len(servers))
	var wg sync.WaitGroup
	for i, srv := range servers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = srv.Shutdown(ctx)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// Error reports a request failure to the client. It is the pre-handlerFunc
// way of reporting errors and renders through respondError, so both styles
// produce the same response and log line. New code should return an
// httpError instead. The internal message and errno only end up in the log.
func (h *HTTP) Error(w http.ResponseWriter, r *http.Request, internal string, external string, errno int, statusCode int) {
	e := &httpError{status: statusCode, message: external}
	if internal != "" {
		e.err = fmt.Errorf("errno %d: %s", errno, internal)
	}
	h.respondError(w, r, e)
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

// publicURL builds the URL of a page on the public listener from the base
// URL. The admin pages use it to link to bins and files, as they are served
// from a separate listener.
func (h *HTTP) publicURL(elem ...string) string {
	u := h.config.BaseUrl
	u.Path = path.Join(append([]string{"/", u.Path}, elem...)...)
	return u.String()
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
		"isSet": func(t sql.NullTime) bool {
			return t.Valid && !t.Time.IsZero()
		},
		"durationInSeconds": func(dur time.Duration) string {
			return fmt.Sprintf("%.3f", dur.Seconds())
		},
		"join": func(s ...string) string {
			return path.Join(s...)
		},
		"publicURL": h.publicURL,
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
