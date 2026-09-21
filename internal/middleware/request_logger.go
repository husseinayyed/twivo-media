package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)

		clientIP := c.ClientIP()
		startedAt := time.Now()
		log.Info().
			Str("request_id", requestID).
			Str("client_ip", clientIP).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Msg("request started")

		c.Next()

		log.Info().
			Str("request_id", requestID).
			Str("client_ip", clientIP).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("duration", time.Since(startedAt)).
			Msg("request completed")
	}
}
