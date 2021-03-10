// Copyright 2013 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Modified for https://github.com/espebra/filebin2

package logging

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
	//"github.com/gorilla/mux"
	"github.com/espebra/filebin2/dbl"
	"github.com/felixge/httpsnoop"
)

// responseLogger is wrapper of http.ResponseWriter that keeps track of its HTTP
// status code and body size
type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Write(b []byte) (int, error) {
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func (l *responseLogger) Status() int {
	return l.status
}

func (l *responseLogger) Size() int {
	return l.size
}

func (l *responseLogger) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := l.w.(http.Hijacker).Hijack()
	if err == nil && l.status == 0 {
		// The status will be StatusSwitchingProtocols if there was no error and
		// WriteHeader has not been called yet
		l.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

type RequestInfo struct {
	Request    *http.Request
	URL        url.URL
	TimeStamp  time.Time
	StatusCode int
	Size       int
}

type LogFormatter func(dao *dbl.DAO, info RequestInfo)

type loggingHandler struct {
	dao       *dbl.DAO
	handler   http.Handler
	formatter LogFormatter
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	t := time.Now()
	logger, w := makeLogger(w)
	url := *req.URL

	h.handler.ServeHTTP(w, req)
	if req.MultipartForm != nil {
		req.MultipartForm.RemoveAll()
	}

	info := RequestInfo{
		Request:    req,
		URL:        url,
		TimeStamp:  t,
		StatusCode: logger.Status(),
		Size:       logger.Size(),
	}

	h.formatter(h.dao, info)
}

func makeLogger(w http.ResponseWriter) (*responseLogger, http.ResponseWriter) {
	logger := &responseLogger{w: w, status: http.StatusOK}
	return logger, httpsnoop.Wrap(w, httpsnoop.Hooks{
		Write: func(httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return logger.Write
		},
		WriteHeader: func(httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return logger.WriteHeader
		},
	})
}

func writeCombinedLog(dao *dbl.DAO, info RequestInfo) {
	_, err := dao.Transaction().Register(info.Request, info.TimeStamp, info.StatusCode, info.Size)
	if err != nil {
		fmt.Printf("Unable to register the transaction: %s\n", err.Error())
	}
}

func CombinedLoggingHandler(dao *dbl.DAO, h http.Handler) http.Handler {
	return loggingHandler{dao, h, writeCombinedLog}
}
