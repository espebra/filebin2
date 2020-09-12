package main

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/espebra/filebin2/ds"
	"github.com/gorilla/mux"
)

func (h *HTTP) ViewBin(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]

	type Data struct {
		ds.Common
		Bin   ds.Bin    `json:"bin"`
		Files []ds.File `json:"files"`
	}
	var data Data
	data.Page = "bin"

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}
	if found == false {
		bin.Id = inputBin
		bin.ExpiredAt = time.Now().UTC().Add(h.expirationDuration)
		err := h.dao.Bin().Insert(&bin)
		if err != nil {
			fmt.Printf("Unable to insert new bin: %s\n", err.Error())
			http.Error(w, "Errno 301", http.StatusInternalServerError)
			return
		}
	}
	data.Bin = bin

	if bin.IsReadable() == false {
		h.Error(w, r, "", fmt.Sprintf("The bin %s is no longer available.", inputBin), 202, http.StatusNotFound)
		return
	}

	files, err := h.dao.File().GetByBin(inputBin, false)
	if err != nil {
		fmt.Printf("Unable to GetByBin(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
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
		if err := h.templates.ExecuteTemplate(w, "bin", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) Archive(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]
	inputFormat := params["format"]

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}
	if found == false {
		h.Error(w, r, "", fmt.Sprintf("The bin %s does not exist.", inputBin), 201, http.StatusNotFound)
		return
	}

	if bin.IsReadable() == false {
		h.Error(w, r, "", fmt.Sprintf("The bin %s is no longer available.", inputBin), 202, http.StatusNotFound)
		return
	}

	files, err := h.dao.File().GetByBin(inputBin, false)
	if err != nil {
		fmt.Printf("Unable to GetByBin(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if inputFormat == "zip" {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+bin.Id+`.zip"`)
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

			fp, err := h.s3.GetObject(bin.Id, file.Filename, file.Nonce, 0, 0)
			if err != nil {
				h.Error(w, r, fmt.Sprintf("Failed to archive object in bin %s: filename %s: %s", bin.Id, file.Filename, err.Error()), "Archive error", 300, http.StatusInternalServerError)
				return
			}

			bytes, err := io.Copy(ze, fp)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("Added %d bytes to the zip archive\n", bytes)
		}
		if err := zw.Close(); err != nil {
			fmt.Println(err)
		}
		return
	} else if inputFormat == "tar" {
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Disposition", `attachment; filename="`+bin.Id+`.tar"`)
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

			fp, err := h.s3.GetObject(bin.Id, file.Filename, file.Nonce, 0, 0)
			if err != nil {
				h.Error(w, r, fmt.Sprintf("Failed to archive object in bin %s: filename %s: %s", bin.Id, file.Filename, err.Error()), "Archive error", 300, http.StatusInternalServerError)
				return
			}

			bytes, err := io.Copy(tw, fp)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("Added %d bytes to the tar archive\n", bytes)
		}
		if err := tw.Close(); err != nil {
			fmt.Println(err)
		}
		return
	} else {
		http.Error(w, "Supported formats: zip and tar", http.StatusNotFound)
		return
	}
}

func (h *HTTP) DeleteBin(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]
	now := time.Now().UTC().Truncate(time.Microsecond)

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
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

	// Set to pending delete
	bin.Hidden = true
	bin.DeletedAt = now
	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Error(w, "Bin deleted successfully ", http.StatusOK)
	return
}

func (h *HTTP) LockBin(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
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
