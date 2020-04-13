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

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 112", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The bin does not exist", http.StatusNotFound)
		return
	}
	if bin.Deleted > 0 {
		http.Error(w, "The bin is no longer available", http.StatusNotFound)
		return
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		fmt.Printf("Unable to GetByName(%s, %s): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Errno 113", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}
	if file.Deleted > 0 {
		http.Error(w, "The file is no longer available", http.StatusNotFound)
		return
	}

	fp, err := h.s3.GetObject(inputBin, inputFilename, file.Nonce)
	if err != nil {
		fmt.Printf("Unable to get object: %s\n", err.Error())
		http.Error(w, "Errno 114", http.StatusInternalServerError)
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
	inputFilename := r.Header.Get("filename")
	inputMD5 := r.Header.Get("Content-MD5")
	inputSHA256 := r.Header.Get("Content-SHA256")
	inputBytes, err := strconv.ParseUint(r.Header.Get("content-length"), 10, 64)
	if err != nil {
		fmt.Printf("Unable to parse the content-length request header: %s\n", err.Error())
		http.Error(w, "content-length is required", http.StatusLengthRequired)
		return
	}
	// TODO: Input validation on content-length. Between min:max.

	// Check if bin exists
	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to load bin %s: %s\n", inputBin, err.Error())
		http.Error(w, "Errno 103", http.StatusInternalServerError)
		return
	}

	if found == false {
		// Bin does not exist, so create it here
		bin = ds.Bin{}
		bin.Id = inputBin
		if err := h.dao.Bin().Insert(&bin); err != nil {
			fmt.Printf("Unable to insert bin %s: %s\n", inputBin, err.Error())
			http.Error(w, "Errno 104", http.StatusInternalServerError)
			return
		}

		// TODO: Execute new bin created trigger
	}

	// Reject uploads to deleted bins
	if bin.Deleted > 0 {
		fmt.Printf("Rejected upload of filename %s to deleted bin %s\n", inputFilename, inputBin)
		http.Error(w, "Unable to upload file to bin that is no longer available", http.StatusMethodNotAllowed)
		return
	}

	// Reject uploads to readonly bins
	if bin.Readonly == true {
		fmt.Printf("Rejected upload of filename %s to readonly bin %s\n", inputFilename, inputBin)
		http.Error(w, "Unable to upload file to bin that is set to readonly", http.StatusMethodNotAllowed)
		return
	}

	fmt.Printf("Uploading filename %s (%s) to bin %s\n", inputFilename, humanize.Bytes(inputBytes), bin.Id)

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
	if uint64(nBytes) != inputBytes {
		fmt.Printf("Rejecting upload for file %s to bin %s since we got %d bytes and should have received %d bytes\n", inputFilename, bin.Id, nBytes, inputBytes)
		http.Error(w, "Request body of different size than specified by content-length", http.StatusBadRequest)
		return
	}
	if nBytes == 0 {
		fmt.Printf("Rejecting upload of empty file %s to bin %s (%d bytes)\n", inputFilename, bin.Id, nBytes)
		http.Error(w, "Empty file uploads are not allowed", http.StatusBadRequest)
		return
	}
	fp.Seek(0, 0)

	t2 := time.Now()

	md5_checksum := md5.New()
	if _, err := io.Copy(md5_checksum, fp); err != nil {
		fmt.Printf("Error during checksum: %s\n", err.Error())
		return
	}
	md5_checksum_string := fmt.Sprintf("%x", md5_checksum.Sum(nil))
	if inputMD5 != "" {
		if md5_checksum_string != inputMD5 {
			fmt.Printf("Rejecting upload for file %s to bin %s due to wrong MD5 checksum (got %s and calculated %s)\n", inputFilename, bin.Id, inputMD5, md5_checksum_string)
			http.Error(w, "MD5 checksum did not match", http.StatusBadRequest)
			return
		}
	}
	fp.Seek(0, 0)

	sha256_checksum := sha256.New()
	if _, err := io.Copy(sha256_checksum, fp); err != nil {
		fmt.Printf("Error during checksum: %s\n", err.Error())
		return
	}
	sha256_checksum_string := fmt.Sprintf("%x", sha256_checksum.Sum(nil))
	if inputSHA256 != "" {
		if sha256_checksum_string != inputSHA256 {
			fmt.Printf("Rejecting upload for file %s to bin %s due to wrong SHA256 checksum (got %s and calculated %s)\n", inputFilename, bin.Id, inputSHA256, sha256_checksum_string)
			http.Error(w, "SHA256 checksum did not match", http.StatusBadRequest)
			return
		}
	}
	fp.Seek(0, 0)

	mime, err := mimetype.DetectReader(fp)
	if err != nil {
		fmt.Printf("Unable to detect mime type on filename %s in bin %s: %s\n", inputFilename, inputBin, err.Error())
		http.Error(w, "Errno 105", http.StatusInternalServerError)
		return
	}
	fp.Seek(0, 0)

	t3 := time.Now()

	// Check if file exists
	file, found, err := h.dao.File().GetByName(bin.Id, inputFilename)
	if err != nil {
		fmt.Printf("Unable to load file %s in bin %s: %s\n", file.Filename, bin.Id, err.Error())
		http.Error(w, "Errno 106", http.StatusInternalServerError)
		return
	}

	// Set values according to the new file
	file.Filename = inputFilename
	file.Bin = bin.Id
	file.Deleted = 0
	file.Bytes = inputBytes
	file.Mime = mime.String()
	file.SHA256 = sha256_checksum_string
	file.MD5 = md5_checksum_string
	if err := h.dao.File().ValidateInput(&file); err != nil {
		fmt.Printf("Rejected upload of filename %s to bin %s due to failed input validation: %s\n", inputFilename, bin.Id, err.Error())
		http.Error(w, "Input validation failed", http.StatusBadRequest)
		return
	}

	nonce, err := h.s3.PutObject(file.Bin, file.Filename, fp, nBytes)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		http.Error(w, "Insufficient storage capacity, please retry later", http.StatusInternalServerError)
		return
	}
	t4 := time.Now()

	file.Nonce = nonce

	if found {
		if err := h.dao.File().Update(&file); err != nil {
			fmt.Printf("Unable to update filename %s (id %d) in bin %s: %s\n", file.Filename, file.Id, bin.Id, err.Error())
			http.Error(w, "Errno 107", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.dao.File().Insert(&file); err != nil {
			fmt.Printf("Unable to insert file %s in bin %s: %s\n", file.Filename, bin.Id, err.Error())
			http.Error(w, "Errno 108", http.StatusInternalServerError)
			return
		}
		// TODO: Execute new file created trigger
	}

	// Update bin to set the correct updated timestamp
	if err := h.dao.Bin().Update(&bin); err != nil {
		fmt.Printf("Unable to update bin %s: %s\n", bin.Id, err.Error())
		http.Error(w, "Errno 109", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Uploaded filename %s (%s) to bin %s (db in %.3fs, buffered in %.3fs, checksum in %.3fs, stored in %.3fs, total %.3fs)\n", file.Filename, humanize.Bytes(file.Bytes), bin.Id, t1.Sub(t0).Seconds(), t2.Sub(t1).Seconds(), t3.Sub(t2).Seconds(), t4.Sub(t3).Seconds(), t4.Sub(t0).Seconds())

	w.WriteHeader(http.StatusCreated)
}

func (h *HTTP) DeleteFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	inputBin := params["bin"]
	inputFilename := params["filename"]

	bin, found, err := h.dao.Bin().GetById(inputBin)
	if err != nil {
		fmt.Printf("Unable to GetById(%s): %s\n", inputBin, err.Error())
		http.Error(w, "Errno 110", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}

	// No need to delete the file if the bin is already deleted
	if bin.Deleted > 0 {
		http.Error(w, "This bin is no longer available", http.StatusNotFound)
		return
	}

	file, found, err := h.dao.File().GetByName(inputBin, inputFilename)
	if err != nil {
		fmt.Printf("Unable to GetByName(%s, %s): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Errno 111", http.StatusInternalServerError)
		return
	}
	if found == false {
		http.Error(w, "The file does not exist", http.StatusNotFound)
		return
	}

	// No need to delete the file twice
	if file.Deleted > 0 {
		http.Error(w, "This file is no longer available", http.StatusNotFound)
		return
	}

	// Set to pending delete
	file.Deleted = 1
	if err := h.dao.File().Update(&file); err != nil {
		fmt.Printf("Unable to update the file (%s, %s): %s\n", inputBin, inputFilename, err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Update the updated timestamp of the bin
	if err := h.dao.Bin().Update(&bin); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Error(w, "File deleted successfully", http.StatusOK)
	return
}
