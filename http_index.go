package main

import (
	"fmt"
	"github.com/espebra/filebin2/ds"
	"net/http"
	"time"
)

func (h *HTTP) Index(w http.ResponseWriter, r *http.Request) {

	type Data struct {
		Bin ds.Bin `json:"bin"`
	}
	var data Data

	bin := &ds.Bin{}
	bin.Expiration = time.Now().UTC().Add(h.expirationDuration)
	err := h.dao.Bin().Insert(bin)
	if err != nil {
		fmt.Printf("Unable to insert new bin: %s\n", err.Error())
		http.Error(w, "Errno 301", http.StatusInternalServerError)
		return
	}
	data.Bin = *bin

	if err := h.templates.ExecuteTemplate(w, "index", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) About(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Cache-Control", "max-age=900")
	if err := h.templates.ExecuteTemplate(w, "about", nil); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) API(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		Bin ds.Bin `json:"bin"`
	}
	var data Data

	//w.Header().Set("Cache-Control", "max-age=900")
	if err := h.templates.ExecuteTemplate(w, "api", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) APISpec(w http.ResponseWriter, r *http.Request) {

	type Data struct {
		Bin ds.Bin `json:"bin"`
	}
	var data Data

	w.Header().Set("Content-Type", "application/json")
	//w.Header().Set("Cache-Control", "max-age=900")
	if err := h.templates.ExecuteTemplate(w, "apispec", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}
