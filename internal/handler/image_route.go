package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/husseinayyed/twivo-media/internal/cache"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
)

var (
    IMGPROXY_URL = os.Getenv("IMGPROXY_URL")
    WEED_FILER_URL = os.Getenv("WEED_FILER_URL")
   imgproxyProxy  *httputil.ReverseProxy
)

func init() {
    if IMGPROXY_URL == "" {
        panic("IMGPROXY_URL enviroment variable must be set")
    }
    u, err := url.ParseRequestURI(IMGPROXY_URL)
	if err != nil {
		panic("Failed to parse URL")
	}
    imgproxyProxy = httputil.NewSingleHostReverseProxy(&url.URL{
    Scheme: u.Scheme,
    Host: u.Host,
})
    imgproxyProxy.Director = func(req *http.Request) {
        req.URL.Scheme = u.Scheme
        req.URL.Host = u.Host
        req.Header.Del("Accept-Encoding")
    }
}

func ServeImageDirect(c *gin.Context, imageID string, v *cache.ImageResponse) {
    targetID := v.BelongsTo

    path := fmt.Sprintf("/unsafe/resize:fit:%d:%d/f:webp/plain/%s/buckets/twivo/%s%s",
        v.Width, v.Height,WEED_FILER_URL, targetID, v.FileType)
    c.Request.URL.Path = path
    imgproxyProxy.ServeHTTP(c.Writer, c.Request)
}



func ImageRoute(c *gin.Context) {
    imageID := c.Param("id")
    if imageID == "" {
        c.JSON(400, gin.H{"error": "Missing required parameter"})
        return
    }

    // 1. LRU Cache
    if imageResponse, found := cache.LruCacheNanoId.Get(imageID); found {
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

        data := &cache.ImageResponse{
            Width:     uint16(width),
            Height:    uint16(height),
            FileType:  fileType,
            BelongsTo: belongsTo,
        }

        cache.LruCacheNanoId.Add(imageID, data)
        ServeImageDirect(c, imageID, data)
        return 
    }
    c.JSON(404, gin.H{"error": "Image not found"})
}