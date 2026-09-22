package mongodb

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/husseinayyed/twivo-media/internal/breaker"
	"github.com/husseinayyed/twivo-media/internal/database/mongodb/schema"
	"github.com/rs/zerolog/log"
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
		log.Fatal().Msg("one or more of MONGODB_URL, MONGODB_USER, MONGODB_PASSWORD environment variables must be set")
	}

	var client *mongo.Client
	var err error

	_, err = breaker.MongoDB.Execute(func() (any, error) {
		credential := options.Credential{
			Username: MONGODB_USER,
			Password: MONGODB_PASSWORD,
		}

		client, err = mongo.Connect(options.Client().ApplyURI(MONGODB_URL).SetAuth(credential))
		if err != nil {
			return nil, err
		}

		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()

		if err = client.Ping(pingCtx, nil); err != nil {
			return nil, err
		}

		return client, nil
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to MongoDB")
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
		log.Fatal().Err(err).Msg("failed to create MongoDB indexes")
	}

	// 🎉 If execution gets here, the connection is active and ready!
	log.Info().Msg("successfully connected to MongoDB")
	Client = client

}

func GetCheckSum(checksum string) (*schema.Image, bool) {
	

	result, err := breaker.MongoDB.Execute(func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var img schema.Image
		err := Client.Database(databaseName).Collection(imageCollection).FindOne(ctx, bson.M{
			"check_sum": checksum,
		}).Decode(&img)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, nil
			}
			return nil, err
		}

		return &img, nil
	})
	if err != nil {
		log.Error().Err(err).Msg("database query failed")
		return nil, false
	}
	if result == nil {
		return nil, false
	}

	img, ok := result.(*schema.Image)
	if !ok {
		return nil, false
	}
	return img, true
}

func GetImage(nano string) (*schema.Image, error) {
	if breaker.MongoDB == nil {
		log.Warn().Msg("mongodb circuit breaker not initialized")
	}

	result, err := breaker.MongoDB.Execute(func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var img schema.Image
		err := Client.Database(databaseName).
			Collection(imageCollection).
			FindOne(ctx, bson.M{"nano_id": nano}).
			Decode(&img)

		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, mongo.ErrNoDocuments
			}
			return nil, err
		}
		return &img, nil
	})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Info().Msg("no image found with that nano ID")
			return nil, mongo.ErrNoDocuments
		}
		return nil, err
	}
	if result == nil {
		return nil, mongo.ErrNoDocuments
	}

	img, ok := result.(*schema.Image)
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	return img, nil
}

func InsertImage(img *schema.Image) (*schema.Image, bool) {
	if breaker.MongoDB == nil {
		log.Warn().Msg("mongodb circuit breaker not initialized")
	}

	result, err := breaker.MongoDB.Execute(func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		res, err := Client.Database(databaseName).Collection(imageCollection).InsertOne(ctx, img)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return nil, nil
			}
			return nil, err
		}

		if img.ID.IsZero() {
			if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
				img.ID = oid
			}
		}

		return img, nil
	})
	if err != nil {
		log.Error().Err(err).Msg("database insert failed")
		return nil, false
	}
	if result == nil {
		log.Warn().Msg("image already exists with this check_sum or nano_id")
		return nil, false
	}

	inserted, ok := result.(*schema.Image)
	if !ok {
		return nil, false
	}
	return inserted, true
}
