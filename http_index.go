package main

import (
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"log"
	"net/http"
	"strconv"
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

func (h *HTTP) Upload(w http.ResponseWriter, r *http.Request) {
	inputBin := r.Header.Get("bin")
	if inputBin == "" {
		// TODO: Generate random bin
	}
	// TODO: Input validation (inputBin)

	inputFilename := r.Header.Get("inputFilename")
	// TODO: Input sanitize (inputFilename)
	inputBytes, err := strconv.Atoi(r.Header.Get("content-length"))
	if err != nil {
		log.Println("Unable to parse the content-length request header: ", err)
	}

	log.Println("Uploading " + humanize.Bytes(uint64(inputBytes)) + " to filename " + inputFilename + " in bin " + inputBin)

	bin := &ds.Bin{}
	bin.Id = inputBin

	if err := h.dao.Bin().Upsert(bin); err != nil {
		log.Printf("Unable to load bin %s: %s\n", inputBin, err.Error())
		http.Error(w, "Errno 2", http.StatusInternalServerError)
		return
	}

	log.Println("Found bin: ", bin)

	file := &ds.File{}
	file.BinId = bin.Id
	file.Filename = inputFilename
	file.Size = inputBytes

	if err := h.dao.File().Upsert(file); err != nil {
		log.Printf("Unable to load file %s: %s\n", inputBin, err.Error())
		http.Error(w, "Errno 3", http.StatusInternalServerError)
		return
	}

	log.Println("Found file: ", file)
	// Does bin exist?
	// No, create
	// Yes, allowed to write?
}
