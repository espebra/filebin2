package ds

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test-id", registry)

	if metrics == nil {
		t.Fatal("NewMetrics returned nil")
	}

	if metrics.Id != "test-id" {
		t.Errorf("Expected Id to be 'test-id', got '%s'", metrics.Id)
	}

	// Verify all metric fields are initialized
	if metrics.dataTransferBytes == nil {
		t.Error("dataTransferBytes not initialized")
	}
	if metrics.fileOperations == nil {
		t.Error("fileOperations not initialized")
	}
	if metrics.binOperations == nil {
		t.Error("binOperations not initialized")
	}
	if metrics.archiveDownloads == nil {
		t.Error("archiveDownloads not initialized")
	}
	if metrics.pageViews == nil {
		t.Error("pageViews not initialized")
	}
	if metrics.operationsInProgress == nil {
		t.Error("operationsInProgress not initialized")
	}
	if metrics.transactions == nil {
		t.Error("transactions not initialized")
	}
	if metrics.storageBytes == nil {
		t.Error("storageBytes not initialized")
	}
	if metrics.files == nil {
		t.Error("files not initialized")
	}
	if metrics.bins == nil {
		t.Error("bins not initialized")
	}
}

func TestDataTransferMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Test incrementing data transfer counters
	metrics.IncrBytesFilebinToClient(100)
	metrics.IncrBytesClientToFilebin(200)
	metrics.IncrBytesFilebinToStorage(300)
	metrics.IncrBytesStorageToFilebin(400)
	metrics.IncrBytesStorageToClient(500)

	expected := `
		# HELP filebin_data_transfer_bytes Approximate data transfer in bytes
		# TYPE filebin_data_transfer_bytes counter
		filebin_data_transfer_bytes{direction="client_to_filebin",id="test"} 200
		filebin_data_transfer_bytes{direction="filebin_to_client",id="test"} 100
		filebin_data_transfer_bytes{direction="filebin_to_storage",id="test"} 300
		filebin_data_transfer_bytes{direction="storage_to_client",id="test"} 500
		filebin_data_transfer_bytes{direction="storage_to_filebin",id="test"} 400
	`

	if err := testutil.CollectAndCompare(metrics.dataTransferBytes, strings.NewReader(expected)); err != nil {
		t.Errorf("Data transfer metrics mismatch: %v", err)
	}
}

func TestFileOperationMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Test incrementing file operation counters
	metrics.IncrFileUploadCount()
	metrics.IncrFileUploadCount()
	metrics.IncrFileDownloadCount()
	metrics.IncrFileDeleteCount()

	expected := `
		# HELP filebin_file_operations Number of file operations
		# TYPE filebin_file_operations counter
		filebin_file_operations{id="test",type="delete"} 1
		filebin_file_operations{id="test",type="download"} 1
		filebin_file_operations{id="test",type="upload"} 2
	`

	if err := testutil.CollectAndCompare(metrics.fileOperations, strings.NewReader(expected)); err != nil {
		t.Errorf("File operation metrics mismatch: %v", err)
	}
}

func TestBinOperationMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Test incrementing bin operation counters
	metrics.IncrNewBinCount()
	metrics.IncrBinDeleteCount()
	metrics.IncrBinBanCount()
	metrics.IncrBinLockCount()

	expected := `
		# HELP filebin_bin_operations Number of bin operations
		# TYPE filebin_bin_operations counter
		filebin_bin_operations{id="test",type="ban"} 1
		filebin_bin_operations{id="test",type="create"} 1
		filebin_bin_operations{id="test",type="delete"} 1
		filebin_bin_operations{id="test",type="lock"} 1
	`

	if err := testutil.CollectAndCompare(metrics.binOperations, strings.NewReader(expected)); err != nil {
		t.Errorf("Bin operation metrics mismatch: %v", err)
	}
}

func TestArchiveDownloadMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Test incrementing archive download counters
	metrics.IncrTarArchiveDownloadCount()
	metrics.IncrTarArchiveDownloadCount()
	metrics.IncrZipArchiveDownloadCount()

	expected := `
		# HELP filebin_archive_downloads Number of archive downloads
		# TYPE filebin_archive_downloads counter
		filebin_archive_downloads{format="tar",id="test"} 2
		filebin_archive_downloads{format="zip",id="test"} 1
	`

	if err := testutil.CollectAndCompare(metrics.archiveDownloads, strings.NewReader(expected)); err != nil {
		t.Errorf("Archive download metrics mismatch: %v", err)
	}
}

func TestPageViewMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Test incrementing page view counters
	metrics.IncrFrontPageViewCount()
	metrics.IncrFrontPageViewCount()
	metrics.IncrBinPageViewCount()
	metrics.IncrErrorPageViewCount()

	expected := `
		# HELP filebin_page_views Number of page views
		# TYPE filebin_page_views counter
		filebin_page_views{id="test",page="bin"} 1
		filebin_page_views{id="test",page="error"} 1
		filebin_page_views{id="test",page="front"} 2
	`

	if err := testutil.CollectAndCompare(metrics.pageViews, strings.NewReader(expected)); err != nil {
		t.Errorf("Page view metrics mismatch: %v", err)
	}
}

func TestOperationsInProgressMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Test incrementing and decrementing in-progress gauges
	metrics.IncrFileUploadInProgress()
	metrics.IncrFileUploadInProgress()
	metrics.DecrFileUploadInProgress()

	metrics.IncrStorageUploadInProgress()
	metrics.IncrArchiveDownloadInProgress()

	expected := `
		# HELP filebin_operations_in_progress Number of file uploads from clients to filebin currently in progress
		# TYPE filebin_operations_in_progress gauge
		filebin_operations_in_progress{id="test",type="archive_download"} 1
		filebin_operations_in_progress{id="test",type="file_upload"} 1
		filebin_operations_in_progress{id="test",type="storage_upload"} 1
	`

	if err := testutil.CollectAndCompare(metrics.operationsInProgress, strings.NewReader(expected)); err != nil {
		t.Errorf("Operations in progress metrics mismatch: %v", err)
	}
}

func TestUpdateGauges(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Set database-sourced values
	metrics.CurrentLogEntries = 100
	metrics.CurrentBytes = 1024
	metrics.LimitBytes = 2048
	metrics.CurrentFiles = 50
	metrics.CurrentBins = 10

	// Update gauges
	metrics.UpdateGauges()

	// Test transactions gauge
	expectedTransactions := `
		# HELP filebin_transactions Number of transactions logged in the database
		# TYPE filebin_transactions gauge
		filebin_transactions{id="test"} 100
	`
	if err := testutil.CollectAndCompare(metrics.transactions, strings.NewReader(expectedTransactions)); err != nil {
		t.Errorf("Transactions gauge mismatch: %v", err)
	}

	// Test storage bytes gauge
	expectedStorage := `
		# HELP filebin_storage_bytes The number of bytes consumed by the files in storage
		# TYPE filebin_storage_bytes gauge
		filebin_storage_bytes{id="test",type="limit"} 2048
		filebin_storage_bytes{id="test",type="used"} 1024
	`
	if err := testutil.CollectAndCompare(metrics.storageBytes, strings.NewReader(expectedStorage)); err != nil {
		t.Errorf("Storage bytes gauge mismatch: %v", err)
	}

	// Test files gauge
	expectedFiles := `
		# HELP filebin_files The number of files in storage
		# TYPE filebin_files gauge
		filebin_files{id="test"} 50
	`
	if err := testutil.CollectAndCompare(metrics.files, strings.NewReader(expectedFiles)); err != nil {
		t.Errorf("Files gauge mismatch: %v", err)
	}

	// Test bins gauge
	expectedBins := `
		# HELP filebin_bins The number of bins active
		# TYPE filebin_bins gauge
		filebin_bins{id="test"} 10
	`
	if err := testutil.CollectAndCompare(metrics.bins, strings.NewReader(expectedBins)); err != nil {
		t.Errorf("Bins gauge mismatch: %v", err)
	}
}

func TestMetricsWithCustomId(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("production", registry)

	metrics.IncrFileUploadCount()

	expected := `
		# HELP filebin_file_operations Number of file operations
		# TYPE filebin_file_operations counter
		filebin_file_operations{id="production",type="upload"} 1
	`

	if err := testutil.CollectAndCompare(metrics.fileOperations, strings.NewReader(expected)); err != nil {
		t.Errorf("Custom ID metrics mismatch: %v", err)
	}
}

func TestConcurrentMetricUpdates(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics("test", registry)

	// Test concurrent increments (Prometheus counters are thread-safe)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				metrics.IncrFileUploadCount()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	expected := `
		# HELP filebin_file_operations Number of file operations
		# TYPE filebin_file_operations counter
		filebin_file_operations{id="test",type="upload"} 1000
	`

	if err := testutil.CollectAndCompare(metrics.fileOperations, strings.NewReader(expected)); err != nil {
		t.Errorf("Concurrent metric updates failed: %v", err)
	}
}
