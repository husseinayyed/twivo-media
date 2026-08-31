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
	"net"
	"github.com/rs/dnscache"
)

var (
	WeedFilerURL = getDefaultWeedFilerURL()
	r = &dnscache.Resolver{}
)

func getDefaultWeedFilerURL() string {
	if url := os.Getenv("WEED_FILER_URL"); url != "" {
		return url
	}
	return "http://weed-filer:8888"
}


var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
        DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
        host, port, err := net.SplitHostPort(addr)
        if err != nil {
            return nil, err
        }
        ips, err := r.LookupHost(ctx, host)
        if err != nil {
            return nil, err
        }
        for _, ip := range ips {
            var dialer net.Dialer
            conn, err = dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
            if err == nil {
                break
            }
        }
        return
    },
    },
    Timeout: 60 * time.Second,
}

func normalizeUploadError(err error) error {
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
	targetFilename := fmt.Sprintf("%s%s", fileUUID, fileType)
	
	// Structured bucket pathway mapping
	bucketPath := fmt.Sprintf("/buckets/twivo/%s", targetFilename)
	uploadURL := WeedFilerURL + bucketPath

	// 1. FIXED: Buffered channel size of 1 ensures the background goroutine can 
	// always emit its result and exit, completely preventing deadlocks.
	uploadErrChan := make(chan error, 1)

	go func() {
		// Ensure the pipe reader is closed on exit to unlock any stuck pipe writers
		defer pr.Close()

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, pr)
		if reqErr != nil {
			uploadErrChan <- reqErr
			return
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, respErr := httpClient.Do(req)
		if respErr != nil {
			fmt.Printf("Error during HTTP request to WeedFiler: %v\n", respErr)
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

	// 2. FIXED: Robust error collection sequence
	populateErr := populateStream(pw)
	if populateErr != nil {
		// Close the pipe with the specific error to forcefully terminate the HTTP client
		pw.CloseWithError(populateErr)
		
		if ctx.Err() != nil {
			return "", normalizeUploadError(ctx.Err())
		}
		if errors.Is(populateErr, io.ErrClosedPipe) || strings.Contains(populateErr.Error(), "closed pipe") {
			return "", normalizeUploadError(ctx.Err())
		}
		return "", normalizeUploadError(populateErr)
	}

	// Safely close the pipe to signal completion to HTTP client
	pw.Close()

	// 3. FIXED: Handle immediate fallback if context cancels while waiting for server response
	select {
	case <-ctx.Done():
		return "", normalizeUploadError(ctx.Err())
	case err := <-uploadErrChan:
		if err != nil {
			return "", normalizeUploadError(err)
		}
	}

	// Return the relative bucket pathway string for clean downstream tracking/deletion
	return bucketPath, nil
}

// DeleteOrphanFile removes an incomplete or rejected file upload from SeaweedFS Filer
func DeleteOrphanFile(bucketPath string) {
	// 4. FIXED: Properly absolute resolves the pathway url match
	if !strings.HasPrefix(bucketPath, "/") {
		bucketPath = "/" + bucketPath
	}
	deleteURL := WeedFilerURL + bucketPath

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
