package main

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
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
	http.Redirect(w, r, binURL.String(), 301)
}

func (h *HTTP) viewBin(w http.ResponseWriter, r *http.Request) {
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
	data.Page = "bin"
	data.Contact = h.config.Contact
	data.BaseUrl = h.config.BaseUrl.String()

	var binURL url.URL
	binURL.Scheme = h.config.BaseUrl.Scheme
	binURL.Host = h.config.BaseUrl.Host
	binURL.Path = path.Join(h.config.BaseUrl.Path, inputBin)
	data.BinUrl = binURL.String()

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}
	if found {
		files, err := h.dao.File().GetByBin(inputBin, true)
		if err != nil {
			fmt.Printf("Unable to GetByBin(%q): %s\n", inputBin, err.Error())
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if bin.IsReadable() {
			data.Files = files
		}
	} else {
		// Synthetize a bin without creating it. It will be created when a file is uploaded.
		bin = ds.Bin{}
		bin.Id = inputBin
		bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)

		// Intentional slowdown to make crawling less efficient
		time.Sleep(1 * time.Second)
	}

	data.Bin = bin

	code := 200
	if bin.IsReadable() == false {
		code = 404
	}

	h.metrics.IncrBinPageViewCount()

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			fmt.Printf("Failed to parse json: %s\n", err.Error())
			http.Error(w, "Errno 201", http.StatusInternalServerError)
			return
		}
		io.WriteString(w, string(out))
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(code)
		if err := h.templates.ExecuteTemplate(w, "bin", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
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
		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}
	if found {
		files, err := h.dao.File().GetByBin(inputBin, true)
		if err != nil {
			fmt.Printf("Unable to GetByBin(%q): %s\n", inputBin, err.Error())
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if bin.IsReadable() {
			data.Files = files
		}
	} else {
		// Synthetize a bin without creating it. It will be created when a file is uploaded.
		bin = ds.Bin{}
		bin.Id = inputBin
		bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
		bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)

		// Intentional slowdown to make crawling less efficient
		time.Sleep(1 * time.Second)
	}

	code := 200
	if bin.IsReadable() == false {
		code = 404
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	for _, file := range data.Files {
		var u url.URL
		u.Scheme = h.config.BaseUrl.Scheme
		u.Host = h.config.BaseUrl.Host
		u.Path = path.Join(h.config.BaseUrl.Path, file.URL)
		fmt.Fprintf(w, "%s\n", u.String())
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

	var q *qrcode.QRCode
	q, err := qrcode.New(binURL.String(), qrcode.Medium)
	if err != nil {
		fmt.Printf("Error generating qr code %s: %s\n", binURL.String(), err.Error())
		http.Error(w, "Unable to generate QR code", http.StatusInternalServerError)
		return
	}

	q.DisableBorder = true

	img := q.Image(size)
	if err != nil {
		fmt.Printf("Error generating qr code %s: %s\n", binURL.String(), err.Error())
		http.Error(w, "Unable to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, img); err != nil {
		fmt.Printf("Unable to write image: %s\n", err.Error())
		http.Error(w, "Unable to generate QR code", http.StatusInternalServerError)
	}
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
		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}
	if found == false {
		h.Error(w, r, "", "The bin does not exist.", 201, http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		h.Error(w, r, "", "This bin is no longer available.", 202, http.StatusNotFound)
		return
	}

	files, err := h.dao.File().GetByBin(inputBin, true)
	if err != nil {
		fmt.Printf("Unable to GetByBin(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if len(files) == 0 {
		// The bin does not contain any files
		http.Error(w, "Not found", http.StatusNotFound)
		return
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
			nextUrl.Path = path.Join(h.config.BaseUrl.Path, r.RequestURI)
			data.NextUrl = nextUrl.String()
			if err := h.templates.ExecuteTemplate(w, "cookie", data); err != nil {
				fmt.Printf("Failed to execute template: %s\n", err.Error())
				http.Error(w, "Errno 302", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	if inputFormat == "zip" {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", bin.Id))
		zw := zip.NewWriter(w)
		for _, file := range files {
			header := &zip.FileHeader{}
			header.Name = file.Filename
			header.Modified = file.UpdatedAt
			header.SetMode(400) // RW for the file owner

			ze, err := zw.CreateHeader(header)
			if err != nil {
				fmt.Println(err)
				return
			}

			fp, err := h.s3.GetObject(bin.Id, file.Filename, 0, 0)
			if err != nil {
				h.Error(w, r, fmt.Sprintf("Failed to archive object in bin %q: filename %q: %s", bin.Id, file.Filename, err.Error()), "Archive error", 300, http.StatusInternalServerError)
				return
			}
			defer fp.Close()
			h.metrics.IncrBytesStorageToFilebin(file.Bytes)

			bytes, err := io.Copy(ze, fp)
			if err != nil {
				fmt.Println(err)
			}
			h.metrics.IncrBytesFilebinToClient(uint64(bytes))
			fmt.Printf("Added file %q at %s (%d bytes) to the zip archive for bin %s\n", file.Filename, humanize.Bytes(uint64(bytes)), bytes, bin.Id)
		}
		if err := zw.Close(); err != nil {
			fmt.Println(err)
		}
		if err := h.dao.Bin().RegisterDownload(&bin); err != nil {
			fmt.Printf("Unable to update bin %q: %s\n", inputBin, err.Error())
		}
		h.metrics.IncrZipArchiveDownloadCount()
		return
	} else if inputFormat == "tar" {
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tar\"", bin.Id))
		tw := tar.NewWriter(w)
		for _, file := range files {
			header := &tar.Header{}
			header.Name = file.Filename
			header.Size = int64(file.Bytes)
			header.ModTime = file.UpdatedAt
			header.Mode = 0600 // rw access for the owner

			if err := tw.WriteHeader(header); err != nil {
				fmt.Println(err)
				return
			}

			fp, err := h.s3.GetObject(bin.Id, file.Filename, 0, 0)
			if err != nil {
				h.Error(w, r, fmt.Sprintf("Failed to archive object in bin %q: filename %q: %s", bin.Id, file.Filename, err.Error()), "Archive error", 300, http.StatusInternalServerError)
				return
			}
			defer fp.Close()
			h.metrics.IncrBytesStorageToFilebin(file.Bytes)

			bytes, err := io.Copy(tw, fp)
			if err != nil {
				fmt.Println(err)
			}
			h.metrics.IncrBytesFilebinToClient(uint64(bytes))
			fmt.Printf("Added file %q at %s (%d bytes) to the tar archive for bin %s\n", file.Filename, humanize.Bytes(uint64(bytes)), bytes, bin.Id)
		}
		if err := tw.Close(); err != nil {
			fmt.Println(err)
		}
		if err := h.dao.Bin().RegisterDownload(&bin); err != nil {
			fmt.Printf("Unable to update bin %q: %s\n", inputBin, err.Error())
		}
		h.metrics.IncrTarArchiveDownloadCount()
		return
	}
}

func (h *HTTP) deleteBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 204", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	// Set to deleted
	now := time.Now().UTC().Truncate(time.Microsecond)
	bin.DeletedAt.Scan(now)

	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.metrics.IncrBinDeleteCount()
	http.Error(w, "Bin deleted successfully ", http.StatusOK)
	return
}

func (h *HTTP) lockBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 205", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	// No need to set the bin to readonlytwice
	if bin.Readonly == true {
		http.Error(w, "This bin is already locked", http.StatusOK)
		return
	}

	// Set to read only
	bin.Readonly = true
	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.metrics.IncrBinLockCount()
	http.Error(w, "Bin locked successfully.", http.StatusOK)
	return
}

func (h *HTTP) approveBin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")

	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetByID(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 205", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	// No need to set the bin to approved twice
	if bin.IsApproved() {
		http.Error(w, "This bin is already approved", http.StatusOK)
		return
	}

	// Set bin as approved with the current timestamp
	now := time.Now().UTC().Truncate(time.Microsecond)
	bin.ApprovedAt.Scan(now)
	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.metrics.IncrBinDeleteCount()
	http.Error(w, "Bin approved successfully.", http.StatusOK)
	return
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

		fmt.Printf("Unable to GetByID(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 205", http.StatusInternalServerError)
		return
	}
	if found == false {
		// To make brute forcing a little bit less effective
		time.Sleep(3 * time.Second)

		http.Error(w, "Bin does not exist", http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		// To make brute forcing a little bit less effective
		time.Sleep(3 * time.Second)

		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	var IPs []string
	files, err := h.dao.File().GetByBin(inputBin, true)
	if err != nil {
		fmt.Printf("Unable to GetByBin(%q): %s\n", inputBin, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	for _, file := range files {
		IPs = append(IPs, file.IP)
	}
	if err := h.dao.Client().Ban(IPs, r.RemoteAddr); err != nil {
		fmt.Printf("Unable to ban client IPs(%v): %s\n", IPs, err.Error())
		http.Error(w, "Unable to ban clients", http.StatusInternalServerError)
		return
	}

	// Set to deleted
	now := time.Now().UTC().Truncate(time.Microsecond)
	bin.DeletedAt.Scan(now)
	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.metrics.IncrBinBanCount()
	http.Error(w, "Bin banned successfully.", http.StatusOK)
	return
}
