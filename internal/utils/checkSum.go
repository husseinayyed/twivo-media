package utils

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/husseinayyed/twivo-media/internal/cache"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	goRedis "github.com/redis/go-redis/v9"
)

// CheckFileIfRepeated checks if a file with the given checksum already exists.
// Returns:
//   - nanoId: the ID of the file to use (new or existing)
//   - belongsTo: if a new ID was created for a different user, this holds the original ID
//   - existingUserId: the user ID who originally uploaded the file
//   - isRepeated: true if the file already exists
//   - isSameUser: true if the file belongs to the same user
//   - err: any error that occurred (Redis failure, etc.)
func CheckFileIfRepeated(c *gin.Context, ctx context.Context, checksumHex string, fileUUID string) (string, string, bool, bool, error) {
	key := fmt.Sprintf("checksum:%s", checksumHex)
	userId := c.GetHeader("X-USER-ID")

	// 1. Check LRU memory cache first (fastest)
	if result, exists := cache.LruCacheCheckSum.Get(key); exists {
		if result.UserId == userId {
			return result.NanoId, result.UserId, true, true, nil
		}
		return result.NanoId, result.UserId, true, false, nil
	}

	// 2. Lua script for atomic get-or-create (prevents race conditions)
	script := goRedis.NewScript(`
    local key = KEYS[1]
    local nanoId = ARGV[1]
    local userId = ARGV[2]

    local exists = redis.call('EXISTS', key)
    if exists == 1 then
        local fields = redis.call('HMGET', key, 'nanoId', 'userId')
        return {1, fields[1] or '', fields[2] or ''}
    else
        redis.call('HSET', key, 'nanoId', nanoId, 'userId', userId)
        redis.call('EXPIRE', key, 86400)
        return {0, nanoId, userId}
    end
`)

	// 3. Execute the script atomically
	result, err := script.Run(ctx, redis.RedisClient, []string{key}, fileUUID, userId).Result()
	if err != nil {
		// Don't send response here; let caller handle it
		return "", "", false, false, fmt.Errorf("redis atomic operation failed: %w", err)
	}

	// 4. Parse the result
	arr, ok := result.([]interface{})
	if !ok || len(arr) < 3 {
		return "", "", false, false, fmt.Errorf("unexpected result from Redis script")
	}

	isExisting := arr[0].(int64) == 1
	existingNanoId := arr[1].(string)
	existingUserId := arr[2].(string)

	// 5. Update LRU cache for faster future lookups
	cache.LruCacheCheckSum.Add(key, &cache.CheckSumResponse{
		NanoId: existingNanoId,
		UserId: existingUserId,
	})

	// 6. Return results
	if isExisting {
		// File exists: return the existing data
		return existingNanoId, existingUserId, true, existingUserId == userId, nil
	}

	// New file: the fileUUID is the new nanoId
	return fileUUID, userId, false, false, nil
}
