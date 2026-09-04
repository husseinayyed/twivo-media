package cache

import (
	"github.com/hashicorp/golang-lru/v2"
)

type ImageResponse struct {
	Width     uint16
	Height    uint16
	BelongsTo string
	OwnerId   string
	FileUUID  string
	TweetId   string
	FileType  string
}
type CheckSumResponse struct {
	NanoId string
	UserId string
}

var (
	LruCacheCheckSum *lru.Cache[string, *CheckSumResponse] // LruCacheNanoId is a global variable that holds the LRU cache instance for image responses.
	LruCacheNanoId   *lru.Cache[string, *ImageResponse]    // LRU cache for storing recently validated JWT IDs
	LruCacheToken    *lru.Cache[string, bool]              // LRU cache for storing recently validated JWT IDs
	LruCacheJTI *lru.Cache[string, bool]				   // LRU cache for storing recently validated JWT IDs
)

func InitCache() {
	lru1, err1 := lru.New[string, *CheckSumResponse](100000) // Cache size of 100,000 entries
	lru2, err2 := lru.New[string, *ImageResponse](100000) // Cache size of 100,000 entries
	lru3, err3 := lru.New[string, bool](10000) // Cache size of 10,000 entries
	lru4, err4 := lru.New[string, bool](10000) // Cache size of 10,000 entries
	errs := []error{err1, err2, err3, err4}

	// 3. Automatically check if any one of them failed
	for _, err := range errs {
		if err != nil {
			panic("Error creating LRU cache: " + err.Error())
		}
	}
	LruCacheCheckSum = lru1
	LruCacheNanoId = lru2
	LruCacheToken = lru3
	LruCacheJTI = lru4
}
