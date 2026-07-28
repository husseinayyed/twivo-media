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
    "github.com/husseinayyed/twivo-media/internal/storage"

)

const (
    MagicBytesWindow = 16
    ChunkSize        = 4096
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

        fileUUID, err := gonanoid.New()
        if err != nil {
            c.JSON(500, gin.H{"error": "Failed to generate file identifier"})
            return
        }

        ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTotalTimeout)
        defer cancel()

        // Read the entire body safely into memory up to MaxSize
        bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, MaxSize))
        if err != nil {
            c.JSON(500, gin.H{"error": "Failed to read request body"})
            return
        }
        defer c.Request.Body.Close()

        if int64(len(bodyBytes)) >= MaxSize {
            c.JSON(400, gin.H{"error": "File size exceeds maximum allowed size of 20 MB"})
            return
        }

        // Detect file type from magic bytes
        inspectLen := MagicBytesWindow
        if len(bodyBytes) < inspectLen {
            inspectLen = len(bodyBytes)
        }
        fileType, err := detectFileType(bodyBytes[:inspectLen])
        if err != nil || fileType == "null" {
            c.JSON(400, gin.H{"error": "Invalid or unsupported file type"})
            return
        }

        // Decode image config safely from a fresh reader over the byte slice
        config, format, decodeErr := image.DecodeConfig(bytes.NewReader(bodyBytes))
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

        // Stream reader for uploading
        fullStream := bytes.NewReader(bodyBytes)
        chunkBuffer := make([]byte, ChunkSize)
        var totalBytesRead int64

        targetFilename, uploadErr := storage.StreamToWeedFiler(ctx, fileUUID, fileType, func(pw io.Writer) error {
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
                    select {
                    case readChan <- struct {
                        n   int
                        err error
                    }{rn, rErr}:
                    default:
                    }
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
                go storage.DeleteOrphanFile(targetFilename)
            }
           
            c.JSON(502, gin.H{"error": "Failed to persist file in storage backend"})
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