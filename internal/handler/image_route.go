package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/husseinayyed/twivo-media/internal/cache"
	"github.com/husseinayyed/twivo-media/internal/database/mongodb"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	IMGPROXY_URL   = os.Getenv("IMGPROXY_URL")
	WEED_FILER_URL = os.Getenv("WEED_FILER_URL")
	imgproxyProxy  *httputil.ReverseProxy
)

const internalServerErrorMessage = "Internal server error"

func init() {
	if IMGPROXY_URL == "" {
		log.Fatalln("IMGPROXY_URL enviroment variable must be set")
	}
	u, err := url.ParseRequestURI(IMGPROXY_URL)
	if err != nil {
		log.Fatalln("Failed to parse URL")
	}
	imgproxyProxy = httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
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
		v.Width, v.Height, WEED_FILER_URL, targetID, v.FileType)
	c.Request.URL.Path = path
	imgproxyProxy.ServeHTTP(c.Writer, c.Request)
}

func ImageRoute(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
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
		c.JSON(500, gin.H{"error": internalServerErrorMessage})
		return
	}

	if exists > 0 {
		hashData, err := redis.RedisClient.HGetAll(c, redisKey).Result()
		if err != nil {
			c.JSON(500, gin.H{"error": internalServerErrorMessage})
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
	} else {
		image, err := mongodb.GetImage(imageID)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				c.JSON(404, gin.H{"error": "Image not found"})
				return
			}
			c.JSON(500, gin.H{"error": internalServerErrorMessage})
			return
		}

		data := &cache.ImageResponse{
			Width:     uint16(image.Width),
			Height:    uint16(image.Height),
			FileType:  image.FileType,
			BelongsTo: image.BelongsTo,
		}

		cache.LruCacheNanoId.Add(imageID, data)
		nanoKey := fmt.Sprintf("nano:%v", image.NanoId)

		// Prepare the event payload
		eventData := map[string]any{
			"user_id":    image.OwnerId,
			"tweet_id":   image.TweetId,
			"file_uuid":  image.NanoId,
			"belongs_to": image.BelongsTo,
			"file_type":  image.FileType,
			"width":      image.Width,
			"height":     image.Height,
		}

		// Use pipeline for atomic operations
		pipe := redis.RedisClient.TxPipeline()

		// Store the hash data safely
		pipe.HSet(ctx, nanoKey, eventData)

		// Set a 24-hour TTL on the hash key so Nginx can read it within that window
		pipe.Expire(ctx, nanoKey, 24*time.Hour)

		// Execute pipeline
		_, err = pipe.Exec(ctx)
		if err != nil {
			c.JSON(500, gin.H{"error": internalServerErrorMessage})
			return
		}
        ServeImageDirect(c,imageID,data)
	}
    

}
