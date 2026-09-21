package handler

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/rs/zerolog/log"
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
		log.Fatal().Msg("IMGPROXY_URL environment variable must be set")
	}
	u, err := url.ParseRequestURI(IMGPROXY_URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse IMGPROXY_URL")
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

	if imageResponse, found := cache.LruCacheNanoId.Get(imageID); found {
		ServeImageDirect(c, imageID, imageResponse)
		return
	}

	imageResponse, found, err := imageFromRedis(ctx, imageID)
	if err != nil {
		c.JSON(500, gin.H{"error": internalServerErrorMessage})
		return
	}
	if found {
		ServeImageDirect(c, imageID, imageResponse)
		return
	}

	imageResponse, err = imageFromMongo(ctx, imageID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(404, gin.H{"error": "Image not found"})
			return
		}
		c.JSON(500, gin.H{"error": internalServerErrorMessage})
		return
	}
	ServeImageDirect(c, imageID, imageResponse)
}

func imageFromRedis(ctx context.Context, imageID string) (*cache.ImageResponse, bool, error) {
	redisKey := fmt.Sprintf("nano:%v", imageID)
	exists, err := redis.RedisClient.Exists(ctx, redisKey).Result()
	if err != nil {
		return nil, false, err
	}
	if exists == 0 {
		return nil, false, nil
	}

	hashData, err := redis.RedisClient.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, false, err
	}
	width, _ := strconv.ParseUint(hashData["width"], 10, 16)
	height, _ := strconv.ParseUint(hashData["height"], 10, 16)
	data := &cache.ImageResponse{
		Width:     uint16(width),
		Height:    uint16(height),
		FileType:  hashData["file_type"],
		BelongsTo: hashData["belongs_to"],
	}
	cache.LruCacheNanoId.Add(imageID, data)
	return data, true, nil
}

func imageFromMongo(ctx context.Context, imageID string) (*cache.ImageResponse, error) {
	image, err := mongodb.GetImage(imageID)
	if err != nil {
		return nil, err
	}

	data := &cache.ImageResponse{
		Width:     uint16(image.Width),
		Height:    uint16(image.Height),
		FileType:  image.FileType,
		BelongsTo: image.BelongsTo,
	}
	cache.LruCacheNanoId.Add(imageID, data)

	eventData := map[string]any{
		"user_id":    image.OwnerId,
		"tweet_id":   image.TweetId,
		"file_uuid":  image.NanoId,
		"belongs_to": image.BelongsTo,
		"file_type":  image.FileType,
		"width":      image.Width,
		"height":     image.Height,
	}
	redisKey := fmt.Sprintf("nano:%v", image.NanoId)
	pipe := redis.RedisClient.TxPipeline()
	pipe.HSet(ctx, redisKey, eventData)
	pipe.Expire(ctx, redisKey, 24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	return data, nil

}
