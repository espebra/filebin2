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
	// TODO: Input validation (inputBin)

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
