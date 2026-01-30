package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
)

// ResumeInfo contains information about a partial download.
type ResumeInfo struct {
	// FilePath is the path to the partial file.
	FilePath string

	// Size is the current size of the partial file.
	Size int64

	// CanResume indicates if the server supports resume.
	CanResume bool

	// TotalSize is the total size if known.
	TotalSize int64
}

// GetResumeInfo checks if a file can be resumed.
func GetResumeInfo(filePath string) (*ResumeInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ResumeInfo{
				FilePath:  filePath,
				Size:      0,
				CanResume: false,
			}, nil
		}

		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &ResumeInfo{
		FilePath:  filePath,
		Size:      stat.Size(),
		CanResume: stat.Size() > 0,
	}, nil
}

// CheckServerResumeSupport checks if a server supports resume for a URL.
func CheckServerResumeSupport(
	ctx context.Context, url string, client *http.Client,
) (bool, int64, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check Accept-Ranges header.
	acceptRanges := resp.Header.Get("Accept-Ranges")
	canResume := acceptRanges == "bytes"

	return canResume, resp.ContentLength, nil
}

// SetResumeHeader sets the Range header for resuming a download.
func SetResumeHeader(req *http.Request, startByte int64) {
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
}

// IsPartialResponse checks if a response is a partial content response.
func IsPartialResponse(resp *http.Response) bool {
	return resp.StatusCode == http.StatusPartialContent
}
