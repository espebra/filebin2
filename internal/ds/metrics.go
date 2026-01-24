package ds

import (
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
