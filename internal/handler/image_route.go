package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"github.com/husseinayyed/twivo-media/internal/types"
)

var imgproxyProxy = httputil.NewSingleHostReverseProxy(&url.URL{
    Scheme: "http",
    Host:   "imgproxy:8080",
})

func init() {
    imgproxyProxy.Director = func(req *http.Request) {
        req.URL.Scheme = "http"
        req.URL.Host = "imgproxy:8080"
        req.Header.Del("Accept-Encoding")
    }
}

func ServeImageDirect(c *gin.Context, imageID string, v *types.ImageResponse) {
    targetID := v.BelongsTo

    path := fmt.Sprintf("/unsafe/resize:fit:%d:%d/f:webp/plain/http://weed-filer:8888/buckets/twivo/%s%s",
        v.Width, v.Height, targetID, v.FileType)
    c.Request.URL.Path = path
    imgproxyProxy.ServeHTTP(c.Writer, c.Request)
}

func InitImageCache() {
    cache, err := lru.New[string, *types.ImageResponse](100000)
    if err != nil {
        fmt.Println("Error creating LRU cache for image responses:", err)
        return
    }
    types.LruCacheNanoId = cache
}

func ImageRoute(c *gin.Context) {
    imageID := c.Param("id")
    if imageID == "" {
        c.JSON(400, gin.H{"error": "Missing required parameter"})
        return
    }

    // 1. LRU Cache
    if imageResponse, found := types.LruCacheNanoId.Get(imageID); found {
        ServeImageDirect(c, imageID, imageResponse)
        return
    }

    // 2. Redis
    redisKey := fmt.Sprintf("nano:%v", imageID)
    exists, err := redis.RedisClient.Exists(c, redisKey).Result()
    if err != nil {
        c.JSON(500, gin.H{"error": "Internal server error"})
        return
    }

    if exists > 0 {
        hashData, err := redis.RedisClient.HGetAll(c, redisKey).Result()
        if err != nil {
            c.JSON(500, gin.H{"error": "Internal server error"})
            return
        }

        width, _ := strconv.ParseUint(hashData["width"], 10, 16)
        height, _ := strconv.ParseUint(hashData["height"], 10, 16)
        belongsTo := hashData["belongs_to"]
        fileType := hashData["file_type"]

        data := &types.ImageResponse{
            Width:     uint16(width),
            Height:    uint16(height),
            FileType:  fileType,
            BelongsTo: belongsTo,
        }

        types.LruCacheNanoId.Add(imageID, data)
        ServeImageDirect(c, imageID, data)
        return 
    }
    c.JSON(404, gin.H{"error": "Image not found"})
}