package main

import (
	"fmt"
	"io"
	"net/http"
	//"encoding/json"
	"crypto/sha256"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/espebra/filebin2/ds"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gorilla/mux"
)

func (h *HTTP) GetFile(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	params := mux.Vars(r)
	inputBin := params["bin"]
	// TODO: Input validation (inputBin)
	inputFilename := params["filename"]
	// TODO: Input validation (inputFilename)

	file, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		fmt.Printf("Unable to GetByName(%s, %s): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	fp, err := h.s3.GetObject(inputBin, inputFilename)
	if err != nil {
		fmt.Printf("Unable to get object: %s\n", err.Error())
		http.Error(w, "Errno 5", http.StatusGone)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Bytes))

	if file.Checksum != "" {
		w.Header().Set("Content-SHA256", file.Checksum)
	}

	if file.Mime != "" {
		w.Header().Set("Content-Type", file.Mime)
	}

	if _, err = io.Copy(w, fp); err != nil {
		fmt.Println("Error during copy: %s\n", err.Error())
		http.Error(w, "Errno 4", http.StatusInternalServerError)
		return
	}

	if err := h.dao.File().RegisterDownload(&file); err != nil {
		fmt.Printf("Unable to update file %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
	}

	fmt.Printf("Downloaded file %s (%s) from bin %s in %.3fs (%d downloads)\n", inputFilename, humanize.Bytes(file.Bytes), inputBin, time.Since(t0).Seconds(), file.Downloads)

	//buf := new(bytes.Buffer)
	//buf.ReadFrom(fp)
	//s := buf.String()
	//if content != s {
	//        t.Errorf("Invalid content from get object. Expected %s, got %s\n", content, s)
	//}
}

func (h *HTTP) Upload(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()

	inputBin := r.Header.Get("bin")
	if inputBin == "" {
		// TODO: Generate random bin
	}
	// TODO: Input validation (inputBin)

	inputFilename := r.Header.Get("filename")
	// TODO: Input sanitize (inputFilename)

	inputBytes, err := strconv.ParseUint(r.Header.Get("content-length"), 10, 64)
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
	file.Bytes = inputBytes
	if err := h.dao.File().Upsert(file); err != nil {
		fmt.Printf("Unable to load file %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
		http.Error(w, "Errno 3", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Uploading filename %s (%s) to bin %s\n", inputFilename, humanize.Bytes(inputBytes), inputBin)

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
	fp.Seek(0, 0)

	t1 := time.Now()

	checksum := sha256.New()
	if _, err := io.Copy(checksum, fp); err != nil {
		fmt.Printf("Error during checksum: %s\n", err.Error())
		return
	}
	file.Checksum = fmt.Sprintf("%x", checksum.Sum(nil))
	fp.Seek(0, 0)

	mime, err := mimetype.DetectReader(fp)
	if err != nil {
		fmt.Printf("Unable to detect mime type on filename %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
		http.Error(w, "Errno 4", http.StatusInternalServerError)
		return
	}
	file.Mime = mime.String()
	fp.Seek(0, 0)

	if err := h.dao.File().Update(file); err != nil {
		fmt.Printf("Unable to update file %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
		http.Error(w, "Errno 3", http.StatusInternalServerError)
		return
	}

	t2 := time.Now()

	//fmt.Printf("Buffered %s to %s in %.3fs\n", humanize.Bytes(nBytes), fp.Name(), time.Since(start).Seconds())

	err = h.s3.PutObject(file.Bin, file.Filename, fp, nBytes)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	t3 := time.Now()

	//fmt.Printf("Uploaded %s to S3 in %.3fs\n", humanize.Bytes(nBytes), time.Since(start).Seconds())
	fmt.Printf("Uploaded filename %s (%s) to bin %s (buffered in %.3fs, checksum in %.3fs, stored in %.3fs)\n", inputFilename, humanize.Bytes(inputBytes), inputBin, t1.Sub(t0).Seconds(), t2.Sub(t1).Seconds(), t3.Sub(t2).Seconds())
}
