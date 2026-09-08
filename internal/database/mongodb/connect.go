package mongodb

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	MONGODB_URL = os.Getenv("MONGODB_URL")
)

func InitMongo() {
	if MONGODB_URL == "" {
		log.Fatalf("MONGODB_URL environment variable must be set")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(MONGODB_URL))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Ensure connection resource cleanup on exit
	defer func() {
		// Create a brand new 5-second context strictly for shutting down
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // Always call cancel to release resources

		if err := client.Disconnect(shutdownCtx); err != nil {
			log.Fatalf("Failed to disconnect safely: %v", err)
		}
		fmt.Println("Disconnected from MongoDB successfully.")
	}()

	// 🔍 Verify the connection is actually alive by sending a Ping
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	// Passing nil to Ping uses the primary node deployment info by default
	if err := client.Ping(pingCtx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB server: %v", err)
	}

	// 🎉 If execution gets here, the connection is active and ready!
	fmt.Println("🚀 Successfully connected to MongoDB!")

}
