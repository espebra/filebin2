package main

import (
	"fmt"
	"io"
	"net/http"
	//"encoding/json"
	"crypto/md5"
	"crypto/sha256"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
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

	bin, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	file, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		fmt.Printf("Unable to GetByName(%s, %s): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	fp, err := h.s3.GetObject(inputBin, inputFilename, file.Nonce)
	if err != nil {
		fmt.Printf("Unable to get object: %s\n", err.Error())
		http.Error(w, "Errno 5", http.StatusGone)
		return
	}

	w.Header().Set("Expires", bin.Expiration.Format(http.TimeFormat))
	w.Header().Set("Last-Modified", file.Updated.Format(http.TimeFormat))
	w.Header().Set("Bin", file.Bin)
	w.Header().Set("Filename", file.Filename)
	//w.Header().Set("Cache-Control", "s-maxage=1")

	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Bytes))

	if file.MD5 != "" {
		w.Header().Set("Content-MD5", file.MD5)
	}

	if file.SHA256 != "" {
		w.Header().Set("Content-SHA256", file.SHA256)
	}

	if file.Mime != "" {
		w.Header().Set("Content-Type", file.Mime)
	}

	// Handling of specific content-types
	if strings.HasPrefix(file.Mime, "video") {
		w.Header().Set("Content-Disposition", "inline")
	} else if strings.HasPrefix(file.Mime, "image") {
		w.Header().Set("Content-Disposition", "inline")
	} else if strings.HasPrefix(file.Mime, "text/plain") {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// Tell the client to tread any other content types as attachment
		w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
	}

	if err := h.dao.File().RegisterDownload(&file); err != nil {
		fmt.Printf("Unable to update file %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
	}

	fmt.Printf("Downloaded file %s (%s) from bin %s in %.3fs (%d downloads)\n", inputFilename, humanize.Bytes(file.Bytes), inputBin, time.Since(t0).Seconds(), file.Downloads)

	if _, err = io.Copy(w, fp); err != nil {
		fmt.Printf("The client cancelled the download: %s\n", err.Error())
		//http.Error(w, "Errno 4", http.StatusInternalServerError)
		return
	}

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

	if err := h.dao.Bin().Update(bin); err != nil {
		fmt.Printf("Unable to update bin %s: %s\n", inputBin, err.Error())
		http.Error(w, "Errno 5", http.StatusInternalServerError)
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

	t1 := time.Now()

	nBytes, err := io.Copy(fp, r.Body)
	if err != nil {
		fmt.Printf("Error occurred during io.Copy: %s\n", err.Error())
		return
	}
	fp.Seek(0, 0)

	t2 := time.Now()

	checksum := md5.New()
	if _, err := io.Copy(checksum, fp); err != nil {
		fmt.Printf("Error during checksum: %s\n", err.Error())
		return
	}
	file.MD5 = fmt.Sprintf("%x", checksum.Sum(nil))
	fp.Seek(0, 0)

	checksum = sha256.New()
	if _, err := io.Copy(checksum, fp); err != nil {
		fmt.Printf("Error during checksum: %s\n", err.Error())
		return
	}
	file.SHA256 = fmt.Sprintf("%x", checksum.Sum(nil))
	fp.Seek(0, 0)

	mime, err := mimetype.DetectReader(fp)
	if err != nil {
		fmt.Printf("Unable to detect mime type on filename %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
		http.Error(w, "Errno 4", http.StatusInternalServerError)
		return
	}
	file.Mime = mime.String()
	fp.Seek(0, 0)

	t3 := time.Now()

	//fmt.Printf("Buffered %s to %s in %.3fs\n", humanize.Bytes(nBytes), fp.Name(), time.Since(start).Seconds())

	nonce, err := h.s3.PutObject(file.Bin, file.Filename, fp, nBytes)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	t4 := time.Now()

	file.Nonce = nonce
	if err := h.dao.File().Update(file); err != nil {
		fmt.Printf("Unable to update file %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
		http.Error(w, "Errno 3", http.StatusInternalServerError)
		return
	}

	//fmt.Printf("Uploaded %s to S3 in %.3fs\n", humanize.Bytes(nBytes), time.Since(start).Seconds())
	fmt.Printf("Uploaded filename %s (%s) to bin %s (db in %.3fs, buffered in %.3fs, checksum in %.3fs, stored in %.3fs, total %.3fs)\n", inputFilename, humanize.Bytes(inputBytes), inputBin, t1.Sub(t0).Seconds(), t2.Sub(t1).Seconds(), t3.Sub(t2).Seconds(), t4.Sub(t3).Seconds(), t4.Sub(t0).Seconds())

	w.WriteHeader(http.StatusCreated)
}
