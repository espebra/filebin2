package main

import (
	"net/http"
)

func (h *HTTP) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Foo", "bar")
	w.WriteHeader(http.StatusOK)
	return
}
