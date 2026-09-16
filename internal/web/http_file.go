package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
	"github.com/espebra/filebin2/internal/phash"
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
	if !found {
		h.Error(w, r, "", "The bin does not exist.", 113, http.StatusNotFound)
		return
	}

	if !bin.IsReadable() {
		h.Error(w, r, "", "This bin is no longer available.", 114, http.StatusNotFound)
		return
	}

	// If approvals are required, then
	if h.config.RequireApproval {
		// Reject downloads from bins that are not approved
		if !bin.IsApproved() {
			h.Error(w, r, "", "This bin requires approval before files can be downloaded.", 521, http.StatusForbidden)
			return
		}
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to select file by bin %q and filename %q: %s", inputBin, inputFilename, err.Error()), "Database error", 115, http.StatusInternalServerError)
		return
	}
	if !found {
		h.Error(w, r, "", "The file does not exist.", 116, http.StatusNotFound)
		return
	}

	// Check if file is available for download (checks file deletion, bin deletion, bin expiry, and content availability)
	available, err := h.dao.File().IsAvailableForDownload(file.Id)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to check file availability: %s", err.Error()), "Database error", 117, http.StatusInternalServerError)
		return
	}
	if !available {
		h.Error(w, r, "", "The file is not available for download.", 118, http.StatusNotFound)
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
		if !h.cookieVerify(w, r) {
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
			if err := h.renderTemplate(w, "cookie", data); err != nil {
				slog.Error("failed to execute template", "error", err)
				http.Error(w, "Errno 303", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	// The increment is the authoritative limit check: the read at the top of
	// the handler races with concurrent downloads, this conditional update
	// does not.
	allowed, err := h.dao.File().RegisterDownloadIfUnderLimit(&file, h.config.LimitFileDownloads)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to register download of filename %q in bin %q: %s", inputFilename, inputBin, err.Error()), "Database error", 141, http.StatusInternalServerError)
		return
	}
	if !allowed {
		h.Error(w, r, "", "The file has been requested too many times.", 421, http.StatusForbidden)
		return
	}

	// Redirect the client to a presigned URL for this fetch, which is more efficient
	// than proxying the request through filebin.
	presignedURL, err := h.s3.PresignedGetObject(file.SHA256, file.Filename, file.Mime)
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Unable to generate presigned URL for bin %q and filename %q: %s", inputBin, inputFilename, err.Error()), "Unable to presign URL for object", 1351, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", presignedURL.String())
	w.WriteHeader(http.StatusFound)

	slog.Info("presigned download", "filename", inputFilename, "bytes", file.Bytes, "sha256", file.SHA256, "bin", inputBin, "duration_seconds", time.Since(t0).Seconds(), "downloads", file.Downloads)

	// Increment the byte counter here
	// Assume that the client will download the entire file from S3. This will
	// not always be the case.
	h.metrics.IncrBytesStorageToClient(file.Bytes)
	h.metrics.IncrFileDownloadCount()
}

// uploadFile handles POST and PUT of a file to a bin. The flow is:
//
//  1. Resolve the target bin and filename from the request.
//  2. Validate the request before reading the body: content-length, file
//     extension, bin state and storage limit. The bin is created here if it
//     does not exist.
//  3. Buffer the body to a temporary file while calculating checksums, then
//     verify the checksums against the request headers.
//  4. Inspect the content: mime type and, for images, perceptual hash.
//  5. Build the file record and validate it.
//  6. Deduplicate against existing content and upload to S3 if needed.
//  7. Under the content lock, verify the object is in S3 and persist the
//     content record and the file reference.
//  8. Touch the bin, update metrics, run the post-upload hook and respond.
func (h *HTTP) uploadFile(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "max-age=0")

	t0 := time.Now()

	h.metrics.IncrFileUploadInProgress()
	defer h.metrics.DecrFileUploadInProgress()

	// Step 1: Resolve the bin and filename from the URL, or from the request
	// headers for legacy clients.
	inputBin, inputFilename, err := h.uploadTarget(r)
	if err != nil {
		return err
	}

	// Step 2: Validate the request before reading the body. The content
	// length is required to size the temporary file and to detect truncated
	// uploads.
	inputBytes, err := strconv.ParseUint(r.Header.Get("content-length"), 10, 64)
	if err != nil {
		return &httpError{status: http.StatusLengthRequired, message: "Missing or invalid content-length header", err: err}
	}
	// TODO: Input validation on content-length. Between min:max.

	// Reject file names with a blocked extension.
	if err := h.checkFileExtension(inputFilename); err != nil {
		return err
	}

	// Load the bin, or create it on the first upload.
	bin, err := h.getOrCreateBin(inputBin)
	if err != nil {
		return err
	}

	// Reject uploads to expired, deleted or locked bins.
	if err := checkBinWritable(bin); err != nil {
		return err
	}

	// Reject the upload if the total storage limit is reached.
	if err := h.checkStorageLimit(); err != nil {
		return err
	}

	t1 := time.Now()

	// Step 3: Buffer the request body to a temporary file. The checksums are
	// calculated during this write so the content is only read from the
	// client once.
	digest := newContentDigest()
	fp, err := h.bufferUpload(r, digest, inputBytes, t0)
	if err != nil {
		return err
	}
	// Remove the tempfile when done to clean up partially uploaded files.
	defer func() { _ = os.Remove(fp.Name()) }()
	defer func() { _ = fp.Close() }()
	// bufferUpload has verified that the body length matches content-length.
	nBytes := int64(inputBytes)

	t2 := time.Now()

	// Verify the checksums against the ones the client provided, if any.
	if err := verifyChecksums(r, digest); err != nil {
		return err
	}

	// Step 4: Detect the mime type and, for images, the perceptual hash.
	mime, pHashValue, pHashDuration, err := inspectContent(fp, inputFilename)
	if err != nil {
		return err
	}

	// Step 5: Build the file record. Start from the existing file if the
	// filename is already in the bin, so that its id and counters carry over.
	file, found, err := h.dao.File().GetByName(bin.Id, inputFilename)
	if err != nil {
		return fmt.Errorf("select file: %w", err)
	}

	if found {
		// Increment the update counter if the file exists.
		file.Updates = file.Updates + 1
	}

	// Keep the request headers for auditing.
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		return fmt.Errorf("dump request: %w", err)
	}
	file.Headers = string(dump)

	// Extract client IP
	ip, err := extractIP(r.RemoteAddr)
	if err != nil {
		return fmt.Errorf("extract client ip from %q: %w", r.RemoteAddr, err)
	}
	file.IP = ip

	// Set values according to the new file
	file.Filename = inputFilename
	file.Bin = bin.Id

	// Reset the deleted status and timestamp in case the file was deleted
	// earlier
	_ = file.DeletedAt.Scan(nil)

	file.Bytes = inputBytes
	file.Mime = mime
	file.SHA256 = digest.SHA256()
	file.MD5 = digest.MD5()

	// Validate the record before touching storage.
	if err := h.dao.File().ValidateInput(&file); err != nil {
		return &httpError{status: http.StatusBadRequest, message: "Input validation failed", err: err}
	}

	// Step 6: Deduplicate. Content is stored once per SHA256, so an upload
	// of content that is already in storage skips the S3 upload. Blocked
	// content is rejected here.
	existingContent, err := h.lookupExistingContent(file.SHA256)
	if err != nil {
		return err
	}
	skipS3Upload := false
	if existingContent != nil {
		if pHashValue == "" {
			pHashValue = existingContent.PHash
		}
		if existingContent.InStorage {
			// Content already in S3, skip upload
			skipS3Upload = true
			slog.Debug("content already exists in storage, skipping S3 upload", "sha256", file.SHA256)
		}
	}

	t3 := time.Now()

	// Upload the content to S3 unless it is already there.
	if !skipS3Upload {
		if err := h.storeContent(file.SHA256, fp, nBytes, 3); err != nil {
			return &httpError{status: http.StatusInternalServerError, message: "Failed to store the object in S3, please try again later", err: err}
		}
	}
	t4 := time.Now()

	// Step 7: Persist the content record and the file reference under the
	// content lock.
	//
	// Serialize against the lurker's content deletion (and other uploads of
	// the same content) while recording that the content is in storage and
	// creating the file reference. The lurker claims and deletes content only
	// while holding this lock, so while it is held no S3 delete of this
	// object can be in flight and the StatObject check below is
	// authoritative. The file reference is created before the lock is
	// released, which prevents the lurker from claiming the content
	// afterwards.
	unlockContent, err := h.dao.FileContent().LockContent(file.SHA256)
	if err != nil {
		return &httpError{status: http.StatusServiceUnavailable, message: "Failed to store the object, please try again later", err: err}
	}
	// unlockContent is idempotent. It is called explicitly once the file
	// reference is persisted; the defer is a safety net for error returns.
	defer unlockContent()

	// Verify that the object actually is in S3. It can be missing if the
	// lurker deleted it between the deduplication check and this point, or,
	// when we uploaded it above, if a previous deletion claim failed after
	// the S3 delete went through and rolled the in_storage flag back.
	if _, statErr := h.s3.StatObject(file.SHA256); statErr != nil {
		slog.Warn("object missing from storage, uploading", "sha256", file.SHA256, "error", statErr)
		// A single attempt, since the content lock is held and sleeping
		// between retries would block the lurker and other uploads of the
		// same content.
		if err := h.storeContent(file.SHA256, fp, nBytes, 1); err != nil {
			return &httpError{status: http.StatusInternalServerError, message: "Failed to store the object in S3, please try again later", err: err}
		}
		if skipS3Upload {
			// The bytes were not counted by the regular upload path above.
			h.metrics.IncrBytesFilebinToStorage(file.Bytes)
		}
	}

	// Record the content as stored. The content lock is held and the object
	// is verified present, so setting in_storage=true is safe.
	fileContent := ds.FileContent{
		SHA256:    file.SHA256,
		Bytes:     file.Bytes,
		MD5:       file.MD5,
		Mime:      file.Mime,
		PHash:     pHashValue,
		InStorage: true,
	}
	if err := h.dao.FileContent().InsertOrIncrement(&fileContent); err != nil {
		return fmt.Errorf("upsert file_content %s: %w", file.SHA256, err)
	}

	// Record upload duration
	file.UploadDurationMs = time.Since(t0).Milliseconds()

	// Insert or update the file reference in the bin.
	if err := h.persistFile(&file, found); err != nil {
		return err
	}

	// The content record and the file reference are persisted, so the
	// lurker's ClaimForDeletion guard now protects the content. Release the
	// lock before the slower bin update, post-upload hook and response.
	unlockContent()

	// Step 8: Finish up. Update the bin to set the correct updated timestamp
	// and extend its expiration. Use Touch (a targeted update of only
	// updated_at/expired_at, guarded by the bin not being deleted) rather
	// than a full Update so that this potentially slow, client-controlled
	// upload cannot revert moderation changes (delete, lock, or approval
	// revocation) that an admin or the lurker applied to the bin while the
	// upload was in flight. A bin locked during the upload is touched and
	// the upload succeeds, since the file was accepted before the lock and
	// is in the bin.
	bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
	updated, err := h.dao.Bin().Touch(&bin)
	if err != nil {
		return fmt.Errorf("touch bin: %w", err)
	}
	if !updated {
		// The bin was deleted concurrently during the upload. The file
		// reference created above will be cleaned up by the lurker; do not
		// resurrect the bin.
		return &httpError{status: http.StatusMethodNotAllowed, message: "The bin is no longer available", err: errors.New("bin was deleted during upload")}
	}

	// Count the upload in the metrics.
	h.metrics.IncrFileUploadCount()
	h.metrics.IncrBytesClientToFilebin(file.Bytes)
	if !skipS3Upload {
		h.metrics.IncrBytesFilebinToStorage(file.Bytes)
	}

	// Notify the post-upload hook, if one is configured.
	hookDuration := h.runPostUploadHook(r.Context(), file)

	// Respond with the bin and file as JSON.
	type Data struct {
		Bin  ds.Bin  `json:"bin"`
		File ds.File `json:"file"`
	}
	var data Data
	data.Bin = bin
	data.File = file

	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	t5 := time.Now()
	slog.Info("uploaded file", "filename", file.Filename, "bytes", file.Bytes, "sha256", file.SHA256, "bin", bin.Id, "db_seconds", t1.Sub(t0).Seconds(), "buffer_seconds", fmt.Sprintf("%f", t2.Sub(t1).Seconds()), "phash_seconds", pHashDuration.Seconds(), "hook_seconds", hookDuration.Seconds(), "store_seconds", t4.Sub(t3).Seconds(), "total_seconds", t5.Sub(t0).Seconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(out)
	return nil
}

// uploadTarget returns the bin and filename the upload is addressed to.
func (h *HTTP) uploadTarget(r *http.Request) (bin string, filename string, err error) {
	params := mux.Vars(r)
	bin = params["bin"]
	filename = params["filename"]
	if bin != "" && filename != "" {
		return bin, filename, nil
	}

	// Deprecated: This block is here to be compatible with the clients that
	// are written for https://github.com/espebra/filebin, meaning clients that
	// upload files to / with the request headers bin and filename set instead
	// of /{bin}/{filename}
	filename = r.Header.Get("filename")
	if filename == "" {
		return "", "", &httpError{status: http.StatusBadRequest, message: "Missing filename request header"}
	}

	bin = r.Header.Get("bin")
	if bin == "" {
		bin = h.dao.Bin().GenerateId()
		slog.Debug("auto generated bin", "bin", bin)
	}
	return bin, filename, nil
}

// checkFileExtension rejects file names with a configured illegal extension.
func (h *HTTP) checkFileExtension(filename string) error {
	thisExtension := path.Ext(filename)
	if len(thisExtension) == 0 {
		return nil
	}
	for _, extension := range h.config.RejectFileExtensions {
		if "."+extension == thisExtension {
			return &httpError{status: http.StatusForbidden, message: "Illegal file extension"}
		}
	}
	return nil
}

// getOrCreateBin returns the bin with the given id, creating it if it does
// not exist yet.
func (h *HTTP) getOrCreateBin(id string) (ds.Bin, error) {
	bin, found, err := h.dao.Bin().GetByID(id)
	if err != nil {
		return bin, fmt.Errorf("select bin %q: %w", id, err)
	}
	if found {
		return bin, nil
	}

	// Bin does not exist, so create it here
	bin = ds.Bin{}
	bin.Id = id

	// Since manual approval is not needed, then just set the approval time at the time of the upload
	if !h.config.RequireApproval {
		now := time.Now().UTC().Truncate(time.Microsecond)
		_ = bin.ApprovedAt.Scan(now)
	}

	// Abort early if the bin is invalid
	if err := h.dao.Bin().ValidateInput(&bin); err != nil {
		return bin, &httpError{status: http.StatusBadRequest, message: err.Error()}
	}

	bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
	inserted, err := h.dao.Bin().Insert(&bin)
	if err != nil {
		return bin, fmt.Errorf("insert bin %q: %w", id, err)
	}
	bin, found, err = h.dao.Bin().GetByID(id)
	if err != nil {
		return bin, fmt.Errorf("select bin %q after insert: %w", id, err)
	}
	if !found {
		return bin, fmt.Errorf("bin %q not found after insert", id)
	}
	if inserted {
		// TODO: Execute new bin created trigger
		h.metrics.IncrNewBinCount()
	}
	return bin, nil
}

// checkBinWritable rejects uploads to bins that are expired, deleted or
// locked.
func checkBinWritable(bin ds.Bin) error {
	if bin.IsExpired() || bin.IsDeleted() {
		return &httpError{status: http.StatusMethodNotAllowed, message: "The bin is no longer available"}
	}
	if bin.Readonly {
		return &httpError{
			status:  http.StatusMethodNotAllowed,
			message: "Uploads to locked bins are not allowed",
			headers: map[string]string{"Allow": "GET, HEAD"},
		}
	}
	return nil
}

// checkStorageLimit rejects the upload if the configured storage limit is
// reached. A limit of 0 disables the check.
func (h *HTTP) checkStorageLimit() error {
	if h.config.LimitStorageBytes == 0 {
		return nil
	}
	totalBytesConsumed := h.getCachedStorageBytes()
	if totalBytesConsumed >= h.config.LimitStorageBytes {
		return &httpError{
			status:  http.StatusInsufficientStorage,
			message: "Insufficient storage, please retry later",
			err:     fmt.Errorf("currently consuming %s", humanize.Bytes(totalBytesConsumed)),
		}
	}
	return nil
}

// bufferUpload writes the request body to a temporary file in the workspace
// while feeding it through the digest, so the checksums are calculated
// during the initial write to reduce disk IOPS. The body must be exactly
// expectedBytes long and not empty. On success the caller owns the returned
// file and must close and remove it. The file offset is at the end of the
// content when returned.
func (h *HTTP) bufferUpload(r *http.Request, digest *contentDigest, expectedBytes uint64, started time.Time) (*os.File, error) {
	// Add timestamp to the temporary file to make it easy to see when
	// an upload was started.
	fp, err := h.workspace.CreateTempFile(expectedBytes, fmt.Sprintf("filebin-%s-", started.Format("20060102-150405")))
	if err != nil {
		return nil, fmt.Errorf("create temporary upload file: %w", err)
	}
	discard := func() {
		_ = fp.Close()
		_ = os.Remove(fp.Name())
	}

	nBytes, err := io.Copy(io.MultiWriter(fp, digest.Writer()), r.Body)
	if err != nil {
		discard()
		err = fmt.Errorf("upload aborted at %s of %s, started %s: %w", humanize.Bytes(uint64(nBytes)), humanize.Bytes(expectedBytes), humanize.Time(started), err)
		// A truncated body or a closed connection means the client went
		// away. Report it as a client error so it is not logged as a
		// server failure.
		if r.Context().Err() != nil || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, &httpError{status: http.StatusBadRequest, message: "Upload aborted", err: err}
		}
		return nil, err
	}
	if uint64(nBytes) != expectedBytes {
		discard()
		return nil, &httpError{
			status:  http.StatusBadRequest,
			message: "Content-length did not match the request body length",
			err:     fmt.Errorf("got %d bytes, expected %d", nBytes, expectedBytes),
		}
	}
	if nBytes == 0 {
		discard()
		return nil, &httpError{status: http.StatusBadRequest, message: "Empty file uploads are not allowed"}
	}
	return fp, nil
}

// verifyChecksums compares the calculated checksums with the ones the client
// provided in the Content-MD5 and Content-SHA256 request headers, if any.
func verifyChecksums(r *http.Request, digest *contentDigest) error {
	if expected := r.Header.Get("Content-MD5"); expected != "" {
		if got := digest.MD5(); got != expected {
			return &httpError{
				status:  http.StatusBadRequest,
				message: "MD5 checksum did not match",
				err:     fmt.Errorf("client sent %s, calculated %s", expected, got),
			}
		}
	}
	if expected := r.Header.Get("Content-SHA256"); expected != "" {
		if got := digest.SHA256(); got != expected {
			return &httpError{
				status:  http.StatusBadRequest,
				message: "SHA256 checksum did not match",
				err:     fmt.Errorf("client sent %s, calculated %s", expected, got),
			}
		}
	}
	return nil
}

// inspectContent detects the mime type of the buffered content and, for
// images, calculates the perceptual hash. A phash failure is logged and
// leaves the phash empty. The file offset is at the start of the content
// when returned.
func inspectContent(fp *os.File, filename string) (mime string, pHash string, pHashDuration time.Duration, err error) {
	_, _ = fp.Seek(0, 0)
	detected, err := mimetype.DetectReader(fp)
	if err != nil {
		return "", "", 0, fmt.Errorf("detect mime type: %w", err)
	}
	mime = detected.String()
	_, _ = fp.Seek(0, 0)

	if strings.HasPrefix(mime, "image/") {
		tPhash := time.Now()
		pHash, err = phash.Compute(fp)
		pHashDuration = time.Since(tPhash)
		if err != nil {
			slog.Warn("failed to compute phash", "filename", filename, "error", err)
		}
		_, _ = fp.Seek(0, 0)
	}
	return mime, pHash, pHashDuration, nil
}

// lookupExistingContent returns the file_content record for the checksum if
// one exists, or nil if the content has not been seen before. Uploads of
// blocked content are rejected. A database failure is returned as an error
// rather than treated as not found, so that blocked content cannot slip
// through while the database is unavailable.
func (h *HTTP) lookupExistingContent(sha256 string) (*ds.FileContent, error) {
	existing, found, err := h.dao.FileContent().GetBySHA256(sha256)
	if err != nil {
		return nil, fmt.Errorf("select file_content %s: %w", sha256, err)
	}
	if !found {
		return nil, nil
	}
	if existing.Blocked {
		return nil, &httpError{
			status:  http.StatusForbidden,
			message: "This content has been blocked and cannot be uploaded",
			err:     fmt.Errorf("content %s is blocked", sha256),
		}
	}
	return existing, nil
}

// storeContent uploads the buffered content to S3 under its SHA256 key,
// making up to attempts tries before giving up.
func (h *HTTP) storeContent(sha256 string, fp *os.File, size int64, attempts int) error {
	h.metrics.IncrStorageUploadInProgress()
	defer h.metrics.DecrStorageUploadInProgress()

	for attempt := 1; ; attempt++ {
		_, _ = fp.Seek(0, 0)
		err := h.s3.PutObjectByHash(sha256, fp, size)
		if err == nil {
			return nil
		}
		if attempt >= attempts {
			return fmt.Errorf("gave up uploading %s to S3 after %d attempts: %w", sha256, attempt, err)
		}
		slog.Warn("failed attempt to upload to S3, retrying", "attempt", attempt, "max_attempts", attempts, "error", err)

		// Sleep a little before retrying
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
}

// persistFile inserts the file reference, or updates it if found is true.
// If the insert loses a race against a concurrent upload of the same
// filename, the existing row is updated instead and file is replaced with
// it.
func (h *HTTP) persistFile(file *ds.File, found bool) error {
	if found {
		if err := h.dao.File().Update(file); err != nil {
			return fmt.Errorf("update file %d: %w", file.Id, err)
		}
		return nil
	}

	inserted, err := h.dao.File().Insert(file)
	if err != nil {
		return fmt.Errorf("insert file: %w", err)
	}
	if inserted {
		// TODO: Execute new file created trigger
		return nil
	}

	// A concurrent upload of the same filename already inserted the row.
	// Fetch the existing file and update it instead.
	existing, _, err := h.dao.File().GetByName(file.Bin, file.Filename)
	if err != nil {
		return fmt.Errorf("select file after insert conflict: %w", err)
	}
	existing.SHA256 = file.SHA256
	existing.Bytes = file.Bytes
	existing.Mime = file.Mime
	existing.MD5 = file.MD5
	existing.Updates = existing.Updates + 1
	existing.IP = file.IP
	existing.Headers = file.Headers
	existing.UploadDurationMs = file.UploadDurationMs
	_ = existing.DeletedAt.Scan(nil)
	if err := h.dao.File().Update(&existing); err != nil {
		return fmt.Errorf("update file %d after insert conflict: %w", existing.Id, err)
	}
	*file = existing
	return nil
}

// runPostUploadHook executes the post-upload hook if one is configured and
// returns how long it took. The hook runs after the upload has been
// persisted and is treated as a notification: its exit code and output are
// logged but do not affect the response to the client.
func (h *HTTP) runPostUploadHook(ctx context.Context, file ds.File) time.Duration {
	if h.config.PostUploadHook == "" {
		return 0
	}
	hookStart := time.Now()
	hookCtx, hookCancel := context.WithTimeout(ctx, h.config.PostUploadHookTimeout)
	defer hookCancel()
	hookCmd := exec.CommandContext(hookCtx, h.config.PostUploadHook,
		"--bin-id", file.Bin,
		"--filename", file.Filename,
		"--content-type", file.Mime,
		"--size", strconv.FormatUint(file.Bytes, 10),
		"--sha256", file.SHA256,
	)
	hookOutput, hookErr := hookCmd.CombinedOutput()
	hookDuration := time.Since(hookStart)
	if hookErr != nil {
		slog.Warn("post-upload hook returned an error", "filename", file.Filename, "bin", file.Bin, "error", hookErr, "output", strings.TrimRight(string(hookOutput), "\n"))
	}
	return hookDuration
}

func (h *HTTP) deleteFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]
	inputFilename := params["filename"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 110", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "The bin does not exist", http.StatusNotFound)
		return
	}

	if !bin.IsReadable() {
		h.Error(w, r, "", "The bin is no longer available", 140, http.StatusNotFound)
		return
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		slog.Error("unable to get file by name", "bin", inputBin, "filename", inputFilename, "error", err)
		http.Error(w, "Errno 111", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}

	// No need to delete the file twice
	if !file.IsReadable() {
		http.Error(w, "This file is no longer available", http.StatusNotFound)
		return
	}

	// Flag as deleted
	now := time.Now().UTC().Truncate(time.Microsecond)
	_ = file.DeletedAt.Scan(now)

	if err := h.dao.File().Update(&file); err != nil {
		slog.Error("unable to update the file", "bin", inputBin, "filename", inputFilename, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Note: File content cleanup is handled by lurker using COUNT(*) to find orphaned content

	// Update the updated timestamp of the bin. The guarded update does not
	// touch expiration or moderation fields, so it cannot revert concurrent
	// changes. The file is already deleted at this point, so a bin that was
	// deleted concurrently is still a success.
	updated, err := h.dao.Bin().TouchUpdatedAt(&bin)
	if err != nil {
		slog.Error("unable to update bin timestamp after file deletion", "bin", bin.Id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !updated {
		slog.Debug("bin was deleted during file deletion", "bin", bin.Id)
	}

	h.metrics.IncrFileDeleteCount()
	http.Error(w, "File deleted successfully", http.StatusOK)
}
