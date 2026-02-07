package ds

import (
	"database/sql"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	Id string `json:"-"`

	// Database-sourced metrics (populated by UpdateMetrics)
	CurrentLogEntries    int64  `json:"current_log_entries"`
	LimitBytes           uint64 `json:"limit_bytes"`
	CurrentBytes         int64  `json:"current_bytes"`
	CurrentBytesReadable string `json:"current_bytes_readable"`
	CurrentFiles         int64  `json:"current_files"`
	CurrentFilesReadable string `json:"current_files_readable"`
	CurrentBins          int64  `json:"current_bins"`
	CurrentBinsReadable  string `json:"current_bins_readable"`
	FreeBytes            int64  `json:"-"`
	FreeBytesReadable    string `json:"-"`
	TotalBytes           int64  `json:"total_bytes"`
	TotalBytesReadable   string `json:"total_bytes_readable"`
	TotalFiles           int64  `json:"total_files"`
	TotalFilesReadable   string `json:"total_files_readable"`
	TotalBins            int64  `json:"total_bins"`
	TotalBinsReadable    string `json:"total_bins_readable"`

	// Prometheus metrics
	dataTransferBytes    *prometheus.CounterVec
	fileOperations       *prometheus.CounterVec
	binOperations        *prometheus.CounterVec
	archiveDownloads     *prometheus.CounterVec
	pageViews            *prometheus.CounterVec
	operationsInProgress *prometheus.GaugeVec
	transactions         prometheus.Gauge
	storageBytes         *prometheus.GaugeVec
	files                prometheus.Gauge
	bins                 prometheus.Gauge

	// HTTP metrics
	httpRequestDuration *prometheus.HistogramVec
	httpResponseStatus  *prometheus.CounterVec

	// S3 metrics
	S3OperationDuration *prometheus.HistogramVec
	S3OperationErrors   *prometheus.CounterVec

	// Database query metrics
	DBQueryDuration *prometheus.HistogramVec
	DBQueryErrors   *prometheus.CounterVec

	// Database connection pool metrics
	dbOpenConnections   prometheus.Gauge
	dbInUseConnections  prometheus.Gauge
	dbIdleConnections   prometheus.Gauge
	dbWaitCount         prometheus.Gauge
	dbWaitDuration      prometheus.Gauge
	dbMaxIdleClosed     prometheus.Gauge
	dbMaxIdleTimeClosed prometheus.Gauge
	dbMaxLifetimeClosed prometheus.Gauge
}

func NewMetrics(id string, registry *prometheus.Registry) *Metrics {
	m := &Metrics{
		Id: id,
	}

	factory := promauto.With(registry)

	m.dataTransferBytes = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_data_transfer_bytes",
			Help: "Approximate data transfer in bytes",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"direction"},
	)

	m.fileOperations = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_file_operations",
			Help: "Number of file operations",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"type"},
	)

	m.binOperations = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_bin_operations",
			Help: "Number of bin operations",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"type"},
	)

	m.archiveDownloads = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_archive_downloads",
			Help: "Number of archive downloads",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"format"},
	)

	m.pageViews = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_page_views",
			Help: "Number of page views",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"page"},
	)

	m.operationsInProgress = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "filebin_operations_in_progress",
			Help: "Number of file uploads from clients to filebin currently in progress",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"type"},
	)

	m.transactions = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "filebin_transactions",
			Help: "Number of transactions logged in the database",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
	)

	m.storageBytes = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "filebin_storage_bytes",
			Help: "The number of bytes consumed by the files in storage",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"type"},
	)

	m.files = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "filebin_files",
			Help: "The number of files in storage",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
	)

	m.bins = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "filebin_bins",
			Help: "The number of bins active",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
	)

	// HTTP request duration histogram
	m.httpRequestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "filebin_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"method", "handler"},
	)

	// HTTP response status counter
	m.httpResponseStatus = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_http_responses_total",
			Help: "Total HTTP responses by method and status code",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"method", "code"},
	)

	// S3 operation duration histogram
	m.S3OperationDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "filebin_s3_operation_duration_seconds",
			Help:    "S3 operation duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"operation"},
	)

	// S3 operation error counter
	m.S3OperationErrors = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_s3_operation_errors_total",
			Help: "Total S3 operation errors",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"operation"},
	)

	// Database query duration histogram
	m.DBQueryDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "filebin_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"operation"},
	)

	// Database query error counter
	m.DBQueryErrors = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "filebin_db_query_errors_total",
			Help: "Total database query errors",
			ConstLabels: prometheus.Labels{
				"id": id,
			},
		},
		[]string{"operation"},
	)

	// Database connection pool gauges
	m.dbOpenConnections = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_open_connections",
		Help:        "Number of established database connections (in use + idle)",
		ConstLabels: prometheus.Labels{"id": id},
	})
	m.dbInUseConnections = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_in_use_connections",
		Help:        "Number of database connections currently in use",
		ConstLabels: prometheus.Labels{"id": id},
	})
	m.dbIdleConnections = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_idle_connections",
		Help:        "Number of idle database connections",
		ConstLabels: prometheus.Labels{"id": id},
	})
	m.dbWaitCount = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_wait_count_total",
		Help:        "Total number of connections waited for",
		ConstLabels: prometheus.Labels{"id": id},
	})
	m.dbWaitDuration = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_wait_duration_seconds_total",
		Help:        "Total time blocked waiting for a new connection in seconds",
		ConstLabels: prometheus.Labels{"id": id},
	})
	m.dbMaxIdleClosed = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_max_idle_closed_total",
		Help:        "Total connections closed due to SetMaxIdleConns",
		ConstLabels: prometheus.Labels{"id": id},
	})
	m.dbMaxIdleTimeClosed = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_max_idle_time_closed_total",
		Help:        "Total connections closed due to SetConnMaxIdleTime",
		ConstLabels: prometheus.Labels{"id": id},
	})
	m.dbMaxLifetimeClosed = factory.NewGauge(prometheus.GaugeOpts{
		Name:        "filebin_db_max_lifetime_closed_total",
		Help:        "Total connections closed due to SetConnMaxLifetime",
		ConstLabels: prometheus.Labels{"id": id},
	})

	return m
}

// Update database-sourced gauges
func (m *Metrics) UpdateGauges() {
	m.transactions.Set(float64(m.CurrentLogEntries))
	m.storageBytes.WithLabelValues("used").Set(float64(m.CurrentBytes))
	m.storageBytes.WithLabelValues("limit").Set(float64(m.LimitBytes))
	m.files.Set(float64(m.CurrentFiles))
	m.bins.Set(float64(m.CurrentBins))
}

// Data transfer methods
func (m *Metrics) IncrBytesFilebinToClient(value uint64) {
	m.dataTransferBytes.WithLabelValues("filebin_to_client").Add(float64(value))
}

func (m *Metrics) IncrBytesClientToFilebin(value uint64) {
	m.dataTransferBytes.WithLabelValues("client_to_filebin").Add(float64(value))
}

func (m *Metrics) IncrBytesFilebinToStorage(value uint64) {
	m.dataTransferBytes.WithLabelValues("filebin_to_storage").Add(float64(value))
}

func (m *Metrics) IncrBytesStorageToFilebin(value uint64) {
	m.dataTransferBytes.WithLabelValues("storage_to_filebin").Add(float64(value))
}

func (m *Metrics) IncrBytesStorageToClient(value uint64) {
	m.dataTransferBytes.WithLabelValues("storage_to_client").Add(float64(value))
}

// File operation methods
func (m *Metrics) IncrFileUploadCount() {
	m.fileOperations.WithLabelValues("upload").Inc()
}

func (m *Metrics) IncrFileDownloadCount() {
	m.fileOperations.WithLabelValues("download").Inc()
}

func (m *Metrics) IncrFileDeleteCount() {
	m.fileOperations.WithLabelValues("delete").Inc()
}

// Archive download methods
func (m *Metrics) IncrTarArchiveDownloadCount() {
	m.archiveDownloads.WithLabelValues("tar").Inc()
}

func (m *Metrics) IncrZipArchiveDownloadCount() {
	m.archiveDownloads.WithLabelValues("zip").Inc()
}

// Page view methods
func (m *Metrics) IncrFrontPageViewCount() {
	m.pageViews.WithLabelValues("front").Inc()
}

func (m *Metrics) IncrBinPageViewCount() {
	m.pageViews.WithLabelValues("bin").Inc()
}

func (m *Metrics) IncrErrorPageViewCount() {
	m.pageViews.WithLabelValues("error").Inc()
}

// Bin operation methods
func (m *Metrics) IncrNewBinCount() {
	m.binOperations.WithLabelValues("create").Inc()
}

func (m *Metrics) IncrBinDeleteCount() {
	m.binOperations.WithLabelValues("delete").Inc()
}

func (m *Metrics) IncrBinBanCount() {
	m.binOperations.WithLabelValues("ban").Inc()
}

func (m *Metrics) IncrBinLockCount() {
	m.binOperations.WithLabelValues("lock").Inc()
}

// In-progress operation methods
func (m *Metrics) IncrFileUploadInProgress() {
	m.operationsInProgress.WithLabelValues("file_upload").Inc()
}

func (m *Metrics) DecrFileUploadInProgress() {
	m.operationsInProgress.WithLabelValues("file_upload").Dec()
}

func (m *Metrics) IncrArchiveDownloadInProgress() {
	m.operationsInProgress.WithLabelValues("archive_download").Inc()
}

func (m *Metrics) DecrArchiveDownloadInProgress() {
	m.operationsInProgress.WithLabelValues("archive_download").Dec()
}

func (m *Metrics) IncrStorageUploadInProgress() {
	m.operationsInProgress.WithLabelValues("storage_upload").Inc()
}

func (m *Metrics) DecrStorageUploadInProgress() {
	m.operationsInProgress.WithLabelValues("storage_upload").Dec()
}

// HTTP metrics methods
func (m *Metrics) ObserveHTTPRequest(method, handler string, duration time.Duration, statusCode int) {
	m.httpRequestDuration.WithLabelValues(method, handler).Observe(duration.Seconds())
	m.httpResponseStatus.WithLabelValues(method, strconv.Itoa(statusCode)).Inc()
}

// S3 metrics methods
func (m *Metrics) ObserveS3Operation(operation string, duration time.Duration) {
	m.S3OperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

func (m *Metrics) IncrS3OperationError(operation string) {
	m.S3OperationErrors.WithLabelValues(operation).Inc()
}

// Database query metrics methods
func (m *Metrics) ObserveDBQuery(operation string, duration time.Duration) {
	m.DBQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

func (m *Metrics) IncrDBQueryError(operation string) {
	m.DBQueryErrors.WithLabelValues(operation).Inc()
}

// Database connection pool metrics
func (m *Metrics) UpdateDBStats(stats sql.DBStats) {
	m.dbOpenConnections.Set(float64(stats.OpenConnections))
	m.dbInUseConnections.Set(float64(stats.InUse))
	m.dbIdleConnections.Set(float64(stats.Idle))
	m.dbWaitCount.Set(float64(stats.WaitCount))
	m.dbWaitDuration.Set(stats.WaitDuration.Seconds())
	m.dbMaxIdleClosed.Set(float64(stats.MaxIdleClosed))
	m.dbMaxIdleTimeClosed.Set(float64(stats.MaxIdleTimeClosed))
	m.dbMaxLifetimeClosed.Set(float64(stats.MaxLifetimeClosed))
}
