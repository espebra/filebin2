package main

import (
	//"io"
	"fmt"
	"net/http"
	//"encoding/json"

	//"github.com/espebra/filebin2/ds"
	"github.com/gorilla/mux"
	//"github.com/dustin/go-humanize"
)

func (h *HTTP) GetFile(w http.ResponseWriter, r *http.Request) {
        params := mux.Vars(r)
        inputBin := params["bin"]
        // TODO: Input validation (inputBin)
        inputFilename := params["filename"]
        // TODO: Input validation (inputFilename)

	fmt.Printf("Foo\n")

        file, err := h.dao.File().GetByName(inputBin, inputFilename)
        if err != nil {
                fmt.Printf("Unable to GetByName(%s, %s): %s\n", inputBin, inputFilename, err.Error())
                http.Error(w, "Errno 1", http.StatusInternalServerError)
        }

	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))

        //fp, err := s3ao.GetObject(name)
        //if err != nil {
        //        t.Errorf("Unable to get object: %s\n", err.Error())
        //}

        //buf := new(bytes.Buffer)
        //buf.ReadFrom(fp)
        //s := buf.String()
        //if content != s {
        //        t.Errorf("Invalid content from get object. Expected %s, got %s\n", content, s)
        //}
}
