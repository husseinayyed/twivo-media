package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

)

const (
	ChunkSize        = 4082
	ChunkReadTimeout = 5 * time.Second
	MaxSize          = 20 * 1024 * 1024 // 20 MB max file upload size
)

var (
	WeedFilerURL = getDefaultWeedFilerURL()
)

func getDefaultWeedFilerURL() string {
	if url := os.Getenv("WEED_FILER_URL"); url != "" {
		return url
	}
	return "http://weed-filer:8888"
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func normalizeUploadError(err error) error {
	fmt.Println("Error: ",err)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("upload canceled before completion")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("upload timed out while streaming to storage")
	default:
		return err
	}
}

// StreamToWeedFiler handles uploading an incoming data stream directly to SeaweedFS Filer via HTTP PUT using an io.Pipe.
func StreamToWeedFiler(ctx context.Context, fileUUID, fileType string, populateStream func(pw io.Writer) error) (string, error) {
	pr, pw := io.Pipe()
	targetFilename := fmt.Sprintf("%s%s",fileUUID, fileType)
	fmt.Println("STREAM DEBUG targetFilename:", targetFilename)
	// If SEAWEEDFS_FILER_URL = "http://weed-filer:8888"
    uploadURL := fmt.Sprintf("%s/buckets/twivo/%s", WeedFilerURL, targetFilename)
	uploadErrChan := make(chan error, 1)

	go func() {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, pr)
		if reqErr != nil {
			fmt.Println("reqErr")
			uploadErrChan <- reqErr
			return
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, respErr := httpClient.Do(req)
		fmt.Println(respErr)
		if respErr != nil {
			fmt.Println("respErr")
			uploadErrChan <- respErr
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			fmt.Println("Weedfiler")
			uploadErrChan <- fmt.Errorf("weedfiler returned status: %d", resp.StatusCode)
			return
		}
		uploadErrChan <- nil
	}()

	if err := populateStream(pw); err != nil {
		if ctx.Err() != nil {
			fmt.Println("ctx.error")
			pw.CloseWithError(ctx.Err())
			return "", normalizeUploadError(ctx.Err())
		}
		if errors.Is(err, io.ErrClosedPipe) || strings.Contains(err.Error(), "closed pipe") {
			fmt.Println("ctx 2error")
			pw.CloseWithError(ctx.Err())
			return "", normalizeUploadError(ctx.Err())
		}
		fmt.Printf("ctx3Err")
		pw.CloseWithError(err)
		return "", normalizeUploadError(err)
	}

	if err := <-uploadErrChan; err != nil {
		fmt.Println("ctx4error")
		return "", normalizeUploadError(err)
	}
	fmt.Println("Uploader Debug. Target file is :",targetFilename)
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