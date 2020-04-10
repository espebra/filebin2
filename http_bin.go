package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/espebra/filebin2/ds"
	"github.com/gorilla/mux"
	//"github.com/dustin/go-humanize"
)

func (h *HTTP) ViewBin(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]

	type Data struct {
		Bin   ds.Bin    `json:"bin"`
		Files []ds.File `json:"files"`
	}
	var data Data

	bin, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	data.Bin = bin

	if bin.Deleted != 0 {
		http.Error(w, "This bin is no longer available", http.StatusGone)
		return
	}

	files, err := h.dao.File().GetByBin(inputBin)
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
			http.Error(w, "Errno 2", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, string(out))
	} else {
		if err := h.templates.ExecuteTemplate(w, "bin", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 1", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) DeleteBin(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]

	bin, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Bin not found", http.StatusNotFound)
		return
	}

	// No need to delete the bin twice
	if bin.Deleted > 0 {
		http.Error(w, "This bin is no longer available", http.StatusGone)
		return
	}

	// Set to pending delete
	bin.Deleted = 1
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

	bin, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Bin not found", http.StatusNotFound)
		return
	}

	// No need to set the bin to readonlytwice
	if bin.Readonly == true {
		http.Error(w, "This bin is already locked", http.StatusConflict)
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