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
	if got, want := d.SHA1(), "739afa0237fd8196c6b774a48676524bd95275b2"; got != want {
		t.Errorf("SHA1: got %s, want %s", got, want)
	}
	// Base64 encoding of d8114b361885ee54897e52ce2308e274.
	if got, want := d.MD5(), "2BFLNhiF7lSJflLOIwjidA=="; got != want {
		t.Errorf("MD5: got %s, want %s", got, want)
	}
	if got, want := md5Hex(d.MD5()), "d8114b361885ee54897e52ce2308e274"; got != want {
		t.Errorf("md5Hex: got %s, want %s", got, want)
	}
	if got := md5Hex("not base64!"); got != "" {
		t.Errorf("md5Hex of invalid input: got %q, want empty", got)
	}
}
