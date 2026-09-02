package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"github.com/husseinayyed/twivo-media/internal/types"
)

func InitImageCache() {
	cache, err := lru.New[string, *types.ImageResponse](100000) // Cache size of 100,000 entries
	if err != nil {
		fmt.Println("Error creating LRU cache for image responses:", err)
		return
	}
	types.LruCacheNanoId = cache
}
func ImageRoute(c *gin.Context) {
	// Extract the image ID from the URL parameters
	imageID := c.Param("id")
	fmt.Println("Received request for image ID:", imageID)
	if imageID == "" {
		c.JSON(400, gin.H{"error": "Missing required header"})
		return
	}
	imageResponse, found := types.LruCacheNanoId.Get(imageID)
	if found {
		c.Writer.Header().Set("X-IMAGE-WIDTH", strconv.Itoa(int(imageResponse.Width)))
		c.Writer.Header().Set("X-IMAGE-HEIGHT", strconv.Itoa(int(imageResponse.Height)))
	
		c.Writer.Header().Set("X-IMAGE-TYPE", imageResponse.FileType)
		c.Status(200)
		return
	} 
	redisKey := fmt.Sprintf("nano:%v", imageID)
	// Check if the key exists in Redis
	exists, err := redis.RedisClient.Exists(c, redisKey).Result()
	if err != nil {
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}
	if exists > 0 {
		// If the key exists, fetch the hash data from Redis
		hashData, err := redis.RedisClient.HGetAll(c, redisKey).Result()
		if err != nil {
			c.JSON(500, gin.H{"error": "Internal server error"})
			return
		}
		width := hashData["width"]
		height := hashData["height"]

		widthUint, err1 := strconv.ParseUint(width, 10, 16)
		heightUint, err2 := strconv.ParseUint(height, 10, 16)
		if err1 != nil || err2 != nil {
			c.JSON(500, gin.H{"error": "Internal server error"})
			return
		}
		types.LruCacheNanoId.Add(imageID, &types.ImageResponse{
			Width:  uint16(widthUint),
			Height: uint16(heightUint),
			FileType: hashData["file_type"],
		})
		c.Writer.Header().Set("X-IMAGE-WIDTH", width)
		c.Writer.Header().Set("X-IMAGE-HEIGHT", height)
		c.Writer.Header().Set("X-IMAGE-TYPE", hashData["file_type"])
		c.Status(200)
		return

	} else {
		fmt.Println("Image not found in Redis for ID:", imageID)
		c.JSON(404, gin.H{"error": "Image not found"})
		return
	}
}
