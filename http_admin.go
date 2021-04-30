package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strings"

	"github.com/espebra/filebin2/ds"
	"github.com/espebra/filebin2/s3"
)

func (h *HTTP) ViewAdminDashboard(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		//Bins Bins `json:"bins"`
		//Files []ds.File `json:"files"`
		BucketInfo s3.BucketInfo `json:"bucketinfo"`
		Page       string        `json:"page"`
		DBInfo     ds.Info       `json:"db_info"`
		Config     ds.Config     `json:"-"`
	}
	var data Data
	data.Config = *h.config
	data.Page = "about"
	data.BucketInfo = h.s3.GetBucketInfo()
	info, err := h.dao.Info().GetInfo()
	if err != nil {
		fmt.Printf("Unable to GetInfo(): %s\n", err.Error())
		http.Error(w, "Errno 326", http.StatusInternalServerError)
		return
	}
	data.DBInfo = info
	freeBytes := int64(h.config.LimitStorageBytes) - info.CurrentBytes
	if freeBytes < 0 {
		freeBytes = 0
	}
	data.DBInfo.FreeBytes = freeBytes
	data.DBInfo.FreeBytesReadable = humanize.Bytes(uint64(freeBytes))

	//binsAvailable, err := h.dao.Bin().GetAll()
	//if err != nil {
	//	fmt.Printf("Unable to GetAll(): %s\n", err.Error())
	//	http.Error(w, "Errno 200", http.StatusInternalServerError)
	//	return
	//}

	//var bins Bins
	//bins.Available = binsAvailable

	//data.Bins = bins

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
		if err := h.templates.ExecuteTemplate(w, "admin_dashboard", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) ViewAdminBins(w http.ResponseWriter, r *http.Request) {
	type Bins struct {
		Available []ds.Bin `json:"available"`
	}

	type Data struct {
		Bins Bins `json:"bins"`
		//Files []ds.File `json:"files"`
		BucketInfo s3.BucketInfo `json:"bucketinfo"`
		Page       string        `json:"page"`
		DBInfo     ds.Info       `json:"db_info"`
	}
	var data Data
	//data.Page = "about"
	//data.BucketInfo = h.s3.GetBucketInfo()
	//info, err := h.dao.Info().GetInfo()
	//if err != nil {
	//	fmt.Printf("Unable to GetInfo(): %s\n", err.Error())
	//	http.Error(w, "Errno 326", http.StatusInternalServerError)
	//	return
	//}
	//data.DBInfo = info

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
		if err := h.templates.ExecuteTemplate(w, "admin_bins", data); err != nil {
			fmt.Printf("Failed to execute template: %s\n", err.Error())
			http.Error(w, "Errno 203", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) ViewAdminLog(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		Transactions []ds.Transaction `json:"transactions"`
	}
	var data Data

	params := mux.Vars(r)
	inputCategory := params["category"]
	inputFilter := params["filter"]

	if inputCategory == "bin" {
		bin := inputFilter
		trs, err := h.dao.Transaction().GetByBin(bin)
		if err != nil {
			http.Error(w, "Errno 361", http.StatusInternalServerError)
			return
		}
		data.Transactions = trs
	} else if inputCategory == "ip" {
		ip := inputFilter
		trs, err := h.dao.Transaction().GetByIP(ip)
		if err != nil {
			http.Error(w, "Errno 361", http.StatusInternalServerError)
			return
		}
		data.Transactions = trs
	} else if inputCategory == "cid" {
		cid := inputFilter
		trs, err := h.dao.Transaction().GetByClientId(cid)
		if err != nil {
			http.Error(w, "Errno 361", http.StatusInternalServerError)
			return
		}
		data.Transactions = trs
	}

	if err := h.templates.ExecuteTemplate(w, "log", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 203", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) ViewAdminCleanup(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		Objects []string `json:"objects"`
		////Files []ds.File `json:"files"`
		//BucketInfo s3.BucketInfo `json:"bucketinfo"`
		//Page       string        `json:"page"`
		Bins []ds.Bin `json:"bins"`
	}

	objects, err := h.s3.ListObjects()
	if err != nil {
		http.Error(w, "Errno 262", http.StatusInternalServerError)
		return
	}

	bins, err := h.dao.Bin().GetAll()
	if err != nil {
		http.Error(w, "Errno 361", http.StatusInternalServerError)
		return
	}

	var allbins []string
	for _, bin := range bins {
		b := sha256.New()
		b.Write([]byte(bin.Id))
		hash := fmt.Sprintf("%x", b.Sum(nil))
		allbins = append(allbins, hash)
	}

	for _, object := range objects {
		splits := strings.Split(object, "/")
		if len(splits) == 2 {
			hash := splits[0]
			if inStringSlice(hash, allbins) {
				fmt.Printf("Match\n")
			} else {
				fmt.Printf("No match\n")
			}
		} else {
			fmt.Printf("No match. Weird length: %d\n", len(splits))
		}
	}

	var data Data
	data.Objects = objects
	data.Bins = bins

	if err := h.templates.ExecuteTemplate(w, "cleanup", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 203", http.StatusInternalServerError)
		return
	}
}
