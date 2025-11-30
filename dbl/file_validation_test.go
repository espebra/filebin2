package dbl

import (
	"strings"
	"testing"

	"github.com/espebra/filebin2/ds"
)

func TestSetCategory(t *testing.T) {
	tests := []struct {
		name     string
		mime     string
		expected string
	}{
		{
			name:     "image mime type (jpeg)",
			mime:     "image/jpeg",
			expected: "image",
		},
		{
			name:     "image mime type (png)",
			mime:     "image/png",
			expected: "image",
		},
		{
			name:     "image mime type (gif)",
			mime:     "image/gif",
			expected: "image",
		},
		{
			name:     "video mime type (mp4)",
			mime:     "video/mp4",
			expected: "video",
		},
		{
			name:     "video mime type (webm)",
			mime:     "video/webm",
			expected: "video",
		},
		{
			name:     "video mime type (quicktime)",
			mime:     "video/quicktime",
			expected: "video",
		},
		{
			name:     "text mime type",
			mime:     "text/plain",
			expected: "unknown",
		},
		{
			name:     "application mime type",
			mime:     "application/pdf",
			expected: "unknown",
		},
		{
			name:     "audio mime type",
			mime:     "audio/mpeg",
			expected: "unknown",
		},
		{
			name:     "empty mime type",
			mime:     "",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &ds.File{Mime: tt.mime}
			setCategory(file)
			if file.Category != tt.expected {
				t.Errorf("setCategory() with mime %q: got category %q, want %q", tt.mime, file.Category, tt.expected)
			}
		})
	}
}

func TestValidateInput(t *testing.T) {
	d := &FileDao{} // No DB needed for validation

	tests := []struct {
		name           string
		inputFilename  string
		expectError    bool
		expectedOutput string
	}{
		{
			name:           "simple valid filename",
			inputFilename:  "test.txt",
			expectError:    false,
			expectedOutput: "test.txt",
		},
		{
			name:           "filename with spaces",
			inputFilename:  "my document.pdf",
			expectError:    false,
			expectedOutput: "my document.pdf",
		},
		{
			name:           "filename with leading whitespace",
			inputFilename:  "  test.txt",
			expectError:    false,
			expectedOutput: "test.txt",
		},
		{
			name:           "filename with trailing whitespace",
			inputFilename:  "test.txt  ",
			expectError:    false,
			expectedOutput: "test.txt",
		},
		{
			name:           "filename with multiple spaces (should collapse)",
			inputFilename:  "my    document    file.pdf",
			expectError:    false,
			expectedOutput: "my document file.pdf",
		},
		{
			name:          "empty filename",
			inputFilename: "",
			expectError:   true,
		},
		{
			name:          "whitespace only filename",
			inputFilename: "   ",
			expectError:   true,
		},
		{
			name:           "filename with path (should extract basename)",
			inputFilename:  "folder/subfolder/file.txt",
			expectError:    false,
			expectedOutput: "file.txt",
		},
		{
			name:           "path traversal attempt (should extract basename)",
			inputFilename:  "../../etc/passwd",
			expectError:    false,
			expectedOutput: "passwd",
		},
		{
			name:           "windows path (backslashes become underscores on unix)",
			inputFilename:  "C:\\Users\\test\\file.txt",
			expectError:    false,
			expectedOutput: "C__Users_test_file.txt", // On Unix, backslash is not a path separator
		},
		{
			name:           "filename starting with dot (should replace)",
			inputFilename:  ".hidden",
			expectError:    false,
			expectedOutput: "_hidden",
		},
		{
			name:           "filename with allowed special characters",
			inputFilename:  "test-file_name=v1.2+final,(copy)[1].txt",
			expectError:    false,
			expectedOutput: "test-file_name=v1.2+final,(copy)[1].txt",
		},
		{
			name:           "filename with disallowed characters (should replace with underscore)",
			inputFilename:  "test@file#name$.txt",
			expectError:    false,
			expectedOutput: "test_file_name_.txt",
		},
		{
			name:           "filename with unicode letters (should preserve)",
			inputFilename:  "café.txt",
			expectError:    false,
			expectedOutput: "café.txt",
		},
		{
			name:           "filename with unicode numbers (should preserve)",
			inputFilename:  "test١٢٣.txt",
			expectError:    false,
			expectedOutput: "test١٢٣.txt",
		},
		{
			name:           "filename with emoji (should replace)",
			inputFilename:  "test😀file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with tab character (should replace)",
			inputFilename:  "test\tfile.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with newline (should replace)",
			inputFilename:  "test\nfile.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "very long filename (should truncate to 120 chars)",
			inputFilename:  strings.Repeat("a", 150) + ".txt",
			expectError:    false,
			expectedOutput: strings.Repeat("a", 120),
		},
		{
			name:           "filename exactly 120 characters (should not truncate)",
			inputFilename:  strings.Repeat("a", 116) + ".txt",
			expectError:    false,
			expectedOutput: strings.Repeat("a", 116) + ".txt",
		},
		{
			name:           "filename with slash (should replace)",
			inputFilename:  "test/file.txt",
			expectError:    false,
			expectedOutput: "file.txt", // filepath.Base extracts basename
		},
		{
			name:           "filename with backslash (backslash becomes underscore on unix)",
			inputFilename:  "test\\file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt", // On Unix, backslash is not a path separator, becomes underscore
		},
		{
			name:           "filename with null byte (should replace)",
			inputFilename:  "test\x00file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with colon (should replace)",
			inputFilename:  "test:file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with asterisk (should replace)",
			inputFilename:  "test*file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with question mark (should replace)",
			inputFilename:  "test?file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with pipe (should replace)",
			inputFilename:  "test|file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with less than (should replace)",
			inputFilename:  "test<file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with greater than (should replace)",
			inputFilename:  "test>file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "filename with quotes (should replace)",
			inputFilename:  "test\"file.txt",
			expectError:    false,
			expectedOutput: "test_file.txt",
		},
		{
			name:           "complex unicode filename with valid characters",
			inputFilename:  "文档.txt",
			expectError:    false,
			expectedOutput: "文档.txt",
		},
		{
			name:           "filename with multiple dots",
			inputFilename:  "archive.tar.gz",
			expectError:    false,
			expectedOutput: "archive.tar.gz",
		},
		{
			name:           "filename starting with dot and having extension",
			inputFilename:  ".gitignore",
			expectError:    false,
			expectedOutput: "_gitignore",
		},
		{
			name:           "filename with parentheses and brackets",
			inputFilename:  "test (copy) [1].txt",
			expectError:    false,
			expectedOutput: "test (copy) [1].txt",
		},
		{
			name:           "filename with equals and plus",
			inputFilename:  "base64=encoded+string.txt",
			expectError:    false,
			expectedOutput: "base64=encoded+string.txt",
		},
		{
			name:           "filename with comma",
			inputFilename:  "data,values,file.csv",
			expectError:    false,
			expectedOutput: "data,values,file.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &ds.File{Filename: tt.inputFilename}
			err := d.ValidateInput(file)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateInput(%q): expected error, got nil", tt.inputFilename)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateInput(%q): unexpected error: %v", tt.inputFilename, err)
				}
				if file.Filename != tt.expectedOutput {
					t.Errorf("ValidateInput(%q): got filename %q, want %q", tt.inputFilename, file.Filename, tt.expectedOutput)
				}
			}
		})
	}
}

func TestValidateInputIdempotency(t *testing.T) {
	d := &FileDao{}

	// Test that running validation twice produces the same result
	testCases := []string{
		"test.txt",
		"my document.pdf",
		"test@#$.txt",
		strings.Repeat("a", 150),
		".hidden",
		"  spaces  everywhere  .txt",
	}

	for _, input := range testCases {
		t.Run(input, func(t *testing.T) {
			file1 := &ds.File{Filename: input}
			err1 := d.ValidateInput(file1)
			if err1 != nil {
				t.Skipf("First validation failed: %v", err1)
			}
			firstResult := file1.Filename

			file2 := &ds.File{Filename: firstResult}
			err2 := d.ValidateInput(file2)
			if err2 != nil {
				t.Errorf("Second validation failed: %v", err2)
			}
			secondResult := file2.Filename

			if firstResult != secondResult {
				t.Errorf("Validation is not idempotent: first=%q, second=%q", firstResult, secondResult)
			}
		})
	}
}
