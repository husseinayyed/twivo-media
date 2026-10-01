package middleware

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/husseinayyed/twivo-media/internal/cache"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"github.com/rs/zerolog/log"
)

var (
	JWTIssuer          = os.Getenv("JWT_ISS")
	JWTAudience        = os.Getenv("JWT_AUD")
	PUBLIC_KEY    = os.Getenv("PUBLIC_KEY")
	ErrInvalidToken    = errors.New("the provided token is invalid")
	tokenBlockDuration = 3 * time.Minute // 3 minutes in time.Duration nanoseconds

	// Global variable to hold your loaded public key across your application
	PublicSigningKey ed25519.PublicKey
)

type VerifyClaims struct {
	ImageID string `json:"id"`
	jwt.RegisteredClaims
}

func init() {
	if JWTIssuer == "" || JWTAudience == "" || PUBLIC_KEY == "" {
		log.Fatal().Msg("one or more required environment variables (JWT_ISS, JWT_AUD, PUBLIC_KEY) are empty")
	}
	// Read and parse your Ed25519 public key file
	pubKeyBytes, err := hex.DecodeString(PUBLIC_KEY)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to decode public key")
	}
	// 3. Store the type-asserted key into your global variable
	PublicSigningKey = ed25519.PublicKey(pubKeyBytes)
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
	claims := &VerifyClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, ErrInvalidToken
		}
		return PublicSigningKey, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(JWTIssuer),
		jwt.WithAudience(JWTAudience),
	)

	if err != nil || token == nil || !token.Valid {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
		return
	}
	if claims.Subject == "" || claims.ID == "" || claims.ImageID == "" {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid or missing token claims"})
		return
	}
	if cache.LruCacheJTI.Contains(claims.ID) {
		c.AbortWithStatusJSON(401, gin.H{"error": "Token has been revoked"})
		return
	}
	// Set a 24-hour expiration for the JTI in Redis to prevent replay attacks
	success, err := redis.RedisClient.SetNX(ctx, claims.ID, "true", tokenBlockDuration).Result()

	if err != nil {
		log.Error().Err(err).Msg("database connectivity error setting JTI registry")
		c.AbortWithStatusJSON(500, gin.H{"error": "Internal server validation error"})
		return
	}

	// 3. Evaluate the result
	if !success {
		// If success is false, the JTI ALREADY existed in Redis.
		// This means another instance or request already consumed it! Block it.
		cache.LruCacheJTI.Add(claims.ID, true)
		cache.LruCacheToken.Add(tokenString, true)
		c.AbortWithStatusJSON(401, gin.H{"error": "Token has already been consumed"})
		return
	}

	// If success is true, Redis successfully saved the key, meaning it was a FRESH token.
	// Sync the consumption status to local memory too
	cache.LruCacheToken.Add(tokenString, true)
	cache.LruCacheJTI.Add(claims.ID, true)

	c.Request.Header.Set("X-USER-ID", claims.Subject)
	c.Request.Header.Set("X-TWEET-ID", claims.ImageID)

	c.Next()

}
