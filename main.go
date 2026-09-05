package main

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"

	pprof "github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/husseinayyed/twivo-media/internal/cache"
	"github.com/husseinayyed/twivo-media/internal/handler"
	"github.com/husseinayyed/twivo-media/internal/middleware"
	_ "golang.org/x/image/webp"
)



func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	pprof.Register(router) // Register pprof routes for profiling and debugging
	port := "8020"
	cache.InitCache() // Initialize the LRU cache for storing image checksums
	handler.InitWorker() // Initialize the worker for handling background tasks
	
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"ping": "pong"})
	})
	router.POST("/upload", middleware.VerifyToken, handler.UploadRoute)
	router.GET("/i/:id", handler.ImageRoute)

	router.Run(fmt.Sprintf(":%v", port))
}