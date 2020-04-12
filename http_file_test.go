package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestUploadFile(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	filename := "a"
	bin := "mytestbin"
	content := "content a"
	req, _ := http.NewRequest("POST", "http://localhost:8080/", strings.NewReader(content))
	req.Header.Set("Filename", filename)
	req.Header.Set("Bin", bin)
	req.Header.Set("Content-SHA256", "8bfe5f10912d733d91a002a4f9990bd72ff03120817d46742f810c0484d626ef")
	req.Header.Set("Content-MD5", "d19f7ae40de729f92bf9aea2657d1c77")
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Did not expect file upload to fail: %s\n", err.Error())
	}
	if resp.StatusCode != 201 {
		t.Errorf("Expected response code 201, got %d\n", resp.StatusCode)
	}
	resp.Body.Close()
	req.Close = true
}
