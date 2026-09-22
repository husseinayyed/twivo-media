package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/husseinayyed/twivo-media/internal/breaker"
	"github.com/rs/dnscache"
	"github.com/rs/zerolog/log"
)

var (
	WeedFilerURL = getDefaultWeedFilerURL()
	r            = &dnscache.Resolver{}
)

func getDefaultWeedFilerURL() string {
	url := os.Getenv("WEED_FILER_URL")
	if url == "" {
		log.Fatal().Msg("WEED_FILER_URL environment variable must be set")
	}

	return url
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
	result, err := breaker.SeaweedFS.Execute(func() (any, error) {
		pr, pw := io.Pipe()
		targetFilename := fmt.Sprintf("%s%s", fileUUID, fileType)

		// Structured bucket pathway mapping
		bucketPath := fmt.Sprintf("/buckets/twivo/%s", targetFilename)
		uploadURL := WeedFilerURL + bucketPath

		// Use a single-slot buffered channel so the goroutine can always report completion or error
		// without blocking. This keeps the upload path from deadlocking when the pipe writer exits
		// early or the server responds quickly.
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
				log.Error().Err(respErr).Msg("error during HTTP request to WeedFiler")
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

		// Populate the pipe writer in the current goroutine so the upload payload is streamed while
		// the HTTP request is concurrently reading from the pipe. This keeps memory usage bounded and
		// preserves the backpressure behavior of the underlying io.Pipe.
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

		// A cancellation can happen while the server is still processing the upload. We fail fast in
		// that case and convert the context error to a normalized domain error instead of surfacing a
		// lower-level transport issue back to the caller.
		select {
		case <-ctx.Done():
			return "", normalizeUploadError(ctx.Err())
		case err := <-uploadErrChan:
			if err != nil {
				return "", normalizeUploadError(err)
			}
		}

		// Return the relative bucket path so downstream code can identify and delete the same artifact
		// without needing to reconstruct the absolute SeaweedFS URL.
		return bucketPath, nil
	})
	if err != nil {
		return "", err
	}

	path, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("seaweedfs breaker returned invalid result type")
	}
	return path, nil
}

// DeleteOrphanFile removes an incomplete or rejected file upload from SeaweedFS Filer
func DeleteOrphanFile(bucketPath string) {
	_, _ = breaker.SeaweedFS.Execute(func() (any, error) {
		// Ensure the path is absolute before constructing the delete URL so the request targets the
		// same bucket layout that was used during upload.
		if !strings.HasPrefix(bucketPath, "/") {
			bucketPath = "/" + bucketPath
		}
		deleteURL := WeedFilerURL + bucketPath

		req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return nil, nil
	})
}
