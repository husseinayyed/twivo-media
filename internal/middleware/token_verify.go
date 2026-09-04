package middleware

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/husseinayyed/twivo-media/internal/cache"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
)

var (
	
	JWTIssuer       = "twivo"
	JWTAudience     = "media"
	PUBLIC_KEY_PATH = "keys/public.pem"
	ErrInvalidToken = errors.New("the provided token is invalid")
    tokenBlockDuration = 3 * time.Minute // 3 minutes in time.Duration nanoseconds

	// Global variable to hold your loaded public key across your application
	PublicSigningKey ed25519.PublicKey
)

func init() {
	// Read and parse your Ed25519 public key file
	b, err := os.ReadFile(PUBLIC_KEY_PATH)
	if err != nil {
		panic("failed to read public_key.pem: " + err.Error())
	}

	block, _ := pem.Decode(b)
	if block == nil {
		panic("failed to decode valid PEM block from public key")
	}

	pubKeyRaw, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("failed to parse PKIX public key: " + err.Error())
	}

	// 3. Store the type-asserted key into your global variable
	var ok bool
	PublicSigningKey, ok = pubKeyRaw.(ed25519.PublicKey)
	if !ok {
		panic("key inside public_key.pem is not a valid Ed25519 public key")
	}
}
func VerifyToken(c *gin.Context) {
	tokenString := c.GetHeader("X-TWIVO-BACKEND")
	ctx := c.Request.Context()
	if tokenString == "" || len(tokenString) < 10 {
		c.AbortWithStatusJSON(401, gin.H{"error": "Missing token"})
		return
	}
	r := cache.LruCacheToken.Contains(tokenString)
	if r {
		c.AbortWithStatusJSON(401, gin.H{"error": "Token has been revoked"})
		return
	}
	cache.LruCacheToken.Add(tokenString, true) // Add the token to the cache to mark it as revoked
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, ErrInvalidToken
		}
		return PublicSigningKey, nil
	})

	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
		return
	}
	tokenClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token claims"})
		return
	}
	// 1. Safely extract and save all claims as strings
	iss, _ := tokenClaims["iss"].(string)
	aud, _ := tokenClaims["aud"].(string)
	sub, _ := tokenClaims["sub"].(string)
	jti, _ := tokenClaims["jti"].(string)
	id, _ := tokenClaims["id"].(string)

	if iss != JWTIssuer || aud != JWTAudience || aud == "" || sub == "" || jti == "" || id == "" {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid or missing token claims"})
		return
	}
	if cache.LruCacheJTI.Contains(jti) {
		c.AbortWithStatusJSON(401, gin.H{"error": "Token has been revoked"})
		return
	}
	// Set a 24-hour expiration for the JTI in Redis to prevent replay attacks
	success, err := redis.RedisClient.SetNX(ctx, jti, "true", tokenBlockDuration).Result()
	
	if err != nil {
		fmt.Println("Database connectivity error setting JTI registry:", err)
		c.AbortWithStatusJSON(500, gin.H{"error": "Internal server validation error"})
		return
	}

	// 3. Evaluate the result
	if !success {
		// If success is false, the JTI ALREADY existed in Redis. 
		// This means another instance or request already consumed it! Block it.
		cache.LruCacheJTI.Add(jti, true)           
		cache.LruCacheToken.Add(tokenString, true) 
		c.AbortWithStatusJSON(401, gin.H{"error": "Token has already been consumed"})
		return
	}

	// If success is true, Redis successfully saved the key, meaning it was a FRESH token.
	// Sync the consumption status to local memory too
	cache.LruCacheToken.Add(tokenString, true)
	cache.LruCacheJTI.Add(jti, true)

	c.Request.Header.Set("X-USER-ID", sub)
	c.Request.Header.Set("X-TWEET-ID", id)

	c.Next()

}
