package store

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	WeedFilerURL = "http://weed-filer:8888/buckets/twivo/"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// StreamToWeedFiler handles uploading an incoming data stream directly to SeaweedFS Filer via HTTP PUT using an io.Pipe.
func StreamToWeedFiler(ctx context.Context, userID, tweetID, fileUUID, fileType string, populateStream func(pw io.Writer) error) (string, error) {
	pr, pw := io.Pipe()

	targetFilename := fmt.Sprintf("%s/%s/%s%s", userID, tweetID, fileUUID, fileType)
	uploadURL := WeedFilerURL + targetFilename

	uploadErrChan := make(chan error, 1)

	go func() {
		defer pr.Close()
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, pr)
		if reqErr != nil {
			uploadErrChan <- reqErr
			return
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, respErr := httpClient.Do(req)
		if respErr != nil {
			uploadErrChan <- respErr
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			uploadErrChan <- fmt.Errorf("weedfiler returned status: %d", resp.StatusCode)
			return
		}
		uploadErrChan <- nil
	}()

	// Pass `pw` directly since *io.PipeWriter implements io.Writer
	if err := populateStream(pw); err != nil {
		pw.CloseWithError(err)
		return "", err
	}

	if err := <-uploadErrChan; err != nil {
		return "", err
	}

	return targetFilename, nil
}

// DeleteOrphanFile removes an incomplete or rejected file upload from SeaweedFS Filer
func DeleteOrphanFile(fileURL string) {
	deleteURL := WeedFilerURL + fileURL

	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}