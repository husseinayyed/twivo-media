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

	"sync"

	"github.com/gin-gonic/gin"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"github.com/husseinayyed/twivo-media/internal/storage"
	"github.com/husseinayyed/twivo-media/internal/tasks"
	"github.com/husseinayyed/twivo-media/internal/worker"
	gonanoid "github.com/matoous/go-nanoid/v2"
	_ "golang.org/x/image/webp"
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
    _,err := redis.ConnectRedis()
    if err != nil {
            fmt.Println("Error worker,", err)
            return
        }
    chunkPool := sync.Pool{
        New: func() any {
            buf := make([]byte, ChunkSize)
            return &buf
        },
    }
    w, err := worker.NewWorker()
    go func() {
        if err != nil {
            fmt.Println("Error worker,", err)
            return
        }
        w.Start()
    }()

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
               // Stream reader for uploading
        fullStream := bytes.NewReader(bodyBytes)
        
        // 1. FIXED: Get from pool and type assert to the actual pointer type (*[]byte)
        bufPtr := chunkPool.Get().(*[]byte)
        
        // 2. FIXED: Ensure buffer reference goes back to pool when endpoint finishes
        defer func() {
            chunkPool.Put(bufPtr)
        }()

        // Dereference once here to get our base backing slice container
        baseBuffer := *bufPtr
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

                // 3. FIXED: Allocate a dedicated sub-slice window for THIS loop iteration.
                // This gives the background goroutine its own localized slice header 
                // preventing data races if the loop moves forward on a timeout.
                workingChunk := baseBuffer[0:ChunkSize]

                readChan := make(chan struct {
                    n   int
                    err error
                }, 1)

                // 4. FIXED: Pass the local sub-slice copy explicitly into the goroutine
                go func(buf []byte) {
                    rn, rErr := fullStream.Read(buf)
                    select {
                    case readChan <- struct {
                        n   int
                        err error
                    }{rn, rErr}:
                    default:
                    }
                }(workingChunk)

                chunkTimer := time.NewTimer(ChunkReadTimeout)

                select {
                case <-ctx.Done():
                    chunkTimer.Stop()
                    return ctx.Err()
                case <-chunkTimer.C:
                    chunkTimer.Stop()
                    // 5. CRITICAL: If a stall occurs, abandon this buffer completely! 
                    // Do NOT return it to the pool because the background goroutine 
                    // might still write to it later, which would corrupt future requests.
                    bufPtr = chunkPool.New().(*[]byte) 
                    baseBuffer = *bufPtr
                    return fmt.Errorf("chunk read stall timeout exceeded")
                case res := <-readChan:
                    chunkTimer.Stop()
                    n := res.n
                    readErr := res.err

                    if n > 0 {
                        totalBytesRead += int64(n)
                        // Slice out exactly what was read from our thread-isolated chunk
                        currentChunk := workingChunk[:n]

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
       data := tasks.UploadPayload{
        FileUUID: fileUUID,
        FileType: fileType,
        TweetID:  tweetID,
        UserID:   userID,
        Width:    fmt.Sprintf("%d", config.Width),
        Height:   fmt.Sprintf("%d", config.Height),
    }
       tasks.ScheduleUploadTask(w.Client,data)

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