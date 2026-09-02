package main

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	pprof "github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/husseinayyed/twivo-media/internal/middleware"
	"github.com/husseinayyed/twivo-media/internal/handler"
	_ "golang.org/x/image/webp"
)



func main() {
	router := gin.Default()
	pprof.Register(router) // Register pprof routes for profiling and debugging
	port := "8020"
	handler.InitWorker() // Initialize the worker for handling background tasks
	handler.InitImageCache() // Initialize the image cache for storing image metadata
	
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"ping": "pong"})
	})
	router.POST("/upload", middleware.VerifyToken, handler.UploadRoute)
	router.GET("/i/:id", handler.ImageRoute)

	router.Run(fmt.Sprintf(":%v", port))
}