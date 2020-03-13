package main

import (
	"log"
	"net/http"
)

func (h *HTTP) Index(w http.ResponseWriter, r *http.Request) {

	type Data struct {
		Filename string `json:"filename"`
	}

	var data Data

	if err := h.templates.ExecuteTemplate(w, "index", data); err != nil {
		log.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 1", http.StatusInternalServerError)
		return
	}
}
