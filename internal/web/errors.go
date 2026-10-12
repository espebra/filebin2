package web

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/gorilla/mux"
)

// httpError is returned by a handler when the client should get a specific
// status code or message. Any other error is reported as a 500 with a
// generic message. The message must be safe to show to anyone. The cause,
// if set, is internal detail that only ends up in the log.
//
// A minimal rejection:
//
//	return &httpError{status: http.StatusBadRequest, message: "Empty file uploads are not allowed"}
//
// With a cause for the log and a header for the response:
//
//	return &httpError{
//		status:  http.StatusMethodNotAllowed,
//		message: "Uploads to locked bins are not allowed",
//		err:     fmt.Errorf("bin %q is locked", bin.Id),
//		headers: map[string]string{"Allow": "GET, HEAD"},
//	}
type httpError struct {
	status  int
	message string
	err     error             // optional cause, logged but never sent to the client
	headers map[string]string // optional response headers, by name
}

func (e *httpError) Error() string {
	if e.err != nil {
		return e.message + ": " + e.err.Error()
	}
	return e.message
}

func (e *httpError) Unwrap() error {
	return e.err
}

// handlerFunc is an HTTP handler that reports failures by returning an error
// instead of writing the response itself. A handler must not write to the
// response before it is past the point where it can fail.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

// handle adapts a handlerFunc to http.HandlerFunc, rendering any error it
// returns as the response.
func (h *HTTP) handle(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			h.respondError(w, r, err)
		}
	}
}

// respondError logs err and writes it as the response. Errors that are not
// an httpError are reported as 500 without revealing the cause to the
// client. Browsers get the HTML error page and everything else plain text.
func (h *HTTP) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var he *httpError
	if !errors.As(err, &he) {
		he = &httpError{status: http.StatusInternalServerError, message: "Internal server error", err: err}
	}

	attrs := []any{"method", r.Method, "path", r.URL.Path, "status", he.status, "error", err}
	if vars := mux.Vars(r); vars["bin"] != "" {
		attrs = append(attrs, "bin", vars["bin"])
		if vars["filename"] != "" {
			attrs = append(attrs, "filename", vars["filename"])
		}
	}
	// Server errors are logged at error and client errors at info, except
	// not found, which is routine traffic from stale links and scanners.
	switch {
	case he.status >= http.StatusInternalServerError:
		slog.Error("request failed", attrs...)
	case he.status == http.StatusNotFound:
		slog.Debug("request rejected", attrs...)
	default:
		slog.Info("request rejected", attrs...)
	}

	w.Header().Set("Cache-Control", "max-age=1")
	w.Header().Set("X-Robots-Tag", "noindex")
	// A handler may have announced a download before failing. The error
	// must be shown, not saved as a file.
	w.Header().Del("Content-Disposition")
	for name, value := range he.headers {
		w.Header().Set(name, value)
	}

	// Disregard any request body there is
	_, _ = io.Copy(io.Discard, r.Body)

	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		h.metrics.IncrErrorPageViewCount()
		data := struct {
			ds.Common
			Text       string
			StatusCode int
		}{Text: he.message, StatusCode: he.status}
		data.Contact = h.config.Contact
		data.BaseUrl = h.config.BaseUrl.String()
		var buf bytes.Buffer
		if err := h.templates.ExecuteTemplate(&buf, "error_page", data); err != nil {
			slog.Error("failed to execute template", "template", "error_page", "error", err)
			http.Error(w, he.message, he.status)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(he.status)
		_, _ = buf.WriteTo(w)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(he.status)
	_, _ = io.WriteString(w, he.message)
}
