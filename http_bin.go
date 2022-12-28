package main

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"fmt"
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
func (h *HTTP) ViewBinRedirect(w http.ResponseWriter, r *http.Request) {
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

func (h *HTTP) ViewBin(w http.ResponseWriter, r *http.Request) {
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

func (h *HTTP) BinQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")
	w.Header().Set("X-Robots-Tag", "noindex")

	params := mux.Vars(r)
	inputBin := params["bin"]

	var binURL url.URL
	binURL.Scheme = h.config.BaseUrl.Scheme
	binURL.Host = h.config.BaseUrl.Host
	binURL.Path = path.Join(h.config.BaseUrl.Path, inputBin)

	var png []byte
	png, err := qrcode.Encode(binURL.String(), qrcode.Medium, 256)
	if err != nil {
		fmt.Printf("Error generating qr code %s: %s\n", binURL.String(), err.Error())
		http.Error(w, "Unable to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")

	if _, err := w.Write(png); err != nil {
		fmt.Printf("Unable to write image: %s\n", err.Error())
		http.Error(w, "Unable to generate QR code", http.StatusInternalServerError)
	}
}

func (h *HTTP) Archive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0")
	w.Header().Set("X-Robots-Tag", "noindex")

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
		h.Error(w, r, "", "This bin is no longer available", 202, http.StatusNotFound)
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
				h.Error(w, r, fmt.Sprintf("Failed to archive object in bin %s: filename %s: %s", bin.Id, file.Filename, err.Error()), "Archive error", 300, http.StatusInternalServerError)
				return
			}

			bytes, err := io.Copy(ze, fp)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("Added file %s at %s (%d bytes) to the zip archive for bin %s\n", file.Filename, humanize.Bytes(uint64(bytes)), bytes, bin.Id)
		}
		if err := zw.Close(); err != nil {
			fmt.Println(err)
		}
		if err := h.dao.Bin().RegisterDownload(&bin); err != nil {
			fmt.Printf("Unable to update bin %q: %s\n", inputBin, err.Error())
		}
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
				h.Error(w, r, fmt.Sprintf("Failed to archive object in bin %s: filename %s: %s", bin.Id, file.Filename, err.Error()), "Archive error", 300, http.StatusInternalServerError)
				return
			}

			bytes, err := io.Copy(tw, fp)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("Added file %s at %s (%d bytes) to the tar archive for bin %s\n", file.Filename, humanize.Bytes(uint64(bytes)), bytes, bin.Id)
		}
		if err := tw.Close(); err != nil {
			fmt.Println(err)
		}
		if err := h.dao.Bin().RegisterDownload(&bin); err != nil {
			fmt.Printf("Unable to update bin %q: %s\n", inputBin, err.Error())
		}
		return
	}
}

func (h *HTTP) DeleteBin(w http.ResponseWriter, r *http.Request) {
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

	http.Error(w, "Bin deleted successfully ", http.StatusOK)
	return
}

func (h *HTTP) LockBin(w http.ResponseWriter, r *http.Request) {
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

	http.Error(w, "Bin locked successfully.", http.StatusOK)
	return
}

func (h *HTTP) ApproveBin(w http.ResponseWriter, r *http.Request) {
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

	http.Error(w, "Bin approved successfully.", http.StatusOK)
	return
}
