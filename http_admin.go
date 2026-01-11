package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/s3"
)

func (h *HTTP) viewAdminDashboard(w http.ResponseWriter, r *http.Request) {
	type StorageMetrics struct {
		UsedBytes          uint64  `json:"used_bytes"`
		UsedBytesReadable  string  `json:"used_bytes_readable"`
		TotalBytes         uint64  `json:"total_bytes"`
		TotalBytesReadable string  `json:"total_bytes_readable"`
		FreeBytes          uint64  `json:"free_bytes"`
		FreeBytesReadable  string  `json:"free_bytes_readable"`
		UsedPercent        float64 `json:"used_percent"`
	}

	type Data struct {
		//Bins Bins `json:"bins"`
		//Files []ds.File `json:"files"`
		BucketMetrics  s3.BucketMetrics `json:"bucketmetrics"`
		Page           string           `json:"page"`
		DBMetrics      ds.Metrics       `json:"db_metrics"`
		DBStats        sql.DBStats      `json:"db_stats"`
		Config         ds.Config        `json:"-"`
		AdminLogins    []AdminLogin     `json:"admin_logins"`
		StorageMetrics StorageMetrics   `json:"storage_metrics"`
	}
	var data Data
	data.Config = *h.config
	data.Page = "about"

	h.adminLoginsMutex.Lock()
	data.AdminLogins = make([]AdminLogin, len(h.adminLogins))
	copy(data.AdminLogins, h.adminLogins)
	h.adminLoginsMutex.Unlock()

	// Get storage metrics
	usedBytes := h.dao.Metrics().StorageBytesAllocated()
	data.StorageMetrics.UsedBytes = usedBytes
	data.StorageMetrics.UsedBytesReadable = humanize.Bytes(usedBytes)
	data.StorageMetrics.TotalBytes = h.config.LimitStorageBytes
	data.StorageMetrics.TotalBytesReadable = h.config.LimitStorageReadable

	if h.config.LimitStorageBytes > 0 {
		if usedBytes > h.config.LimitStorageBytes {
			data.StorageMetrics.FreeBytes = 0
		} else {
			data.StorageMetrics.FreeBytes = h.config.LimitStorageBytes - usedBytes
		}
		data.StorageMetrics.FreeBytesReadable = humanize.Bytes(data.StorageMetrics.FreeBytes)
		data.StorageMetrics.UsedPercent = float64(usedBytes) / float64(h.config.LimitStorageBytes) * 100
	} else {
		// No limit configured
		data.StorageMetrics.FreeBytes = 0
		data.StorageMetrics.FreeBytesReadable = "Unlimited"
		data.StorageMetrics.UsedPercent = 0
	}
	//data.BucketMetrics = h.s3.GetBucketMetrics()
	//err := h.dao.Metrics().GetMetrics(h.metrics)
	//if err != nil {
	//	fmt.Printf("Unable to GetMetrics(): %s\n", err.Error())
	//	http.Error(w, "Errno 326", http.StatusInternalServerError)
	//	return
	//}
	//data.DBMetrics = metrics
	//freeBytes := int64(h.config.LimitStorageBytes) - metrics.CurrentBytes
	//if freeBytes < 0 {
	//	freeBytes = 0
	//}
	//data.DBMetrics.FreeBytes = freeBytes
	//data.DBMetrics.FreeBytesReadable = humanize.Bytes(uint64(freeBytes))

	data.DBStats = h.dao.Stats()

	//binsAvailable, err := h.dao.Bin().GetAll()
	//if err != nil {
	//	fmt.Printf("Unable to GetAll(): %s\n", err.Error())
	//	http.Error(w, "Errno 200", http.StatusInternalServerError)
	//	return
	//}

	//var bins Bins
	//bins.Available = binsAvailable

	//data.Bins = bins

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_dashboard", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) viewAdminFiles(w http.ResponseWriter, r *http.Request) {
	inputLimit := r.URL.Query().Get("limit")

	// default
	limit := 25

	i, err := strconv.Atoi(inputLimit)
	if err == nil {
		if i >= 1 && i <= 5000 {
			limit = i
		}
	}

	type Files struct {
		ByCreated   []ds.File `json:"by-created"`
		ByUpdated   []ds.File `json:"by-updated"`
		ByBytes     []ds.File `json:"by-bytes"`
		ByDownloads []ds.File `json:"by-downloads"`
		ByUpdates   []ds.File `json:"by-updates"`
	}

	type Data struct {
		Files Files `json:"files"`
		Limit int   `json:"limit"`
	}
	var data Data
	data.Limit = limit

	filesByCreated, err := h.dao.File().GetByCreated(limit)
	if err != nil {
		fmt.Printf("Unable to GetByCreated(): %s\n", err.Error())
		http.Error(w, "Errno 494", http.StatusInternalServerError)
		return
	}

	filesByUpdated, err := h.dao.File().GetByUpdated(limit)
	if err != nil {
		fmt.Printf("Unable to GetByUpdated(): %s\n", err.Error())
		http.Error(w, "Errno 495", http.StatusInternalServerError)
		return
	}

	filesByBytes, err := h.dao.File().GetByBytes(limit)
	if err != nil {
		fmt.Printf("Unable to GetByBytes(): %s\n", err.Error())
		http.Error(w, "Errno 496", http.StatusInternalServerError)
		return
	}

	filesByDownloads, err := h.dao.File().GetTopDownloads(limit)
	if err != nil {
		fmt.Printf("Unable to GetTopDownloads(): %s\n", err.Error())
		http.Error(w, "Errno 497", http.StatusInternalServerError)
		return
	}

	filesByUpdates, err := h.dao.File().GetByUpdates(limit)
	if err != nil {
		fmt.Printf("Unable to GetByUpdates(): %s\n", err.Error())
		http.Error(w, "Errno 498", http.StatusInternalServerError)
		return
	}

	var files Files
	files.ByCreated = filesByCreated
	files.ByUpdated = filesByUpdated
	files.ByBytes = filesByBytes
	files.ByDownloads = filesByDownloads
	files.ByUpdates = filesByUpdates

	data.Files = files

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_files", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) viewAdminFileContent(w http.ResponseWriter, r *http.Request) {
	inputLimit := r.URL.Query().Get("limit")

	// default
	limit := 25

	i, err := strconv.Atoi(inputLimit)
	if err == nil {
		if i >= 1 && i <= 5000 {
			limit = i
		}
	}

	type FileContent struct {
		ByReferenceCount []ds.FileByChecksum `json:"by-reference-count"`
		ByBytesEach      []ds.FileByChecksum `json:"by-bytes-each"`
		ByBytesTotal     []ds.FileByChecksum `json:"by-bytes-total"`
		ByCreated        []ds.FileByChecksum `json:"by-created"`
		Blocked          []ds.FileByChecksum `json:"blocked"`
	}

	type Data struct {
		FileContent FileContent `json:"file_content"`
		Limit       int         `json:"limit"`
	}
	var data Data
	data.Limit = limit

	// By reference count (deduplication count)
	contentByRefCount, err := h.dao.File().FilesByChecksum(limit)
	if err != nil {
		fmt.Printf("Unable to FilesByChecksum(): %s\n", err.Error())
		http.Error(w, "Errno 493", http.StatusInternalServerError)
		return
	}

	// By bytes each
	contentByBytesEach, err := h.dao.File().FilesByBytes(limit)
	if err != nil {
		fmt.Printf("Unable to FilesByBytes(): %s\n", err.Error())
		http.Error(w, "Errno 494", http.StatusInternalServerError)
		return
	}

	// By bytes total
	contentByBytesTotal, err := h.dao.File().FilesByBytesTotal(limit)
	if err != nil {
		fmt.Printf("Unable to FilesByBytesTotal(): %s\n", err.Error())
		http.Error(w, "Errno 495", http.StatusInternalServerError)
		return
	}

	// By created at
	contentByCreated, err := h.dao.FileContent().GetByCreated(limit)
	if err != nil {
		fmt.Printf("Unable to GetByCreated(): %s\n", err.Error())
		http.Error(w, "Errno 495", http.StatusInternalServerError)
		return
	}

	// Blocked content
	blockedContent, err := h.dao.FileContent().GetBlocked(limit)
	if err != nil {
		fmt.Printf("Unable to GetBlocked(): %s\n", err.Error())
		http.Error(w, "Errno 496", http.StatusInternalServerError)
		return
	}

	var content FileContent
	content.ByReferenceCount = contentByRefCount
	content.ByBytesEach = contentByBytesEach
	content.ByBytesTotal = contentByBytesTotal
	content.ByCreated = contentByCreated
	content.Blocked = blockedContent

	data.FileContent = content

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_filecontent", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) viewAdminFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputSHA256 := params["sha256"]

	type Data struct {
		Files          []ds.File       `json:"files"`
		FileContent    *ds.FileContent `json:"file_content,omitempty"`
		SHA256         string          `json:"sha256"`
		S3URL          string          `json:"s3_url"`
		S3PresignedURL string          `json:"s3_presigned_url"`
	}
	var data Data
	data.SHA256 = inputSHA256
	data.S3URL = h.s3.GetObjectURL(inputSHA256)

	// Get file content metadata (common across all files with this SHA256)
	fileContent, err := h.dao.FileContent().GetBySHA256(inputSHA256)
	if err != nil {
		fmt.Printf("Unable to get file content for SHA256 %s: %s\n", inputSHA256, err.Error())
		// Don't fail completely, just log the error
	} else {
		data.FileContent = fileContent

		// Generate presigned URL for direct S3 access
		presignedURL, err := h.s3.PresignedGetObject(inputSHA256, inputSHA256, fileContent.Mime)
		if err != nil {
			fmt.Printf("Unable to generate presigned URL for SHA256 %s: %s\n", inputSHA256, err.Error())
			data.S3PresignedURL = ""
		} else {
			data.S3PresignedURL = presignedURL.String()
		}
	}

	fileByChecksum, err := h.dao.File().FileByChecksum(inputSHA256)
	if err != nil {
		fmt.Printf("Unable to FileByChecksum(): %s\n", err.Error())
		http.Error(w, "Errno 494", http.StatusInternalServerError)
		return
	}

	data.Files = fileByChecksum

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_file_by_checksum", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) blockFileContent(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sha256 := params["sha256"]

	// Block the content (marks it as blocked and deletes all file references)
	err := h.dao.FileContent().BlockContent(sha256)
	if err != nil {
		fmt.Printf("Unable to block content %s: %s\n", sha256, err.Error())
		http.Error(w, "Failed to block content", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Blocked content with SHA256: %s\n", sha256)

	// Redirect back to the file view page
	http.Redirect(w, r, "/admin/file/"+sha256, http.StatusSeeOther)
}

func (h *HTTP) unblockFileContent(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sha256 := params["sha256"]

	// Unblock the content
	err := h.dao.FileContent().UnblockContent(sha256)
	if err != nil {
		fmt.Printf("Unable to unblock content %s: %s\n", sha256, err.Error())
		http.Error(w, "Failed to unblock content", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Unblocked content with SHA256: %s\n", sha256)

	// Redirect back to the file view page
	http.Redirect(w, r, "/admin/file/"+sha256, http.StatusSeeOther)
}

func (h *HTTP) viewAdminBins(w http.ResponseWriter, r *http.Request) {
	inputLimit := r.URL.Query().Get("limit")

	// default
	limit := 25

	i, err := strconv.Atoi(inputLimit)
	if err == nil {
		if i >= 1 && i <= 5000 {
			limit = i
		}
	}

	type Bins struct {
		ByLastUpdated []ds.Bin `json:"by-last-updated"`
		ByBytes       []ds.Bin `json:"by-bytes"`
		ByDownloads   []ds.Bin `json:"by-downloads"`
		ByFiles       []ds.Bin `json:"by-files"`
		ByCreated     []ds.Bin `json:"by-created"`
	}

	type Data struct {
		Bins Bins `json:"bins"`
		//Files []ds.File `json:"files"`
		BucketMetrics s3.BucketMetrics `json:"bucketmetrics"`
		Page          string           `json:"page"`
		DBMetrics     ds.Metrics       `json:"db_metrics"`
		Limit         int              `json:"limit"`
	}
	var data Data
	data.Limit = limit

	binsByLastUpdated, err := h.dao.Bin().GetLastUpdated(limit)
	if err != nil {
		fmt.Printf("Unable to GetAll(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	binsByBytes, err := h.dao.Bin().GetByBytes(limit)
	if err != nil {
		fmt.Printf("Unable to GetByBytes(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	binsByDownloads, err := h.dao.Bin().GetByDownloads(limit)
	if err != nil {
		fmt.Printf("Unable to GetByDownloads(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	binsByFiles, err := h.dao.Bin().GetByFiles(limit)
	if err != nil {
		fmt.Printf("Unable to GetByFiles(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	binsByCreated, err := h.dao.Bin().GetByCreated(limit)
	if err != nil {
		fmt.Printf("Unable to GetByCreated(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	var bins Bins
	bins.ByLastUpdated = binsByLastUpdated
	bins.ByBytes = binsByBytes
	bins.ByDownloads = binsByDownloads
	bins.ByFiles = binsByFiles
	bins.ByCreated = binsByCreated

	data.Bins = bins

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_bins", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) viewAdminBinsAll(w http.ResponseWriter, r *http.Request) {
	type Bins struct {
		Available []ds.Bin `json:"available"`
	}

	type Data struct {
		Bins Bins `json:"bins"`
		//Files []ds.File `json:"files"`
		BucketMetrics s3.BucketMetrics `json:"bucketmetrics"`
		Page          string           `json:"page"`
		DBMetrics     ds.Metrics       `json:"db_metrics"`
	}
	var data Data
	//data.Page = "about"
	//data.BucketMetrics = h.s3.GetBucketMetrics()
	//metrics, err := h.dao.Metrics().GetMetrics()
	//if err != nil {
	//	fmt.Printf("Unable to GetMetrics(): %s\n", err.Error())
	//	http.Error(w, "Errno 326", http.StatusInternalServerError)
	//	return
	//}
	//data.DBMetrics = metrics

	binsAvailable, err := h.dao.Bin().GetAll()
	if err != nil {
		fmt.Printf("Unable to GetAll(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	var bins Bins
	bins.Available = binsAvailable

	data.Bins = bins

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_bins_all", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) viewAdminLog(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		Transactions []ds.Transaction `json:"transactions"`
	}
	var data Data

	params := mux.Vars(r)
	inputCategory := params["category"]
	inputFilter := params["filter"]

	if inputCategory == "bin" {
		bin := inputFilter
		trs, err := h.dao.Transaction().GetByBin(bin)
		if err != nil {
			http.Error(w, "Errno 361", http.StatusInternalServerError)
			return
		}
		data.Transactions = trs
	} else if inputCategory == "ip" {
		ip := inputFilter
		trs, err := h.dao.Transaction().GetByIP(ip)
		if err != nil {
			http.Error(w, "Errno 361", http.StatusInternalServerError)
			return
		}
		data.Transactions = trs
	}

	if err := h.templates.ExecuteTemplate(w, "log", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 203", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) viewAdminCleanup(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		Objects []string `json:"objects"`
		////Files []ds.File `json:"files"`
		//BucketMetrics s3.BucketMetrics `json:"bucketmetrics"`
		//Page       string        `json:"page"`
		Bins []ds.Bin `json:"bins"`
	}

	objects, err := h.s3.ListObjects()
	if err != nil {
		http.Error(w, "Errno 262", http.StatusInternalServerError)
		return
	}

	bins, err := h.dao.Bin().GetAll()
	if err != nil {
		http.Error(w, "Errno 361", http.StatusInternalServerError)
		return
	}

	var allbins []string
	for _, bin := range bins {
		b := sha256.New()
		b.Write([]byte(bin.Id))
		hash := fmt.Sprintf("%x", b.Sum(nil))
		allbins = append(allbins, hash)
	}

	for _, object := range objects {
		splits := strings.Split(object, "/")
		if len(splits) == 2 {
			hash := splits[0]
			if inStringSlice(hash, allbins) {
				fmt.Printf("Match\n")
			} else {
				fmt.Printf("No match\n")
			}
		} else {
			fmt.Printf("No match. Weird length: %d\n", len(splits))
		}
	}

	var data Data
	data.Objects = objects
	data.Bins = bins

	if err := h.templates.ExecuteTemplate(w, "cleanup", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 203", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) viewAdminClients(w http.ResponseWriter, r *http.Request) {
	inputLimit := r.URL.Query().Get("limit")

	// default
	limit := 25

	i, err := strconv.Atoi(inputLimit)
	if err == nil {
		if i >= 1 && i <= 5000 {
			limit = i
		}
	}

	type Clients struct {
		ByLastActiveAt  []ds.Client           `json:"by-last-active-at"`
		ByRequests      []ds.Client           `json:"by-requests"`
		ByBannedAt      []ds.Client           `json:"by-banned-at"`
		ByFilesUploaded []ds.Client           `json:"by-files-uploaded"`
		ByBytesUploaded []ds.Client           `json:"by-bytes-uploaded"`
		ByCountry       []ds.ClientByCountry  `json:"by-country"`
		ByNetwork       []ds.ClientByNetwork  `json:"by-network"`
		ByASN           []ds.AutonomousSystem `json:"by-asn"`
	}

	type Data struct {
		Clients Clients `json:"clients"`
		Limit   int     `json:"limit"`
	}
	var data Data
	data.Limit = limit

	clientsByLastActiveAt, err := h.dao.Client().GetByLastActiveAt(limit)
	if err != nil {
		fmt.Printf("Unable to .GetLastActive(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	clientsByRequests, err := h.dao.Client().GetByRequests(limit)
	if err != nil {
		fmt.Printf("Unable to .GetByRequests(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	clientsByBannedAt, err := h.dao.Client().GetByBannedAt(limit)
	if err != nil {
		fmt.Printf("Unable to .GetByBannedAt(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	clientsByFilesUploaded, err := h.dao.Client().GetByFilesUploaded(limit)
	if err != nil {
		fmt.Printf("Unable to .GetByFilesUploaded(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	clientsByBytesUploaded, err := h.dao.Client().GetByBytesUploaded(limit)
	if err != nil {
		fmt.Printf("Unable to .GetByBytesUploaded(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	clientsByCountry, err := h.dao.Client().GetByCountry(limit)
	if err != nil {
		fmt.Printf("Unable to .GetByCountry(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	clientsByNetwork, err := h.dao.Client().GetByNetwork(limit)
	if err != nil {
		fmt.Printf("Unable to .GetByNetwork(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	asnWithStats, err := h.dao.Client().GetASNWithStats(limit)
	if err != nil {
		fmt.Printf("Unable to .GetASNWithStats(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	var clients Clients
	clients.ByLastActiveAt = clientsByLastActiveAt
	clients.ByRequests = clientsByRequests
	clients.ByBannedAt = clientsByBannedAt
	clients.ByFilesUploaded = clientsByFilesUploaded
	clients.ByBytesUploaded = clientsByBytesUploaded
	clients.ByCountry = clientsByCountry
	clients.ByNetwork = clientsByNetwork
	clients.ByASN = asnWithStats

	data.Clients = clients

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_clients", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) viewAdminClientsAll(w http.ResponseWriter, r *http.Request) {
	clients, err := h.dao.Client().GetAll()
	if err != nil {
		fmt.Printf("Unable to .GetAll(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}
	type Data struct {
		Clients []ds.Client `json:"all-clients"`
	}

	var data Data
	data.Clients = clients

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "admin_clients_all", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}
