package main

import (
	"fmt"
	"github.com/espebra/filebin2/ds"
	"net/http"
)

func (h *HTTP) Index(w http.ResponseWriter, r *http.Request) {

	type Data struct {
		Bin ds.Bin `json:"bin"`
	}
	var data Data

	bin := &ds.Bin{}
	err := h.dao.Bin().Insert(bin)
	if err != nil {
		fmt.Printf("Unable to insert new bin: %s\n", err.Error())
		http.Error(w, "Errno 5", http.StatusInternalServerError)
		return
	}
	data.Bin = *bin

	if err := h.templates.ExecuteTemplate(w, "index", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 1", http.StatusInternalServerError)
		return
	}
}
