package mongodb

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/husseinayyed/twivo-media/internal/database/mongodb/schema"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	databaseName    = "twivo"
	imageCollection = "images"
)

var (
	MONGODB_URL      = os.Getenv("MONGODB_URL")
	MONGODB_USER     = os.Getenv("MONGODB_USER")
	MONGODB_PASSWORD = os.Getenv("MONGODB_PASSWORD")
	Client           *mongo.Client
)

func InitMongo() {
	if MONGODB_URL == "" || MONGODB_USER == "" || MONGODB_PASSWORD == "" {
		log.Fatalf("One or more of (MONGODB_URL , MONGODB_USER , MONGODB_PASSWORD) environment variable must be set")
	}
	credential := options.Credential{
		Username: MONGODB_USER,
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
	// 1. Target the indexes interface for your collection
	indexView := client.Database(databaseName).Collection(imageCollection).Indexes()

	// 2. Drop the old non-unique index to clear the naming conflict
	// We ignore the error in case the index doesn't exist yet (e.g., fresh database)
	_ = indexView.DropOne(context.Background(), "check_sum_idx")

	// 3. Now your existing CreateMany code will run without conflicts
	_, err = indexView.CreateMany(
		context.Background(),
		[]mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "nano_id", Value: 1}},
				Options: options.Index().SetName("nano_id_idx"),
			},
			{
				Keys:    bson.D{{Key: "check_sum", Value: 1}},
				Options: options.Index().SetName("check_sum_idx").SetUnique(true), // Safe to run now!
			},
			{
				Keys:    bson.D{{Key: "phash", Value: 1}},
				Options: options.Index().SetName("phash_idx"),
			},
		},
	)
	if err != nil {
		log.Fatalf("Failed to create MongoDB indexes: %v", err)
	}

	// 🎉 If execution gets here, the connection is active and ready!
	fmt.Println("🚀 Successfully connected to MongoDB!")
	Client = client

}

func GetCheckSum(checksum string) (*schema.Image, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var img schema.Image
	err := Client.Database(databaseName).Collection(imageCollection).FindOne(ctx, bson.M{
		"check_sum": checksum,
	}).Decode(&img)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false
		}
		log.Printf("Database query failed: %v", err)
		return nil, false
	}

	return &img, true
}
func InsertImage(img *schema.Image) (*schema.Image, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Capture the result so we can get the generated ID
	res, err := Client.Database(databaseName).Collection(imageCollection).InsertOne(ctx, img)

	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			log.Printf("Insert failed: a document with this check_sum or nano_id already exists")
			return nil, false
		}

		log.Printf("Database insert failed: %v", err)
		return nil, false
	}

	// 2. If the original ID was empty, fill it with the database-generated ID
	if img.ID.IsZero() {
		if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
			img.ID = oid
		}
	}

	// 3. Returning 'img' returns the mutated pointer
	return img, true
}
