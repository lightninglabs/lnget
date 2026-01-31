package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestGetResumeInfo tests resume info retrieval.
func TestGetResumeInfo(t *testing.T) {
	// Create temp directory.
	tmpDir := t.TempDir()

	// Test non-existent file.
	t.Run("non-existent file", func(t *testing.T) {
		info, err := GetResumeInfo(filepath.Join(tmpDir, "nonexistent"))
		if err != nil {
			t.Fatalf("GetResumeInfo() error = %v", err)
		}

		if info.Size != 0 {
			t.Errorf("Size = %d, want 0", info.Size)
		}

		if info.CanResume {
			t.Error("CanResume should be false for non-existent file")
		}
	})

	// Test existing file with content.
	t.Run("existing file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "partial")

		err := os.WriteFile(filePath, []byte("partial content"), 0600)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		info, err := GetResumeInfo(filePath)
		if err != nil {
			t.Fatalf("GetResumeInfo() error = %v", err)
		}

		if info.Size != 15 {
			t.Errorf("Size = %d, want 15", info.Size)
		}

		if !info.CanResume {
			t.Error("CanResume should be true for existing file")
		}
	})

	// Test empty file.
	t.Run("empty file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "empty")

		err := os.WriteFile(filePath, []byte{}, 0600)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		info, err := GetResumeInfo(filePath)
		if err != nil {
			t.Fatalf("GetResumeInfo() error = %v", err)
		}

		if info.Size != 0 {
			t.Errorf("Size = %d, want 0", info.Size)
		}

		if info.CanResume {
			t.Error("CanResume should be false for empty file")
		}
	})
}

// TestSetResumeHeader tests setting the Range header.
func TestSetResumeHeader(t *testing.T) {
	tests := []struct {
		startByte int64
		expected  string
	}{
		{0, "bytes=0-"},
		{100, "bytes=100-"},
		{1024, "bytes=1024-"},
	}

	for _, tc := range tests {
		req, _ := http.NewRequestWithContext(
			context.Background(), http.MethodGet,
			"http://example.com", nil,
		)
		SetResumeHeader(req, tc.startByte)

		result := req.Header.Get("Range")
		if result != tc.expected {
			t.Errorf("SetResumeHeader(%d) = %q, want %q",
				tc.startByte, result, tc.expected)
		}
	}
}

// TestIsPartialResponse tests partial response detection.
func TestIsPartialResponse(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{http.StatusOK, false},
		{http.StatusPartialContent, true},
		{http.StatusNotFound, false},
	}

	for _, tc := range tests {
		resp := &http.Response{StatusCode: tc.statusCode}

		result := IsPartialResponse(resp)
		if result != tc.expected {
			t.Errorf("IsPartialResponse(%d) = %v, want %v",
				tc.statusCode, result, tc.expected)
		}
	}
}

// TestCheckServerResumeSupport tests checking server resume support.
func TestCheckServerResumeSupport(t *testing.T) {
	t.Run("server supports resume", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", "1024")
			},
		))
		defer server.Close()

		canResume, size, err := CheckServerResumeSupport(
			context.Background(), server.URL, server.Client(),
		)
		if err != nil {
			t.Fatalf("CheckServerResumeSupport() error = %v", err)
		}

		if !canResume {
			t.Error("canResume should be true")
		}

		if size != 1024 {
			t.Errorf("size = %d, want 1024", size)
		}
	})

	t.Run("server does not support resume", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "512")
				// No Accept-Ranges header.
			},
		))
		defer server.Close()

		canResume, size, err := CheckServerResumeSupport(
			context.Background(), server.URL, server.Client(),
		)
		if err != nil {
			t.Fatalf("CheckServerResumeSupport() error = %v", err)
		}

		if canResume {
			t.Error("canResume should be false")
		}

		if size != 512 {
			t.Errorf("size = %d, want 512", size)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		_, _, err := CheckServerResumeSupport(
			context.Background(), "://invalid", http.DefaultClient,
		)
		if err == nil {
			t.Error("expected error for invalid URL")
		}
	})
}

// TestResumeInfoStruct tests the ResumeInfo struct.
func TestResumeInfoStruct(t *testing.T) {
	info := ResumeInfo{
		FilePath:  "/tmp/test",
		Size:      1024,
		CanResume: true,
		TotalSize: 2048,
	}

	if info.FilePath != "/tmp/test" {
		t.Errorf("FilePath = %q, want '/tmp/test'", info.FilePath)
	}

	if info.Size != 1024 {
		t.Errorf("Size = %d, want 1024", info.Size)
	}

	if !info.CanResume {
		t.Error("CanResume should be true")
	}

	if info.TotalSize != 2048 {
		t.Errorf("TotalSize = %d, want 2048", info.TotalSize)
	}
}
