package ds

import (
	//"fmt"
	"sync"
)

type Metrics struct {
	CurrentLogEntries         int64  `json:"current_log_entries"`
	CurrentBytes              int64  `json:"current_bytes"`
	CurrentBytesReadable      string `json:"current_bytes_readable"`
	CurrentFiles              int64  `json:"current_files"`
	CurrentFilesReadable      string `json:"current_files_readable"`
	CurrentBins               int64  `json:"current_bins"`
	CurrentBinsReadable       string `json:"current_bins_readable"`
	FreeBytes                 int64  `json:"-"`
	FreeBytesReadable         string `json:"-"`
	TotalBytes                int64  `json:"total_bytes"`
	TotalBytesReadable        string `json:"total_bytes_readable"`
	TotalFiles                int64  `json:"total_files"`
	TotalFilesReadable        string `json:"total_files_readable"`
	TotalBins                 int64  `json:"total_bins"`
	TotalBinsReadable         string `json:"total_bins_readable"`
	BytesFilebinToStorage     uint64 `json:"bytes-filebin-to-storage"`
	BytesStorageToFilebin     uint64 `json:"bytes-storage-to-filebin"`
	BytesFilebinToClient      uint64 `json:"bytes-filebin-to-client"`
	BytesClientToFilebin      uint64 `json:"bytes-client-to-client"`
	BytesStorageToClient      uint64 `json:"bytes-storage-to-client"`
	FileUploadCount           uint64 `json:"file-upload-count"`
	FileDownloadCount         uint64 `json:"file-download-count"`
	FileDeleteCount           uint64 `json:"file-delete-count"`
	BinDeleteCount            uint64 `json:"bin-delete-count"`
	TarArchiveDownloadCount   uint64 `json:"tar-archive-download-count"`
	ZipArchiveDownloadCount   uint64 `json:"zip-archive-download-count"`
	FrontPageViewCount        uint64 `json:"front-page-view-count"`
	BinPageViewCount          uint64 `json:"bin-page-view-count"`
	NewBinCount               uint64 `json:"new-bin-count"`
	FileUploadInProgress      uint64 `json:"file-uploads-in-progress"`
	StorageUploadInProgress   uint64 `json:"storage-uploads-in-progress"`
	ArchiveDownloadInProgress uint64 `json:"archive-downloads-in-progress"`
	sync.Mutex
}

func (m *Metrics) IncrBytesFilebinToClient(value uint64) {
	m.Lock()
	m.BytesFilebinToClient = m.BytesFilebinToClient + value
	m.Unlock()
}

func (m *Metrics) IncrBytesClientToFilebin(value uint64) {
	m.Lock()
	m.BytesClientToFilebin = m.BytesClientToFilebin + value
	m.Unlock()
}

func (m *Metrics) IncrBytesFilebinToStorage(value uint64) {
	m.Lock()
	m.BytesFilebinToStorage = m.BytesFilebinToStorage + value
	m.Unlock()
}

func (m *Metrics) IncrBytesStorageToFilebin(value uint64) {
	m.Lock()
	m.BytesStorageToFilebin = m.BytesStorageToFilebin + value
	m.Unlock()
}

func (m *Metrics) IncrBytesStorageToClient(value uint64) {
	m.Lock()
	m.BytesStorageToClient = m.BytesStorageToClient + value
	m.Unlock()
}

func (m *Metrics) IncrFileUploadCount() {
	m.Lock()
	m.FileUploadCount = m.FileUploadCount + 1
	m.Unlock()
}

func (m *Metrics) IncrFileDownloadCount() {
	m.Lock()
	m.FileDownloadCount = m.FileDownloadCount + 1
	m.Unlock()
}

func (m *Metrics) IncrTarArchiveDownloadCount() {
	m.Lock()
	m.TarArchiveDownloadCount = m.TarArchiveDownloadCount + 1
	m.Unlock()
}

func (m *Metrics) IncrZipArchiveDownloadCount() {
	m.Lock()
	m.ZipArchiveDownloadCount = m.ZipArchiveDownloadCount + 1
	m.Unlock()
}

func (m *Metrics) IncrFileDeleteCount() {
	m.Lock()
	m.FileDeleteCount = m.FileDeleteCount + 1
	m.Unlock()
}

func (m *Metrics) IncrFrontPageViewCount() {
	m.Lock()
	m.FrontPageViewCount = m.FrontPageViewCount + 1
	m.Unlock()
}

func (m *Metrics) IncrBinPageViewCount() {
	m.Lock()
	m.BinPageViewCount = m.BinPageViewCount + 1
	m.Unlock()
}

func (m *Metrics) IncrNewBinCount() {
	m.Lock()
	m.NewBinCount = m.NewBinCount + 1
	m.Unlock()
}

func (m *Metrics) IncrBinDeleteCount() {
	m.Lock()
	m.BinDeleteCount = m.BinDeleteCount + 1
	m.Unlock()
}

func (m *Metrics) IncrFileUploadInProgress() {
	m.Lock()
	m.FileUploadInProgress = m.FileUploadInProgress + 1
	m.Unlock()
}

func (m *Metrics) DecrFileUploadInProgress() {
	m.Lock()
	m.FileUploadInProgress = m.FileUploadInProgress - 1
	m.Unlock()
}

func (m *Metrics) IncrArchiveDownloadInProgress() {
	m.Lock()
	m.ArchiveDownloadInProgress = m.ArchiveDownloadInProgress + 1
	m.Unlock()
}

func (m *Metrics) DecrArchiveDownloadInProgress() {
	m.Lock()
	m.ArchiveDownloadInProgress = m.ArchiveDownloadInProgress - 1
	m.Unlock()
}

func (m *Metrics) IncrStorageUploadInProgress() {
	m.Lock()
	m.StorageUploadInProgress = m.StorageUploadInProgress + 1
	m.Unlock()
}

func (m *Metrics) DecrStorageUploadInProgress() {
	m.Lock()
	m.StorageUploadInProgress = m.StorageUploadInProgress - 1
	m.Unlock()
}
