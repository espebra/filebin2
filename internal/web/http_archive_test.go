package web

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
)

func TestArchiveDownload(t *testing.T) {
	// Setup: Upload test files to a bin
	binID := "archivetest01"
	files := map[string]string{
		"file1.txt": "content of file 1",
		"file2.txt": "content of file 2",
		"file3.txt": "content of file 3",
	}

	// Upload files
	for filename, content := range files {
		tc := TestCase{
			Description:   fmt.Sprintf("Upload %s for archive test", filename),
			Method:        "POST",
			Bin:           binID,
			Filename:      filename,
			UploadContent: content,
			StatusCode:    http.StatusCreated,
		}
		statusCode, _, err := httpRequest(tc)
		if err != nil {
			t.Fatalf("Failed to upload test file %s: %s", filename, err.Error())
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("Expected status %d for upload, got %d", http.StatusCreated, statusCode)
		}
	}

	t.Run("download tar archive", func(t *testing.T) {
		statusCode, body, err := downloadArchive(binID, "tar")
		if err != nil {
			t.Fatalf("Failed to download tar archive: %s", err.Error())
		}
		if statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", statusCode)
		}

		// Verify tar contents
		tr := tar.NewReader(strings.NewReader(body))
		foundFiles := make(map[string]string)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Error reading tar: %s", err.Error())
			}

			content, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("Error reading tar file content: %s", err.Error())
			}
			foundFiles[header.Name] = string(content)
		}

		// Verify all files are in the archive
		for filename, expectedContent := range files {
			actualContent, found := foundFiles[filename]
			if !found {
				t.Errorf("Expected file %s not found in tar archive", filename)
			} else if actualContent != expectedContent {
				t.Errorf("File %s content mismatch. Expected %q, got %q", filename, expectedContent, actualContent)
			}
		}

		if len(foundFiles) != len(files) {
			t.Errorf("Expected %d files in archive, found %d", len(files), len(foundFiles))
		}
	})

	t.Run("download zip archive", func(t *testing.T) {
		statusCode, body, err := downloadArchive(binID, "zip")
		if err != nil {
			t.Fatalf("Failed to download zip archive: %s", err.Error())
		}
		if statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", statusCode)
		}

		// Verify zip contents
		zipReader, err := zip.NewReader(strings.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatalf("Error opening zip archive: %s", err.Error())
		}

		foundFiles := make(map[string]string)
		for _, file := range zipReader.File {
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("Error opening file in zip: %s", err.Error())
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("Error reading file content from zip: %s", err.Error())
			}
			foundFiles[file.Name] = string(content)
		}

		// Verify all files are in the archive
		for filename, expectedContent := range files {
			actualContent, found := foundFiles[filename]
			if !found {
				t.Errorf("Expected file %s not found in zip archive", filename)
			} else if actualContent != expectedContent {
				t.Errorf("File %s content mismatch. Expected %q, got %q", filename, expectedContent, actualContent)
			}
		}

		if len(foundFiles) != len(files) {
			t.Errorf("Expected %d files in archive, found %d", len(files), len(foundFiles))
		}
	})

	t.Run("verify tar archive is not double-compressed", func(t *testing.T) {
		statusCode, body, err := downloadArchiveWithAcceptEncoding(binID, "tar", "gzip")
		if err != nil {
			t.Fatalf("Failed to download tar archive: %s", err.Error())
		}
		if statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", statusCode)
		}

		// Try to decompress with gzip - should fail or not be gzipped
		gzipReader, err := gzip.NewReader(strings.NewReader(body))
		if err == nil {
			// If it successfully opens as gzip, the archive was compressed
			gzipReader.Close()
			t.Error("Archive appears to be gzip compressed when it shouldn't be")
		}

		// Verify it's a valid tar archive
		tr := tar.NewReader(strings.NewReader(body))
		_, err = tr.Next()
		if err != nil {
			t.Errorf("Archive is not a valid tar file: %s", err.Error())
		}
	})

	t.Run("verify zip archive is not double-compressed", func(t *testing.T) {
		statusCode, body, err := downloadArchiveWithAcceptEncoding(binID, "zip", "gzip")
		if err != nil {
			t.Fatalf("Failed to download zip archive: %s", err.Error())
		}
		if statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", statusCode)
		}

		// Try to decompress with gzip - should fail or not be gzipped
		gzipReader, err := gzip.NewReader(strings.NewReader(body))
		if err == nil {
			// If it successfully opens as gzip, the archive was compressed
			gzipReader.Close()
			t.Error("Archive appears to be gzip compressed when it shouldn't be")
		}

		// Verify it's a valid zip archive
		_, err = zip.NewReader(strings.NewReader(body), int64(len(body)))
		if err != nil {
			t.Errorf("Archive is not a valid zip file: %s", err.Error())
		}
	})
}

func TestArchiveErrorCases(t *testing.T) {
	t.Run("non-existent bin returns 404", func(t *testing.T) {
		statusCode, _, err := downloadArchive("nonexistentbin123", "tar")
		if err != nil {
			t.Fatalf("Request failed: %s", err.Error())
		}
		if statusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", statusCode)
		}
	})

	t.Run("invalid format returns 404", func(t *testing.T) {
		// First create a bin with a file
		binID := "archivetest02"
		tc := TestCase{
			Method:        "POST",
			Bin:           binID,
			Filename:      "test.txt",
			UploadContent: "test content",
			StatusCode:    http.StatusCreated,
		}
		_, _, _ = httpRequest(tc)

		// Try to download with invalid format
		statusCode, _, err := downloadArchive(binID, "rar")
		if err != nil {
			t.Fatalf("Request failed: %s", err.Error())
		}
		if statusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for invalid format, got %d", statusCode)
		}
	})

	t.Run("empty bin returns 404", func(t *testing.T) {
		// Create a bin with a file, then delete the file
		binID := "archivetest03"
		tc := TestCase{
			Method:        "POST",
			Bin:           binID,
			Filename:      "test.txt",
			UploadContent: "test content",
			StatusCode:    http.StatusCreated,
		}
		_, _, _ = httpRequest(tc)

		// Delete the file
		tc.Method = "DELETE"
		tc.UploadContent = ""
		tc.StatusCode = http.StatusOK
		_, _, _ = httpRequest(tc)

		// Try to download archive of empty bin
		statusCode, _, err := downloadArchive(binID, "tar")
		if err != nil {
			t.Fatalf("Request failed: %s", err.Error())
		}
		if statusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for empty bin, got %d", statusCode)
		}
	})

	t.Run("deleted bin returns 404", func(t *testing.T) {
		// Create a bin with a file
		binID := "archivetest04"
		tc := TestCase{
			Method:        "POST",
			Bin:           binID,
			Filename:      "test.txt",
			UploadContent: "test content",
			StatusCode:    http.StatusCreated,
		}
		_, _, _ = httpRequest(tc)

		// Delete the bin
		tc.Method = "DELETE"
		tc.Filename = ""
		tc.UploadContent = ""
		tc.StatusCode = http.StatusOK
		_, _, _ = httpRequest(tc)

		// Try to download archive of deleted bin
		statusCode, _, err := downloadArchive(binID, "tar")
		if err != nil {
			t.Fatalf("Request failed: %s", err.Error())
		}
		if statusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for deleted bin, got %d", statusCode)
		}
	})
}

func TestArchiveSingleFile(t *testing.T) {
	binID := "archivetest05"
	filename := "singlefile.txt"
	content := "This is a single file"

	// Upload a single file
	tc := TestCase{
		Method:        "POST",
		Bin:           binID,
		Filename:      filename,
		UploadContent: content,
		StatusCode:    http.StatusCreated,
	}
	statusCode, _, err := httpRequest(tc)
	if err != nil {
		t.Fatalf("Failed to upload file: %s", err.Error())
	}
	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", statusCode)
	}

	t.Run("tar archive with single file", func(t *testing.T) {
		statusCode, body, err := downloadArchive(binID, "tar")
		if err != nil {
			t.Fatalf("Failed to download tar: %s", err.Error())
		}
		if statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", statusCode)
		}

		// Verify tar contains the file
		tr := tar.NewReader(strings.NewReader(body))
		header, err := tr.Next()
		if err != nil {
			t.Fatalf("Error reading tar: %s", err.Error())
		}
		if header.Name != filename {
			t.Errorf("Expected filename %s, got %s", filename, header.Name)
		}

		fileContent, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("Error reading file content: %s", err.Error())
		}
		if string(fileContent) != content {
			t.Errorf("Content mismatch. Expected %q, got %q", content, string(fileContent))
		}
	})

	t.Run("zip archive with single file", func(t *testing.T) {
		statusCode, body, err := downloadArchive(binID, "zip")
		if err != nil {
			t.Fatalf("Failed to download zip: %s", err.Error())
		}
		if statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", statusCode)
		}

		// Verify zip contains the file
		zipReader, err := zip.NewReader(strings.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatalf("Error reading zip: %s", err.Error())
		}
		if len(zipReader.File) != 1 {
			t.Fatalf("Expected 1 file in zip, got %d", len(zipReader.File))
		}

		file := zipReader.File[0]
		if file.Name != filename {
			t.Errorf("Expected filename %s, got %s", filename, file.Name)
		}

		rc, err := file.Open()
		if err != nil {
			t.Fatalf("Error opening file: %s", err.Error())
		}
		defer rc.Close()

		fileContent, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("Error reading file content: %s", err.Error())
		}
		if string(fileContent) != content {
			t.Errorf("Content mismatch. Expected %q, got %q", content, string(fileContent))
		}
	})
}

func TestArchiveContentType(t *testing.T) {
	// Setup: Create bin with file
	binID := "archivetest06"
	tc := TestCase{
		Method:        "POST",
		Bin:           binID,
		Filename:      "test.txt",
		UploadContent: "test",
		StatusCode:    http.StatusCreated,
	}
	_, _, _ = httpRequest(tc)

	t.Run("tar has correct content type", func(t *testing.T) {
		contentType, err := getArchiveContentType(binID, "tar")
		if err != nil {
			t.Fatalf("Failed to get content type: %s", err.Error())
		}
		// Tar archives typically use application/x-tar or application/octet-stream
		if !strings.Contains(contentType, "application/") {
			t.Errorf("Expected application/* content type for tar, got %s", contentType)
		}
	})

	t.Run("zip has correct content type", func(t *testing.T) {
		contentType, err := getArchiveContentType(binID, "zip")
		if err != nil {
			t.Fatalf("Failed to get content type: %s", err.Error())
		}
		if !strings.Contains(contentType, "application/zip") && !strings.Contains(contentType, "application/octet-stream") {
			t.Errorf("Expected application/zip content type, got %s", contentType)
		}
	})
}

// Helper function to download an archive
func downloadArchive(binID, format string) (int, string, error) {
	return downloadArchiveWithAcceptEncoding(binID, format, "")
}

// Helper function to download an archive with specific Accept-Encoding
func downloadArchiveWithAcceptEncoding(binID, format, acceptEncoding string) (int, string, error) {
	u, err := url.Parse("http://localhost:8080")
	if err != nil {
		return -1, "", err
	}
	u.Path = path.Join("/archive", binID, format)

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return -1, "", err
	}
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	req.Close = true

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -1, "", err
	}
	defer resp.Body.Close()

	// Read the body - important: do not automatically decompress
	var body bytes.Buffer
	_, err = io.Copy(&body, resp.Body)
	if err != nil {
		return -1, "", err
	}

	return resp.StatusCode, body.String(), nil
}

// Helper function to get content type header
func getArchiveContentType(binID, format string) (string, error) {
	u, err := url.Parse("http://localhost:8080")
	if err != nil {
		return "", err
	}
	u.Path = path.Join("/archive", binID, format)

	req, err := http.NewRequest("HEAD", u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Close = true

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return resp.Header.Get("Content-Type"), nil
}
