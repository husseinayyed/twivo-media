package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/husseinayyed/twivo-media/internal/reader"
	"github.com/husseinayyed/twivo-media/internal/storage"
	"github.com/husseinayyed/twivo-media/internal/tasks"
	"github.com/husseinayyed/twivo-media/internal/utils"
	"github.com/husseinayyed/twivo-media/internal/worker"
	gonanoid "github.com/matoous/go-nanoid/v2"
	_ "golang.org/x/image/webp"
)

const (
	MagicBytesWindow    = 16
	ChunkSize           = 4096
	MaxSize             = 20 * 1024 * 1024 // 20 MB max file upload size
	RequestTotalTimeout = 30 * time.Second
	ChunkReadTimeout    = 5 * time.Second
)

var (
	chunkPool = sync.Pool{
		New: func() any {
			buf := make([]byte, ChunkSize)
			return &buf
		},
	}
	w   *worker.Worker
	err error
)

func InitWorker() {
	// Initialize the chunk pool with a predefined number of buffers
	w, err = worker.NewWorker()
	go func() {
		if err != nil {
			fmt.Println("Error worker,", err)
			os.Exit(1)
			return
		}
		w.Start()

	}()
}
func UploadRoute(c *gin.Context) {
	userID := c.GetHeader("X-USER-ID")
	tweetID := c.GetHeader("X-TWEET-ID")
	// Check for missing required headers and return a 400 error if they are not provided
	if userID == "" || tweetID == "" {
		c.JSON(400, gin.H{"error": "Missing required headers"})
		return
	}

	fileUUID, err := gonanoid.New(16) // Generate a unique identifier for the uploaded file
	hasher := sha256.New()            // Create a new SHA-256 hash instance
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate file identifier"})
		return
	}

	// Setting up a context with a total timeout for the request
	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTotalTimeout)
	defer cancel()

	// Limiting the request body to prevent excessive memory usage and wrapping it with a timeout reader
	limitedBody := io.LimitReader(c.Request.Body, MaxSize)
	timeoutStream := reader.NewTimeoutReader(limitedBody, ChunkReadTimeout) // Wrap the limited body with a timeout reader
	headerBuf := make([]byte, MagicBytesWindow)                             // Buffer to hold the first 16 bytes for file type detection
	n, err := io.ReadFull(timeoutStream, headerBuf)                         // Read the first 16 bytes to detect the file type
	if err != nil || n < 16 {
		c.JSON(400, gin.H{"error": "File too small or read error"})
		return
	} // Read the first 16 bytes to detect the file type
	fileType, err := utils.DetectFileType(headerBuf) // Detect the file type based on the magic bytes
	if err != nil || fileType == "null" {
		c.JSON(400, gin.H{"error": "Unsupported file type"})
		return
	}
	limitedheader := io.LimitReader(timeoutStream, 128*1024)                // Limit to 128KB for header detection
	fullReader := io.MultiReader(bytes.NewReader(headerBuf), limitedheader) // Combine the header and the rest of the stream for further processing
	recReader := reader.NewRecordingReader(fullReader)                      // Wrap the combined reader with a recording reader to capture the data for later use

	// Using a buffer from the pool to read the first chunk
	bufPtr := chunkPool.Get().(*[]byte)
	defer func() {
		chunkPool.Put(bufPtr)
	}()
	baseBuffer := *bufPtr
	// validate the image dimensions using the recorded data from the recording reader
	config, decodeErr := utils.ValidateImageDimensions(recReader)
	if decodeErr != nil {
		c.JSON(400, gin.H{"error": decodeErr.Error()})
		return
	}

	// Use io.MultiReader to combine the first chunk and the remaining stream for uploading
	uploadStream := io.MultiReader(recReader.GetRecorded(), timeoutStream)
	teeReader := io.TeeReader(uploadStream, hasher) // Create a TeeReader to compute the SHA-256 hash while reading the upload stream

	var totalBytesRead int64
	totalBytesRead = recReader.Len() // Initialize totalBytesRead with the bytes already read during detection
	// Stream the file to Weed Filer and handle potential errors, including cleanup of orphaned files
	targetFilename, uploadErr := storage.StreamToWeedFiler(ctx, fileUUID, fileType, func(pw io.Writer) error {
		for {
			// Check if the context has been canceled or timed out before each read operation
			if err := ctx.Err(); err != nil {
				return err
			}
			// Use a buffer from the pool for each chunk read
			workingChunk := baseBuffer[0:ChunkSize]
			rn, rErr := teeReader.Read(workingChunk)

			if rn > 0 {
				totalBytesRead += int64(rn)
				// Write the read chunk to the Weed Filer
				if _, writeErr := pw.Write(workingChunk[:rn]); writeErr != nil {
					return writeErr
				}
			}
			// Handle read errors, including EOF and timeout scenarios
			if rErr != nil {
				if rErr == io.EOF {
					return nil
				}
				return rErr
			}
		}
	})

	if uploadErr != nil {
		if targetFilename != "" {
			// Attempt to delete the orphaned file in a separate goroutine to avoid blocking the response
			go storage.DeleteOrphanFile(targetFilename)
		}
		fmt.Println("Error uploading file to Weed Filer:", uploadErr)
		c.JSON(502, gin.H{"error": "Failed to persist file in storage backend"})
		return
	}
	// Schedule the upload task for further processing
	if w == nil || w.Client == nil {
		c.JSON(500, gin.H{"error": "worker unavailable"})
		return
	}
	originalUUID,_, isRepeated, isOwner,err := utils.CheckFileIfRepeated(c, ctx, hex.EncodeToString(hasher.Sum(nil)), fileUUID)

	if err != nil {
		// Log the error but don't fail the upload (the file is already uploaded)
		fmt.Printf("⚠️ Checksum check failed: %v\n", err)
		// Continue with normal flow (upload as new file)
	}

	fmt.Println(originalUUID,isRepeated,isOwner)
	if isRepeated && isOwner {
		go storage.DeleteOrphanFile(targetFilename)
		// Return a successful response with the file details

		c.JSON(200, gin.H{
			"status":          "success",
			"file_url":        fileUUID,
			"bytes_processed": totalBytesRead,
			"message":         "Upload successful, file already exists and belongs to you",
		})

		return
	}
	
	data := tasks.UploadPayload{
		FileUUID:  fileUUID,
		FileType:  fileType,
		TweetID:   tweetID,
		UserID:    userID,
		Width:     fmt.Sprintf("%d", config.Width),
		Height:    fmt.Sprintf("%d", config.Height),
		BelongsTo: originalUUID,
	}
	if err := tasks.ScheduleUploadTask(w.Client, data); err != nil {
		c.JSON(500, gin.H{"error": "failed to schedule upload task"})
		return
	}
	if isRepeated && !isOwner {
		go storage.DeleteOrphanFile(targetFilename)
		// Return a successful response with the file details
		
		c.JSON(200, gin.H{
			"status":          "success",
			"file_url":        fileUUID,
			"bytes_processed": totalBytesRead,
			"message":         "Upload successful, file already exists but does not belong to you",
		})
		return
	}
	// Return a successful response with the file details
	c.JSON(200, gin.H{
		"status":          "success",
		"file_url":        fileUUID,
		"bytes_processed": totalBytesRead,
	})
}
