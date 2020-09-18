package main

import (
	"encoding/json"
	"fmt"
	"github.com/espebra/filebin2/ds"
	"io"
	"net/http"
	"time"
)

func (h *HTTP) Index(w http.ResponseWriter, r *http.Request) {

	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data

	data.Page = "front"

	bin := &ds.Bin{}
	bin.ExpiredAt = time.Now().UTC().Add(h.expirationDuration)
	bin.Id = h.dao.Bin().GenerateId()
	data.Bin = *bin

	if err := h.templates.ExecuteTemplate(w, "index", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) About(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Cache-Control", "max-age=900")
	type Data struct {
		ds.Common
	}
	var data Data
	data.Page = "about"

	if err := h.templates.ExecuteTemplate(w, "about", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) Privacy(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Cache-Control", "max-age=900")
	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "privacy"

	if err := h.templates.ExecuteTemplate(w, "privacy", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) Terms(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Cache-Control", "max-age=900")
	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "terms"

	if err := h.templates.ExecuteTemplate(w, "terms", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) API(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "api"

	//w.Header().Set("Cache-Control", "max-age=900")
	if err := h.templates.ExecuteTemplate(w, "api", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) APISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "api"

	w.Header().Set("Content-Type", "text/plain")
	//w.Header().Set("Cache-Control", "max-age=900")
	if err := h.templates.ExecuteTemplate(w, "apispec", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) FilebinStatus(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		AppStatus bool `json:"app-status"`
		DbStatus  bool `json:"db-status"`
		S3Status  bool `json:"s3-status"`
	}
	var data Data

	code := 200
	data.AppStatus = true
	if h.dao.Status() {
		data.DbStatus = true
	} else {
		data.DbStatus = false
		code = 503
	}

	if h.s3.Status() {
		data.S3Status = true
	} else {
		data.S3Status = false
		code = 503
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=1")
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Printf("Failed to parse json: %s\n", err.Error())
		http.Error(w, "Errno 201", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	io.WriteString(w, string(out))
}
