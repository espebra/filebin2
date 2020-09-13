package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	//"time"

	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/s3"
	//"github.com/gorilla/mux"
)

func (h *HTTP) ViewAdminDashboard(w http.ResponseWriter, r *http.Request) {
	//params := mux.Vars(r)
	//inputBin := params["bin"]

	type Bins struct {
		Available []ds.Bin `json:"available"`
	}

	type Data struct {
		Bins Bins `json:"bins"`
		//Files []ds.File `json:"files"`
		BucketInfo s3.BucketInfo `json:"bucketinfo"`
		Page       string        `json:"page"`
	}
	var data Data
	data.Page = "about"
	data.BucketInfo = h.s3.GetBucketInfo()

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
		if err := h.templates.ExecuteTemplate(w, "dashboard", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}
