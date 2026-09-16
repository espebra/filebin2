package web

import (
	"io"
	"strings"
	"testing"
)

func TestContentDigest(t *testing.T) {
	d := newContentDigest()
	if _, err := io.Copy(d.Writer(), strings.NewReader("content a")); err != nil {
		t.Fatal(err)
	}
	// Expected values are shared with the upload test cases in http_file_test.go.
	if got, want := d.SHA256(), "0069ffe8481777aa403982d9e9b3fa48957015a07cfa0f66dae32050b95bda54"; got != want {
		t.Errorf("SHA256: got %s, want %s", got, want)
	}
	// Base64 encoding of d8114b361885ee54897e52ce2308e274.
	if got, want := d.MD5(), "2BFLNhiF7lSJflLOIwjidA=="; got != want {
		t.Errorf("MD5: got %s, want %s", got, want)
	}
}
