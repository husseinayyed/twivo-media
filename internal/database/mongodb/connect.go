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
	MONGODB_USER = os.Getenv("MONGODB_USER")
	MONGODB_PASSWORD = os.Getenv("MONGODB_PASSWORD")
	MongoDbClient *mongo.Client
)

func InitMongo() {
	if MONGODB_URL == "" || MONGODB_USER == "" || MONGODB_PASSWORD==""{
		log.Fatalf("One or more of (MONGODB_URL , MONGODB_USER , MONGODB_PASSWORD) environment variable must be set")
	}
	credential := options.Credential{
		Username:MONGODB_USER,
		Password: MONGODB_PASSWORD,
	}
	client, err := mongo.Connect(options.Client().ApplyURI(MONGODB_URL).SetAuth(credential))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// 🔍 Verify the connection is actually alive by sending a Ping
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	// Passing nil to Ping uses the primary node deployment info by default
	if err := client.Ping(pingCtx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB server: %v", err)
	}
	// 🎉 If execution gets here, the connection is active and ready!
	fmt.Println("🚀 Successfully connected to MongoDB!")
	MongoDbClient = client

}