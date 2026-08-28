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
 "sync"
 "time"

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

// TimeoutReader wraps an io.Reader and enforces a timeout for each read operation.
type TimeoutReader struct {
 r       io.Reader
 timeout time.Duration
}

func NewTimeoutReader(r io.Reader, timeout time.Duration) *TimeoutReader {
 return &TimeoutReader{r: r, timeout: timeout}
}

func (tr *TimeoutReader) Read(p []byte) (int, error) {
 type result struct {
  n   int
  err error
 }
 ch := make(chan result, 1)

 go func() {
  n, err := tr.r.Read(p)
  select {
  case ch <- result{n: n, err: err}:
  default:
  }
 }()
// Create a timer that will trigger after the specified timeout duration
 timer := time.NewTimer(tr.timeout)
 defer timer.Stop()

 select { 
 case <-timer.C:
  return 0, fmt.Errorf("chunk read stall timeout exceeded")
 case res := <-ch:
  return res.n, res.err
 }
}

func main() {
 router := gin.Default()
 port := "8020"
 _, err := redis.ConnectRedis()
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

  // Setting up a context with a total timeout for the request
  ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTotalTimeout)
  defer cancel()

  // Limiting the request body to prevent excessive memory usage and wrapping it with a timeout reader
  limitedBody := io.LimitReader(c.Request.Body, MaxSize)
  timeoutStream := NewTimeoutReader(limitedBody, ChunkReadTimeout)
  // Using a buffer from the pool to read the first chunk
  bufPtr := chunkPool.Get().(*[]byte)
  hasTimedOut := false
  defer func() {
    // Return the buffer to the pool if it hasn't timed out
   if !hasTimedOut {
    chunkPool.Put(bufPtr)
   }
  }()
  baseBuffer := *bufPtr
  // Read the first chunk to detect file type and validate image dimensions
  workingChunk := baseBuffer[0:ChunkSize]
  // Read the first chunk to detect file type and validate image dimensions
  n, readErr := timeoutStream.Read(workingChunk)
  if n == 0 || (readErr != nil && readErr != io.EOF) {
   c.JSON(400, gin.H{"error": "Invalid or empty file upload or stall timeout"})
   return
  }
 // Slice the working chunk to the actual number of bytes read
  firstChunk := workingChunk[:n]
 
  inspectLen := MagicBytesWindow
  // Adjust the inspection length if the first chunk is smaller than the defined window
  if len(firstChunk) < inspectLen {
   inspectLen = len(firstChunk)
  }
  fileType, err := detectFileType(firstChunk[:inspectLen])
  if err != nil || fileType == "null" {
   c.JSON(400, gin.H{"error": "Invalid or unsupported file type"})
   return
  }
  // Use io.MultiReader to combine the first chunk and the remaining stream for image decoding
  configReader := io.MultiReader(bytes.NewReader(firstChunk), timeoutStream)

  config, _, decodeErr := image.DecodeConfig(configReader)
  if decodeErr != nil {
   c.JSON(400, gin.H{"error": "Invalid or corrupted image structure"})
   return
  }
  // Validate image dimensions against the defined constraints
  if config.Width < MinImageWidth || config.Height < MinImageHeight ||
   config.Width > MaxImageWidth || config.Height > MaxImageHeight {
   c.JSON(400, gin.H{
    "error": fmt.Sprintf("Image dimensions must be between %dx%d and %dx%d pixels", MinImageWidth, MinImageHeight, MaxImageWidth, MaxImageHeight),
   })
   return
  }
  // Use io.MultiReader to combine the first chunk and the remaining stream for uploading
  uploadStream := io.MultiReader(bytes.NewReader(firstChunk), timeoutStream)
  var totalBytesRead int64
  // Stream the file to Weed Filer and handle potential errors, including cleanup of orphaned files
  targetFilename, uploadErr := storage.StreamToWeedFiler(ctx, fileUUID, fileType, func(pw io.Writer) error {
   for {
    // Check if the context has been canceled or timed out before each read operation
    if err := ctx.Err(); err != nil {
     return err
    }
    // Use a buffer from the pool for each chunk read
    workingChunk := baseBuffer[0:ChunkSize]
    rn, rErr := uploadStream.Read(workingChunk)

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
     // If the read error is due to a timeout, set the hasTimedOut flag to true to prevent returning the buffer to the pool
     hasTimedOut = true
     return rErr
    }
   }
  })

  if uploadErr != nil {
   if targetFilename != "" {
    // Attempt to delete the orphaned file in a separate goroutine to avoid blocking the response
    go storage.DeleteOrphanFile(targetFilename)
   }
   c.JSON(502, gin.H{"error": "Failed to persist file in storage backend"})
   return
  }
  // Schedule the upload task for further processing
  data := tasks.UploadPayload{
   FileUUID: fileUUID,
   FileType: fileType,
   TweetID:  tweetID,
   UserID:   userID,
   Width:    fmt.Sprintf("%d", config.Width),
   Height:   fmt.Sprintf("%d", config.Height),
  }
  tasks.ScheduleUploadTask(w.Client, data)
  // Return a successful response with the file details
  c.JSON(200, gin.H{
   "status":          "success",
   "file_url":        fileUUID,
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