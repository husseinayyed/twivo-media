package types

import lru "github.com/hashicorp/golang-lru/v2"

type ImageResponse struct {
	Width  uint16
	Height uint16
	BelongsTo string
	OwnerId string
	FileUUID string
	TweetId string
	FileType string
}

var (
	LruCacheNanoId *lru.Cache[string, *ImageResponse] // LRU cache for storing recently validated JWT IDs
)