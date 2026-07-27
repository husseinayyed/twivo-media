package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	lru "github.com/hashicorp/golang-lru/v2"
	gonanoid "github.com/matoous/go-nanoid/v2"
	_ "golang.org/x/image/webp"

	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"github.com/husseinayyed/twivo-media/internal/storage"
)

const (
	MagicBytesWindow = 16
	ChunkSize        = 4082
	MaxSize          = 20 * 1024 * 1024 // 20 MB max file upload size

	MinImageWidth  = 100
	MinImageHeight = 100
	MaxImageWidth  = 2048
	MaxImageHeight = 2048

	RequestTotalTimeout = 30 * time.Second
	ChunkReadTimeout    = 5 * time.Second
)

var (
	JpegMagic = []byte{0xFF, 0xD8, 0xFF}
	PngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	WebpMagic = []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
)

func main() {
	_, err := redis.ConnectRedis()
	if err != nil {
		fmt.Println("Error connecting to Redis:", err)
		os.Exit(1)
	}
	router := gin.Default()
	port := "8020"

	cache, err := lru.New[string, string](100)
	if err != nil {
		fmt.Println("Error creating LRU cache:", err)
		os.Exit(1)
	}
	_ = cache

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"ping": "pong"})
	})

	router.POST("/upload", func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		tweetID := c.GetHeader("X-Tweet-ID")

		if userID == "" || tweetID == "" {
			c.JSON(400, gin.H{"error": "Missing required headers"})
			return
		}

		fileUUID, err := gonanoid.New(16)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to generate file identifier"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTotalTimeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		body := io.LimitReader(c.Request.Body, MaxSize)
		defer c.Request.Body.Close()

		magicBytes := make([]byte, MagicBytesWindow)
		n, err := io.ReadFull(body, magicBytes)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			c.JSON(500, gin.H{"error": "Failed to read request body"})
			return
		}

		inspectedBytes := magicBytes[:n]
		fileType, err := detectFileType(inspectedBytes)
		if err != nil || fileType == "null" {
			c.JSON(400, gin.H{"error": "Invalid or unsupported file type"})
			return
		}

		fullStream := io.MultiReader(bytes.NewReader(inspectedBytes), body)

		config, format, decodeErr := image.DecodeConfig(fullStream)
		if decodeErr != nil {
			c.JSON(400, gin.H{"error": "Invalid or corrupted image structure"})
			return
		}

		if config.Width < MinImageWidth || config.Height < MinImageHeight ||
			config.Width > MaxImageWidth || config.Height > MaxImageHeight {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("Image dimensions must be between %dx%d and %dx%d pixels", MinImageWidth, MinImageHeight, MaxImageWidth, MaxImageHeight),
			})
			return
		}

		fullStream = io.MultiReader(bytes.NewReader(inspectedBytes), body)

		UploadCtx, uploadCancel := context.WithTimeout(ctx, RequestTotalTimeout)
		defer uploadCancel()

		targetFilename, totalBytesRead, uploadErr := storage.UploadFileToWeedFiler(UploadCtx, userID, tweetID, fileUUID, fileType, fullStream)
		if uploadErr != nil {
			c.JSON(500, gin.H{"error": uploadErr.Error()})
			return
		}

		c.JSON(200, gin.H{
			"status":          "success",
			"file_url":        targetFilename,
			"file_type":       fileType,
			"detected_format": format,
			"width":           config.Width,
			"height":          config.Height,
			"bytes_processed": totalBytesRead,
		})
	})

	router.Run(fmt.Sprintf(":%v", port))
}

func detectFileType(buf []byte) (string, error) {
	switch {
	case bytes.HasPrefix(buf, JpegMagic):
		return ".jpeg", nil
	case bytes.HasPrefix(buf, PngMagic):
		return ".png", nil
	case len(buf) >= 12 && bytes.Equal(buf[0:4], WebpMagic[0:4]) && bytes.Equal(buf[8:12], WebpMagic[8:12]):
		return ".webp", nil
	default:
		return "null", nil
	}
}