package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Image struct {
	ID primitive.ObjectID `bson:"_id,omitempty"`
	NanoId string `bson:"nano_id"`
	FileType string `bson:"file_type"`
	OwnerId string `bson:"owner_id"`
	TweetId string `bson:"tweet_id"`
	BelongsTo string `bson:"belongs_to"`
	Width int `bson:"width"`
	Height int `bson:"height"`
	CheckSum string `bson:"check_sum"`
	Phash string `bson:"phash"`
	CreatedAt time.Time  `bson:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at"`
}