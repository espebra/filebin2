package main

import (
	"fmt"
	"io"
	"io/ioutil"
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
	}
	var data Data

	if err := h.templates.ExecuteTemplate(w, "index", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
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
		fmt.Printf("Unable to parse the content-length request header: %s\n", err.Error())
	}
	// TODO: Input validation on content-length. Between min:max.

	// TODO: Check if bin exists and is writable

	// If bin does not exist, create it
	bin := &ds.Bin{}
	bin.Id = inputBin
	if err := h.dao.Bin().Upsert(bin); err != nil {
		fmt.Printf("Unable to load bin %s: %s\n", inputBin, err.Error())
		http.Error(w, "Errno 2", http.StatusInternalServerError)
		return
	}

	// TODO: Can files be overwritten?
	file := &ds.File{}
	file.Bin = bin.Id
	file.Filename = inputFilename
	file.Size = inputBytes
	if err := h.dao.File().Upsert(file); err != nil {
		fmt.Printf("Unable to load file %s: %s\n", inputBin, err.Error())
		http.Error(w, "Errno 3", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Uploading filename %s (%s) to bin %s\n", inputFilename, humanize.Bytes(uint64(inputBytes)), inputBin)

	fp, err := ioutil.TempFile(os.TempDir(), "filebin")
	// Defer removal of the tempfile to clean up partially uploaded files.
	defer os.Remove(fp.Name())
	defer fp.Close()

	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return
	}

	nBytes, err := io.Copy(fp, r.Body)
	if err != nil {
		fmt.Printf("Error occurred during io.Copy: %s\n", err.Error())
		return
	}
	d1 := time.Since(start)
	fp.Seek(0, 0)

	//fmt.Printf("Buffered %s to %s in %.3fs\n", humanize.Bytes(uint64(nBytes)), fp.Name(), time.Since(start).Seconds())

	err = h.s3.PutObject(file.Bin, file.Filename, fp, nBytes)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	d2 := time.Since(start)
	//fmt.Printf("Uploaded %s to S3 in %.3fs\n", humanize.Bytes(uint64(nBytes)), time.Since(start).Seconds())
	fmt.Printf("Uploaded filename %s (%s) to bin %s (buffered in %.3fs, stored in %.3fs)\n", inputFilename, humanize.Bytes(uint64(inputBytes)), inputBin, d1.Seconds(), d2.Seconds())
}
