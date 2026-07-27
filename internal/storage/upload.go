package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/husseinayyed/twivo-media/internal/database/redis"
)

const (
	WeedFilerURL     = "http://weed-filer:8888/buckets/twivo/"
	ChunkSize        = 4082
	ChunkReadTimeout = 5 * time.Second
	MaxSize          = 20 * 1024 * 1024 // 20 MB max file upload size
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

// UploadFileToWeedFiler orchestrates chunked reading with stall timeouts, checks max size limits, and streams to WeedFiler.
func UploadFileToWeedFiler(ctx context.Context, userID, tweetID, fileUUID, fileType string, fullStream io.Reader) (string, int64, error) {
	chunkBuffer := make([]byte, ChunkSize)
	var totalBytesRead int64

	targetFilename, uploadErr := StreamToWeedFiler(ctx, userID, tweetID, fileUUID, fileType, func(pw io.Writer) error {
		pipeWriter, ok := pw.(*io.PipeWriter)
		if ok {
			defer pipeWriter.Close()
		}

		for {
			if err := ctx.Err(); err != nil {
				return err
			}

			readChan := make(chan struct {
				n   int
				err error
			}, 1)

			go func(buf []byte) {
				rn, rErr := fullStream.Read(buf)
				readChan <- struct {
					n   int
					err error
				}{rn, rErr}
			}(chunkBuffer)

			chunkTimer := time.NewTimer(ChunkReadTimeout)

			select {
			case <-ctx.Done():
				chunkTimer.Stop()
				return ctx.Err()
			case <-chunkTimer.C:
				chunkTimer.Stop()
				return fmt.Errorf("chunk read stall timeout exceeded")
			case res := <-readChan:
				chunkTimer.Stop()
				n := res.n
				readErr := res.err

				if n > 0 {
					totalBytesRead += int64(n)
					if totalBytesRead > MaxSize {
						return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", MaxSize)
					}

					currentChunk := chunkBuffer[:n]
					if _, writeErr := pw.Write(currentChunk); writeErr != nil {
						return writeErr
					}
				}

				if readErr != nil {
					if readErr == io.EOF {
						return nil
					}
					return readErr
				}
			}
		}
	})

	if uploadErr != nil {
		if targetFilename != "" {
			go DeleteOrphanFile(targetFilename)
		}
		return "", 0, uploadErr
	}
	redis.RedisClient.Set(ctx, targetFilename, totalBytesRead, 0)
	return targetFilename, totalBytesRead, nil
}