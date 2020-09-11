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
		PendingDelete []ds.Bin `json:"pending_delete"`
		Available     []ds.Bin `json:"available"`
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

	binsPendingDelete, err := h.dao.Bin().GetAll(true)
	if err != nil {
		fmt.Printf("Unable to GetAll(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	binsAvailable, err := h.dao.Bin().GetAll(false)
	if err != nil {
		fmt.Printf("Unable to GetAll(): %s\n", err.Error())
		http.Error(w, "Errno 200", http.StatusInternalServerError)
		return
	}

	var bins Bins
	bins.PendingDelete = binsPendingDelete
	bins.Available = binsAvailable

	data.Bins = bins
	//if found == false {
	//	h.Error(w, r, "", fmt.Sprintf("The bin %s does not exist.", inputBin), 201, http.StatusNotFound)
	//	return
	//}
	//data.Bin = bin

	//if bin.IsReadable() == false {
	//	h.Error(w, r, "", fmt.Sprintf("The bin %s is no longer available.", inputBin), 202, http.StatusNotFound)
	//	return
	//}

	//files, err := h.dao.File().GetByBin(inputBin, 0)
	//if err != nil {
	//	fmt.Printf("Unable to GetByBin(%s): %s\n", inputBin, err.Error())
	//	http.Error(w, "Not found", http.StatusNotFound)
	//	return
	//}
	//data.Files = files

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
