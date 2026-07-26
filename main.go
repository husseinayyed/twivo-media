package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/matoous/go-nanoid/v2"
	"github.com/hashicorp/golang-lru/v2"
	"os"
)
func main() {
	// Create a new Gin router
	router := gin.Default()
	id, err := gonanoid.New()
	if err != nil {
		fmt.Println("Error generating nanoid:", err)
		os.Exit(1)
	}
	_ = id // Use the generated nanoid as needed
	// Create an LRU cache with a maximum size of 100 items
	cache, err := lru.New[string, string](100)
	if err != nil {
		fmt.Println("Error creating LRU cache:", err)
		os.Exit(1)
	}
	_ = cache // Use the LRU cache as needed
	router.GET("/ping", func(c *gin.Context) {

		c.JSON(200, gin.H{"ping": "pong"})
	})
	router.Run(":7080")
	}
