package web

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
	"github.com/gorilla/mux"

	qrcode "github.com/skip2/go-qrcode"
)

// This handler adds a trailing slash to bin URLs to make them possible
// to exclude from robots.txt
func (h *HTTP) viewBinRedirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")

	// Bins should never be indexed
	w.Header().Set("X-Robots-Tag", "noindex")

	params := mux.Vars(r)
	inputBin := params["bin"]

	var binURL url.URL
	binURL.Scheme = h.config.BaseUrl.Scheme
	binURL.Host = h.config.BaseUrl.Host
	binURL.Path = path.Join(h.config.BaseUrl.Path, inputBin)
	http.Redirect(w, r, binURL.String(), http.StatusMovedPermanently)
}

func (h *HTTP) viewBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")
	w.Header().Set("X-Robots-Tag", "noindex")

	params := mux.Vars(r)
	inputBin := params["bin"]

	type Data struct {
		ds.Common
		Bin         ds.Bin          `json:"bin"`
		Files       []ds.File       `json:"files"`
		SiteMessage *ds.SiteMessage `json:"site_message,omitempty"`
	}
	var data Data
	data.Page = "bin"
	data.Contact = h.config.Contact
	data.BaseUrl = h.config.BaseUrl.String()

	// Fetch site message if published for bin page
	h.siteMessageMutex.RLock()
	if h.siteMessage.IsPublishedBinPage() {
		message := h.siteMessage
		data.SiteMessage = &message
	}
	h.siteMessageMutex.RUnlock()

	var binURL url.URL
	binURL.Scheme = h.config.BaseUrl.Scheme
	binURL.Host = h.config.BaseUrl.Host
	binURL.Path = path.Join(h.config.BaseUrl.Path, inputBin)
	data.BinUrl = binURL.String()

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 262", http.StatusInternalServerError)
		return
	}
	if found {
		files, err := h.dao.File().GetByBin(inputBin, true)
		if err != nil {
			slog.Error("unable to get files by bin", "bin", inputBin, "error", err)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if bin.IsReadable() {
			data.Files = files
		}
	} else {
		// Synthesize a bin without creating it. It will be created when a file is uploaded.
		bin = ds.Bin{}
		bin.Id = inputBin
		bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)

		// Intentional slowdown to make crawling less efficient
		time.Sleep(1 * time.Second)
	}

	data.Bin = bin

	code := 200
	if !bin.IsReadable() {
		code = 404
	}

	h.metrics.IncrBinPageViewCount()

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			slog.Error("failed to parse json", "error", err)
			http.Error(w, "Errno 263", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(code)
		_, _ = w.Write(out)
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(code)
		if err := h.renderTemplate(w, "bin", data); err != nil {
			slog.Error("failed to execute template", "error", err)
			return
		}
	}
}

func (h *HTTP) viewBinPlainText(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")
	w.Header().Set("X-Robots-Tag", "noindex")

	params := mux.Vars(r)
	inputBin := params["bin"]

	type Data struct {
		ds.Common
		Bin   ds.Bin    `json:"bin"`
		Files []ds.File `json:"files"`
	}
	var data Data

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 264", http.StatusInternalServerError)
		return
	}
	if found {
		files, err := h.dao.File().GetByBin(inputBin, true)
		if err != nil {
			slog.Error("unable to get files by bin", "bin", inputBin, "error", err)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if bin.IsReadable() {
			data.Files = files
		}
	} else {
		// Synthesize a bin without creating it. It will be created when a file is uploaded.
		bin = ds.Bin{}
		bin.Id = inputBin
		bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)

		// Intentional slowdown to make crawling less efficient
		time.Sleep(1 * time.Second)
	}

	code := 200
	if !bin.IsReadable() {
		code = 404
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	for _, file := range data.Files {
		var u url.URL
		u.Scheme = h.config.BaseUrl.Scheme
		u.Host = h.config.BaseUrl.Host
		u.Path = path.Join(h.config.BaseUrl.Path, file.URL)
		_, _ = fmt.Fprintf(w, "%s\n", u.String())
	}
}

// viewBinSha256 lists the files in a bin along with their SHA256
// checksums in plain text, formatted like the output of sha256sum(1):
// the checksum, two spaces, then the filename, one file per line.
func (h *HTTP) viewBinSha256(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")
	w.Header().Set("X-Robots-Tag", "noindex")

	params := mux.Vars(r)
	inputBin := params["bin"]

	var files []ds.File

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 271", http.StatusInternalServerError)
		return
	}
	if found {
		fs, err := h.dao.File().GetByBin(inputBin, true)
		if err != nil {
			slog.Error("unable to get files by bin", "bin", inputBin, "error", err)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if bin.IsReadable() {
			files = fs
		}
	} else {
		// Synthesize a bin without creating it. It will be created when a file is uploaded.
		bin = ds.Bin{}
		bin.Id = inputBin
		bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)

		// Intentional slowdown to make crawling less efficient
		time.Sleep(1 * time.Second)
	}

	code := 200
	if !bin.IsReadable() {
		code = 404
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	for _, file := range files {
		_, _ = fmt.Fprintf(w, "%s  %s\n", file.SHA256, file.Filename)
	}
}

func (h *HTTP) binQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=31536000")
	w.Header().Set("X-Robots-Tag", "noindex")

	params := mux.Vars(r)
	inputBin := params["bin"]

	// Check if the thumbnail query parameter is set
	thumbnail := r.URL.Query().Has("thumbnail")

	size := 256
	if thumbnail {
		size = 80
	}

	var binURL url.URL
	binURL.Scheme = h.config.BaseUrl.Scheme
	binURL.Host = h.config.BaseUrl.Host
	binURL.Path = path.Join(h.config.BaseUrl.Path, inputBin)

	q, err := qrcode.New(binURL.String(), qrcode.Medium)
	if err != nil {
		slog.Error("error generating qr code", "url", binURL.String(), "error", err)
		http.Error(w, "Unable to generate QR code", http.StatusInternalServerError)
		return
	}

	q.DisableBorder = true

	img := q.Image(size)

	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, img); err != nil {
		slog.Error("unable to write image", "error", err)
	}
}

// archiveWriter provides a common interface for creating archive files
type archiveWriter interface {
	addFile(file ds.File) (io.Writer, error)
	close() error
}

// zipArchiveWriter wraps zip.Writer to implement archiveWriter
type zipArchiveWriter struct {
	writer *zip.Writer
}

func newZipArchiveWriter(w io.Writer) *zipArchiveWriter {
	return &zipArchiveWriter{writer: zip.NewWriter(w)}
}

func (z *zipArchiveWriter) addFile(file ds.File) (io.Writer, error) {
	header := &zip.FileHeader{}
	header.Name = file.Filename
	header.Modified = file.UpdatedAt
	header.SetMode(0600) // Read-write for the file owner
	return z.writer.CreateHeader(header)
}

func (z *zipArchiveWriter) close() error {
	return z.writer.Close()
}

// tarArchiveWriter wraps tar.Writer to implement archiveWriter
type tarArchiveWriter struct {
	writer *tar.Writer
}

func newTarArchiveWriter(w io.Writer) *tarArchiveWriter {
	return &tarArchiveWriter{writer: tar.NewWriter(w)}
}

func (t *tarArchiveWriter) addFile(file ds.File) (io.Writer, error) {
	header := &tar.Header{}
	header.Name = file.Filename
	header.Size = int64(file.Bytes)
	header.ModTime = file.UpdatedAt
	header.Mode = 0600 // rw access for the owner

	if err := t.writer.WriteHeader(header); err != nil {
		return nil, err
	}
	return t.writer, nil
}

func (t *tarArchiveWriter) close() error {
	return t.writer.Close()
}

// addFilesToArchive adds files from S3 to an archive writer. Once archive
// bytes have been written to the response, a failure can no longer be
// communicated with an error response: the status code is already sent, and
// writing an error page would corrupt the archive stream. Such failures
// abort the handler with http.ErrAbortHandler instead, which resets the
// connection so the client observes a failed transfer rather than a
// seemingly complete but corrupt archive.
func (h *HTTP) addFilesToArchive(w http.ResponseWriter, r *http.Request, bin ds.Bin, files []ds.File, archiver archiveWriter, format string) error {
	started := false
	for _, file := range files {
		// Fetch the object before writing the archive entry header, so a
		// failure on the first file is rejected with a proper error
		// response before any archive bytes are written.
		fp, err := h.s3.GetObject(file.SHA256, 0, 0)
		if err != nil {
			if !started {
				h.Error(w, r, fmt.Sprintf("Failed to archive object in bin %q: filename %q: %s", bin.Id, file.Filename, err.Error()), "Archive error", 300, http.StatusInternalServerError)
				return err
			}
			slog.Error("aborting archive, unable to get object mid-stream", "filename", file.Filename, "bin", bin.Id, "format", format, "error", err)
			panic(http.ErrAbortHandler)
		}

		writer, err := archiver.addFile(file)
		if err != nil {
			_ = fp.Close()
			slog.Error("aborting archive, unable to write entry header", "filename", file.Filename, "bin", bin.Id, "format", format, "error", err)
			panic(http.ErrAbortHandler)
		}
		started = true

		bytes, err := io.Copy(writer, fp)
		_ = fp.Close()
		if err != nil {
			slog.Error("aborting archive, unable to stream object", "filename", file.Filename, "bin", bin.Id, "format", format, "error", err)
			panic(http.ErrAbortHandler)
		}

		// Count the download and the transferred bytes only after the
		// object was streamed completely, so failed transfers do not
		// consume download credits.
		if err := h.dao.File().RegisterDownload(&file); err != nil {
			slog.Error("unable to increment download counter", "filename", file.Filename, "bin", bin.Id, "error", err)
		}
		h.metrics.IncrBytesStorageToFilebin(uint64(bytes))
		h.metrics.IncrBytesFilebinToClient(uint64(bytes))
		slog.Debug("added file to archive", "filename", file.Filename, "bytes", bytes, "format", format, "bin", bin.Id)
	}
	return archiver.close()
}

func (h *HTTP) archive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")
	w.Header().Set("X-Robots-Tag", "noindex")

	h.metrics.IncrArchiveDownloadInProgress()
	defer h.metrics.DecrArchiveDownloadInProgress()

	params := mux.Vars(r)
	inputBin := params["bin"]
	inputFormat := params["format"]

	if inputFormat != "zip" && inputFormat != "tar" {
		http.Error(w, "Supported formats: zip and tar", http.StatusNotFound)
		return
	}

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 265", http.StatusInternalServerError)
		return
	}
	if !found {
		h.Error(w, r, "", "The bin does not exist.", 266, http.StatusNotFound)
		return
	}

	if !bin.IsReadable() {
		h.Error(w, r, "", "This bin is no longer available.", 267, http.StatusNotFound)
		return
	}

	// If approvals are required, reject downloads from bins that are not approved
	if h.config.RequireApproval {
		if !bin.IsApproved() {
			h.Error(w, r, "", "This bin requires approval before files can be downloaded.", 522, http.StatusForbidden)
			return
		}
	}

	files, err := h.dao.File().GetByBin(inputBin, true)
	if err != nil {
		slog.Error("unable to get files by bin", "bin", inputBin, "error", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if len(files) == 0 {
		// The bin does not contain any files
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Filter out files that have exceeded the download limit
	if h.config.LimitFileDownloads > 0 {
		var allowed []ds.File
		for _, file := range files {
			if file.Downloads < h.config.LimitFileDownloads {
				allowed = append(allowed, file)
			}
		}
		files = allowed
		if len(files) == 0 {
			h.Error(w, r, "", "All files in this bin have exceeded the download limit.", 422, http.StatusForbidden)
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
				http.Error(w, "Errno 302", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	var archiver archiveWriter
	if inputFormat == "zip" {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", bin.Id+".zip"))
		archiver = newZipArchiveWriter(w)
	} else {
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", bin.Id+".tar"))
		archiver = newTarArchiveWriter(w)
	}

	if err := h.addFilesToArchive(w, r, bin, files, archiver, inputFormat); err != nil {
		slog.Error("failed to create archive", "bin", bin.Id, "format", inputFormat, "error", err)
		return
	}

	if err := h.dao.Bin().RegisterDownload(&bin); err != nil {
		slog.Error("unable to update bin", "bin", inputBin, "error", err)
	}

	if inputFormat == "zip" {
		h.metrics.IncrZipArchiveDownloadCount()
	} else {
		h.metrics.IncrTarArchiveDownloadCount()
	}
}

func (h *HTTP) deleteBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 204", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if !bin.IsReadable() {
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	// Set to deleted with a guarded update that cannot revert concurrent
	// changes to the other moderation fields.
	deleted, err := h.dao.Bin().MarkDeleted(&bin)
	if err != nil {
		slog.Error("unable to mark bin as deleted", "bin", bin.Id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !deleted {
		// The bin was deleted concurrently by another request.
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	h.metrics.IncrBinDeleteCount()
	http.Error(w, "Bin deleted successfully", http.StatusOK)
}

func (h *HTTP) lockBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 205", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if !bin.IsReadable() {
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	// No need to set the bin to readonly twice
	if bin.Readonly {
		http.Error(w, "This bin is already locked", http.StatusOK)
		return
	}

	// Set to read only with a guarded update that cannot resurrect a bin
	// that was deleted concurrently.
	locked, err := h.dao.Bin().Lock(&bin)
	if err != nil {
		slog.Error("unable to lock bin", "bin", bin.Id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !locked {
		// The bin was deleted concurrently by another request.
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	h.metrics.IncrBinLockCount()
	http.Error(w, "Bin locked successfully.", http.StatusOK)
}

func (h *HTTP) approveBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 206", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if !bin.IsReadable() {
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	// No need to set the bin to approved twice
	if bin.IsApproved() {
		http.Error(w, "This bin is already approved", http.StatusOK)
		return
	}

	// Set bin as approved with a guarded update that cannot revert
	// concurrent changes to the other moderation fields.
	approved, err := h.dao.Bin().Approve(&bin)
	if err != nil {
		slog.Error("unable to approve bin", "bin", bin.Id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !approved {
		// The bin was deleted concurrently.
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	http.Error(w, "Bin approved successfully.", http.StatusOK)
}

// Ban the client IP addresses that have uploaded files to the given bin
func (h *HTTP) banBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		// To make brute forcing a little bit less effective
		time.Sleep(3 * time.Second)

		slog.Error("unable to get bin by ID", "bin", inputBin, "error", err)
		http.Error(w, "Errno 207", http.StatusInternalServerError)
		return
	}
	if !found {
		// To make brute forcing a little bit less effective
		time.Sleep(3 * time.Second)

		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if !bin.IsReadable() {
		// To make brute forcing a little bit less effective
		time.Sleep(3 * time.Second)

		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	var IPs []string
	files, err := h.dao.File().GetByBin(inputBin, true)
	if err != nil {
		slog.Error("unable to get files by bin", "bin", inputBin, "error", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	for _, file := range files {
		IPs = append(IPs, file.IP)
	}
	if err := h.dao.Client().Ban(IPs, r.RemoteAddr); err != nil {
		slog.Error("unable to ban client IPs", "ips", IPs, "error", err)
		http.Error(w, "Unable to ban clients", http.StatusInternalServerError)
		return
	}

	// Set to deleted with a guarded update that cannot revert concurrent
	// changes to the other moderation fields. The clients are already
	// banned at this point, so a bin that was deleted concurrently is
	// still a success.
	deleted, err := h.dao.Bin().MarkDeleted(&bin)
	if err != nil {
		slog.Error("unable to mark banned bin as deleted", "bin", bin.Id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !deleted {
		slog.Debug("banned bin was already deleted", "bin", bin.Id)
	}

	h.metrics.IncrBinBanCount()
	http.Error(w, "Bin banned successfully.", http.StatusOK)
}
