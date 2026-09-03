package cache
import (
	"github.com/hashicorp/golang-lru/v2"
)
type CheckSumResponse struct {
	NanoId string
	UserId string
}
var (
	// LruCacheNanoId is a global variable that holds the LRU cache instance for image responses.
	LruCacheCheckSum *lru.Cache[string, *CheckSumResponse]
)

func InitCache() {
	cache, err := lru.New[string, *CheckSumResponse](100000) // Cache size of 100,000 entries
	if err != nil {
		panic("Error creating LRU cache for image responses: " + err.Error())
	}
	LruCacheCheckSum = cache
}



