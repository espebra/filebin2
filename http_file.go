package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	//"strings"
	"time"

	"github.com/espebra/filebin2/ds"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gorilla/mux"
)

func (h *HTTP) getFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	// Files should never be indexed
	w.Header().Set("X-Robots-Tag", "noindex")

	t0 := time.Now()
	params := mux.Vars(r)
	inputBin := params["bin"]
	// TODO: Input validation (inputBin)
	inputFilename := params["filename"]
	// TODO: Input validation (inputFilename)

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to select bin by id %q: %s", inputBin, err.Error()), "Database error", 112, http.StatusInternalServerError)
		return
	}
	if found == false {
		h.Error(w, r, "", "The bin does not exist.", 113, http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		h.Error(w, r, "", "This bin is no longer available.", 114, http.StatusNotFound)
		return
	}

	// If approvals are required, then
	if h.config.RequireApproval {
		// Reject downloads from bins that are not approved
		if bin.IsApproved() == false {
			h.Error(w, r, "", "This bin requires approval before files can be downloaded.", 521, http.StatusForbidden)
			return
		}
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to select file by bin %q and filename %q: %s", inputBin, inputFilename, err.Error()), "Database error", 115, http.StatusInternalServerError)
		return
	}
	if found == false {
		h.Error(w, r, "", "The file does not exist.", 116, http.StatusNotFound)
		return
	}
	if file.IsReadable() == false {
		h.Error(w, r, "", "The file is no longer available.", 117, http.StatusNotFound)
		return
	}
	if file.InStorage == false {
		h.Error(w, r, "", "The file is not available.", 118, http.StatusNotFound)
		return
	}

	// Download limit
	// 0 disables the limit
	// >= 1 enforces a limit
	if h.config.LimitFileDownloads > 0 {
		if file.Downloads >= h.config.LimitFileDownloads {
			h.Error(w, r, "", "The file has been requested too many times.", 421, http.StatusForbidden)
			return
		}
	}

	// The file is downloadable at this point
	if h.config.RequireCookie {
		if h.cookieVerify(w, r) == false {
			// Set the cookie
			h.setVerificationCookie(w, r)

			// Show the warning
			type Data struct {
				ds.Common
				Bin     ds.Bin `json:"bin"`
				NextUrl string `json:"next_url"`
			}
			var data Data
			data.Bin = bin
			var nextUrl url.URL
			nextUrl.Scheme = h.config.BaseUrl.Scheme
			nextUrl.Host = h.config.BaseUrl.Host
			nextUrl.Path = path.Join(h.config.BaseUrl.Path, r.URL.Path)
			data.NextUrl = nextUrl.String()
			if err := h.templates.ExecuteTemplate(w, "cookie", data); err != nil {
				fmt.Printf("Failed to execute template: %s\n", err.Error())
				http.Error(w, "Errno 302", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	if err := h.dao.File().RegisterDownload(&file); err != nil {
		fmt.Printf("Unable to update file %q in bin %q: %s\n", inputFilename, inputBin, err.Error())
	}

	// Redirect the client to a presigned URL for this fetch, which is more efficient
	// than proxying the request through filebin.
	presignedURL, err := h.s3.PresignedGetObject(inputBin, inputFilename, file.Mime)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Unable to generate presigned URL for bin %q and filename %q: %s", inputBin, inputFilename, err.Error()), "Unable to presign URL for object", 1351, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", presignedURL.String())
	w.WriteHeader(http.StatusFound)
	io.WriteString(w, "")

	fmt.Printf("Presigned download of file %q (%s) from bin %q in %.3fs (%d downloads)\n", inputFilename, humanize.Bytes(file.Bytes), inputBin, time.Since(t0).Seconds(), file.Downloads)

	// Increment the byte counter here
	// Assume that the client will download the entire file from S3. This will
	// not always be the case.
	h.metrics.IncrBytesStorageToClient(file.Bytes)
	h.metrics.IncrFileDownloadCount()

	return
}

func (h *HTTP) uploadFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	t0 := time.Now()

	params := mux.Vars(r)
	inputBin := params["bin"]
	inputFilename := params["filename"]

	h.metrics.IncrFileUploadInProgress()
	defer h.metrics.DecrFileUploadInProgress()

	// Deprecated: This block is here to be compatible with the clients that
	// are written for https://github.com/espebra/filebin, meaning clients that
	// upload files to / with the request headers bin and filename set instead
	// of /{bin}/{filename}
	if inputBin == "" || inputFilename == "" {
		inputFilename = r.Header.Get("filename")
		if inputFilename == "" {
			h.Error(w, r, "Upload failed: missing filename request header", "Missing filename request header", 952, http.StatusBadRequest)
			return
		}

		inputBin = r.Header.Get("bin")
		if inputBin == "" {
			inputBin = h.dao.Bin().GenerateId()
			fmt.Printf("Auto generated bin: %s\n", inputBin)
		}
	}

	inputMD5 := r.Header.Get("Content-MD5")
	inputSHA256 := r.Header.Get("Content-SHA256")

	inputBytes, err := strconv.ParseUint(r.Header.Get("content-length"), 10, 64)
	if err != nil {
		h.Error(w, r, "Upload failed: Invalid content-length header", "Missing or invalid content-length header", 120, http.StatusLengthRequired)
		return
	}
	// TODO: Input validation on content-length. Between min:max.

	// Reject file names with certain extensions
	// Remove the . from the extension
	thisExtension := path.Ext(inputFilename)
	if len(thisExtension) > 0 {
		for _, extension := range h.config.RejectFileExtensions {
			if "."+extension == thisExtension {
				h.Error(w, r, fmt.Sprintf("Rejecting file name %s with illegal extension: %s", inputFilename, extension), "Illegal file extension", 992, http.StatusForbidden)
				return
			}
		}
	}

	// Check if bin exists
	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to select bin by id %q: %s", inputBin, err.Error()), "Database error", 112, http.StatusInternalServerError)
		return
	}

	if found == false {
		// Bin does not exist, so create it here
		bin = ds.Bin{}
		bin.Id = inputBin

		// Since manual approval is not needed, then just set the approval time at the time of the upload
		if h.config.RequireApproval == false {
			now := time.Now().UTC().Truncate(time.Microsecond)
			bin.ApprovedAt.Scan(now)
		}

		// Abort early if the bin is invalid
		if err := h.dao.Bin().ValidateInput(&bin); err != nil {
			h.Error(w, r, fmt.Sprintf("Input validation error on upload: %s", err.Error()), err.Error(), 623, http.StatusBadRequest)
			return
		}

		bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
		if err := h.dao.Bin().Upsert(&bin); err != nil {
			h.Error(w, r, fmt.Sprintf("Unable to upsert bin %q: %s", inputBin, err.Error()), "Database error", 121, http.StatusInternalServerError)
			return
		}
		// TODO: Execute new bin created trigger

		h.metrics.IncrNewBinCount()
	}

	if bin.IsWritable() == false {
		if bin.IsExpired() {
			h.Error(w, r, fmt.Sprintf("Upload failed: Bin %q is expired", inputBin), "The bin is no longer available", 122, http.StatusMethodNotAllowed)
			return
		} else if bin.IsDeleted() {
			// Reject uploads to deleted bins
			h.Error(w, r, fmt.Sprintf("Upload failed: Bin %q is deleted", inputBin), "The bin is no longer available", 132, http.StatusMethodNotAllowed)
			return
		} else if bin.Readonly == true {
			// Reject uploads to readonly bins
			w.Header().Set("Allow", "GET, HEAD")
			h.Error(w, r, fmt.Sprintf("Rejected upload of filename %q to readonly bin %q", inputFilename, inputBin), "Uploads to locked binds are not allowed", 123, http.StatusMethodNotAllowed)
			return
		} else {
			h.Error(w, r, fmt.Sprintf("Rejected upload of filename %q to bin %q for unknown reason", inputFilename, inputBin), "Unexpected upload failure", 134, http.StatusInternalServerError)
			return
		}
	}

	// Storage limit
	// 0 disables the limit
	// >= 1 enforces a limit, in number of gigabytes stored
	if h.config.LimitStorageBytes > 0 {
		totalBytesConsumed := h.dao.Metrics().StorageBytesAllocated()
		if totalBytesConsumed >= h.config.LimitStorageBytes {
			h.Error(w, r, fmt.Sprintf("Storage limit reached (currently consuming %s) when trying to upload file %q to bin %q", humanize.Bytes(totalBytesConsumed), inputFilename, inputBin), "Insufficient storage, please retry later\n", 633, http.StatusInsufficientStorage)
			return
		}
	}

	//fmt.Printf("Uploading filename %s (%s) to bin %s\n", inputFilename, humanize.Bytes(inputBytes), bin.Id)

	// Add timestamp to the temporary file to make it easy to see when
	// an upload was started.
	fp, err := h.workspace.CreateTempFile(inputBytes, fmt.Sprintf("filebin-%s-", t0.Format("20060102-150405")))
	// Defer removal of the tempfile to clean up partially uploaded files.
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to create temporary upload file: %s", err.Error()), "Storage error", 124, http.StatusInternalServerError)
		return
	}
	defer os.Remove(fp.Name())
	defer fp.Close()

	t1 := time.Now()

	// Compute MD5 and SHA256 checksums during the initial write to reduce disk IOPS
	md5Checksum := md5.New()
	sha256Checksum := sha256.New()
	multiWriter := io.MultiWriter(fp, md5Checksum, sha256Checksum)

	nBytes, err := io.Copy(multiWriter, r.Body)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("File upload of file %q bin %q aborted at %s of %s, upload started %s: %s (%s)", inputFilename, bin.Id, humanize.Bytes(uint64(nBytes)), humanize.Bytes(inputBytes), humanize.Time(t0), err.Error(), fp.Name()), "Storage error", 125, http.StatusInternalServerError)
		return
	}
	if uint64(nBytes) != inputBytes {
		h.Error(w, r, fmt.Sprintf("Rejecting upload for file %q to bin %q since we got %d bytes and should have received %d bytes", inputFilename, bin.Id, nBytes, inputBytes), "Content-length did not match the request body length", 126, http.StatusBadRequest)
		return
	}
	if nBytes == 0 {
		h.Error(w, r, "", "Empty file uploads are not allowed", 127, http.StatusBadRequest)
		return
	}

	t2 := time.Now()

	// Checksums are already calculated from the write above
	md5ChecksumString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%x", md5Checksum.Sum(nil))))
	if inputMD5 != "" {
		if md5ChecksumString != inputMD5 {
			h.Error(w, r, fmt.Sprintf("Rejecting upload for file %q to bin %q due to wrong MD5 checksum (got %s and calculated %s)", inputFilename, bin.Id, inputMD5, md5ChecksumString), "MD5 checksum did not match", 129, http.StatusBadRequest)
			return
		}
	}

	sha256ChecksumString := fmt.Sprintf("%x", sha256Checksum.Sum(nil))
	if inputSHA256 != "" {
		if sha256ChecksumString != inputSHA256 {
			h.Error(w, r, fmt.Sprintf("Rejecting upload for file %q to bin %q due to wrong SHA256 checksum (got %s and calculated %s)", inputFilename, bin.Id, inputSHA256, sha256ChecksumString), "SHA256 checksum did not match", 130, http.StatusBadRequest)
			return
		}
	}
	fp.Seek(0, 0)

	mime, err := mimetype.DetectReader(fp)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Unable to detect mime type on filename %q in bin %q: %s", inputFilename, inputBin, err.Error()), "Processing error", 131, http.StatusInternalServerError)
		return
	}
	fp.Seek(0, 0)

	t3 := time.Now()

	// Check if file exists
	file, found, err := h.dao.File().GetByName(bin.Id, inputFilename)
	if err != nil {
		fmt.Printf("Unable to load file %q in bin %q: %s\n", file.Filename, bin.Id, err.Error())
		http.Error(w, "Errno 106", http.StatusInternalServerError)
		return
	}

	if found {
		// Increment the update counter if the file exists.
		file.Updates = file.Updates + 1
	}

	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		h.Error(w, r, "Failed to dump request", "Parse error", 135, http.StatusInternalServerError)
		return
	}
	file.Headers = string(dump)

	// Extract client IP
	ip, err := extractIP(r.RemoteAddr)
	if err != nil {
		h.Error(w, r, "Failed to dump request", "Parse error", 136, http.StatusInternalServerError)
		return
	}
	file.IP = ip

	// Set values according to the new file
	file.Filename = inputFilename
	file.Bin = bin.Id

	// Reset the deleted status and timestamp in case the file was deleted
	// earlier
	file.DeletedAt.Scan(nil)

	file.ClientId = ""
	file.Bytes = inputBytes
	file.Mime = mime.String()
	file.SHA256 = sha256ChecksumString
	file.MD5 = md5ChecksumString
	if err := h.dao.File().ValidateInput(&file); err != nil {
		fmt.Printf("Rejected upload of filename %q to bin %q due to failed input validation: %s\n", inputFilename, bin.Id, err.Error())
		http.Error(w, "Input validation failed", http.StatusBadRequest)
		return
	}

	// Retry if the S3 upload fails
	retryLimit := 3
	retryCounter := 1

	h.metrics.IncrStorageUploadInProgress()
	defer h.metrics.DecrStorageUploadInProgress()

	for {
		fp.Seek(0, 0)
		err := h.s3.PutObject(file.Bin, file.Filename, fp, nBytes)
		if err == nil {
			// Completed successfully
			h.s3.SetTrace(false)
			break
		} else {
			// Completed with error
			if retryCounter >= retryLimit {
				// Give up after a few attempts
				fmt.Printf("Gave up uploading to S3 after %d/%d attempts: %s\n", retryCounter, retryLimit, err.Error())
				http.Error(w, "Failed to store the object in S3, please try again later", http.StatusInternalServerError)
				return
			}
			fmt.Printf("Failed attempt to upload to S3 (%d/%d): %s\n", retryCounter, retryLimit, err.Error())

			retryCounter = retryCounter + 1

			// Sleep a little before retrying
			time.Sleep(time.Duration(retryCounter) * time.Second)

			// Get some more debug data in case the retry also fails
			h.s3.SetTrace(true)
		}
	}
	t4 := time.Now()

	file.InStorage = true

	if found {
		if err := h.dao.File().Update(&file); err != nil {
			fmt.Printf("Unable to update filename %q (id %d) in bin %q: %s\n", file.Filename, file.Id, bin.Id, err.Error())
			http.Error(w, "Errno 107", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.dao.File().Insert(&file); err != nil {
			fmt.Printf("Unable to insert file %q in bin %q: %s\n", file.Filename, bin.Id, err.Error())
			http.Error(w, "Errno 108", http.StatusInternalServerError)
			return
		}
		// TODO: Execute new file created trigger
	}

	// Update bin to set the correct updated timestamp
	bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
	if err := h.dao.Bin().Update(&bin); err != nil {
		fmt.Printf("Unable to update bin %q: %s\n", bin.Id, err.Error())
		http.Error(w, "Errno 109", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Uploaded filename %q (%s) to bin %q (db in %.3fs, buffered in %.3fs, checksum in %.3fs, stored in %.3fs, total %.3fs)\n", file.Filename, humanize.Bytes(file.Bytes), bin.Id, t1.Sub(t0).Seconds(), t2.Sub(t1).Seconds(), t3.Sub(t2).Seconds(), t4.Sub(t3).Seconds(), t4.Sub(t0).Seconds())

	// Metrics
	h.metrics.IncrFileUploadCount()
	h.metrics.IncrBytesClientToFilebin(file.Bytes)
	h.metrics.IncrBytesFilebinToStorage(file.Bytes)

	type Data struct {
		Bin  ds.Bin  `json:"bin"`
		File ds.File `json:"file"`
	}
	var data Data
	data.Bin = bin
	data.File = file

	w.Header().Set("Content-Type", "application/json")
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Printf("Failed to parse json: %s\n", err.Error())
		http.Error(w, "Errno 201", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	io.WriteString(w, string(out))
}

func (h *HTTP) deleteFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]
	inputFilename := params["filename"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 110", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		h.Error(w, r, "", "The bin is no longer available", 122, http.StatusNotFound)
		return
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		fmt.Printf("Unable to GetByName(%q, %q): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Errno 111", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}

	// No need to delete the file twice
	if file.IsReadable() == false {
		http.Error(w, "This file is no longer available", http.StatusNotFound)
		return
	}

	// Flag as deleted
	now := time.Now().UTC().Truncate(time.Microsecond)
	file.DeletedAt.Scan(now)

	if err := h.dao.File().Update(&file); err != nil {
		fmt.Printf("Unable to update the file (%q, %q): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Update the updated timestamp of the bin
	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.metrics.IncrFileDeleteCount()
	http.Error(w, "File deleted successfully", http.StatusOK)
	return
}
