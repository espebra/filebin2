package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
	//"path"
	"strconv"

	"github.com/espebra/filebin2/ds"

	//"github.com/aws/aws-sdk-go/aws"
	//"github.com/aws/aws-sdk-go/aws/credentials"
	//"github.com/aws/aws-sdk-go/aws/session"
	//"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dustin/go-humanize"
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
	start := time.Now()

	inputBin := r.Header.Get("bin")
	if inputBin == "" {
		// TODO: Generate random bin
	}
	// TODO: Input validation (inputBin)

	inputFilename := r.Header.Get("filename")
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

	fp, err := ioutil.TempFile(os.TempDir(), "filebin")
	// Defer removal of the tempfile to clean up partially uploaded files.
	defer os.Remove(fp.Name())
	defer fp.Close()

	if err != nil {
		log.Println(err)
		return
	}

	nBytes, err := io.Copy(fp, r.Body)
	if err != nil {
		log.Println("Error occurred during io.Copy: " + err.Error())
		return
	}
	fp.Seek(0, 0)

	log.Printf("Buffered %s to %s in %s\n", humanize.Bytes(uint64(nBytes)), fp.Name(), time.Since(start))

	err = h.s3.PutObject(file.Filename, fp, nBytes)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	log.Printf("Uploaded %s bytes to S3 in %s\n", humanize.Bytes(uint64(nBytes)), time.Since(start))
}
