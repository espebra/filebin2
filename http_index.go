package main

import (
	"fmt"
	"net/http"
)

func (h *HTTP) Index(w http.ResponseWriter, r *http.Request) {

	type Data struct {
	}
	var data Data

	if err := h.templates.ExecuteTemplate(w, "index", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 1", http.StatusInternalServerError)
		return
	}
}
