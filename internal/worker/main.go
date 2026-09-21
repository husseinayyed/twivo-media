package worker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/husseinayyed/twivo-media/internal/cache"
	"github.com/husseinayyed/twivo-media/internal/database/mongodb"
	"github.com/husseinayyed/twivo-media/internal/database/mongodb/schema"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"github.com/husseinayyed/twivo-media/internal/tasks"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Worker struct {
	Client *asynq.Client
}

var workerLog = log.With().Str("service", "worker").Logger()

func NewWorker() (*Worker, error) {

	asynqOpt, err := asynq.ParseRedisURI(redis.REDIS_URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL for Asynq: %v", err)
	}

	// 3. Instantiate the authenticated Asynq Client
	client := asynq.NewClient(asynqOpt)

	return &Worker{Client: client}, nil
}

func (w *Worker) Start() {
	redisServerOpt, err := asynq.ParseRedisURI(redis.REDIS_URL)
	if err != nil {
		workerLog.Fatal().Err(err).Msg("failed to parse Redis URL for Asynq Server")
	}

	srv := asynq.NewServer(
		redisServerOpt, // Pass the fully authenticated choices object here
		asynq.Config{
			Concurrency: 20,
			Queues: map[string]int{
				"critical": 6,
				"high":     3,
				"default":  1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc("upload_file", w.handleUploadFileTask)
	workerLog.Info().Msg("worker started")

	// 1. Intercept OS signals inside a background thread before running the server
	go func() {
		quit := make(chan os.Signal, 1)
		// Listen for standard kill/exit signals (Ctrl+C and termination events)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// This blocks the background goroutine until you trigger a signal
		<-quit
		workerLog.Info().Msg("[Asynq Worker] Shutdown signal detected. Cleaning up...")

		// 2. Coordinated Internal Shutdown Sequence
		workerLog.Info().Msg("[Asynq Worker] Halting queue polling loops...")
		srv.Stop() // Instantly stops workers from grabbing NEW tasks

		workerLog.Info().Msg("[Asynq Worker] Waiting for running pipelines to finish...")
		srv.Shutdown() // Blocks until currently processing items hit 100% completion

		workerLog.Info().Msg("[Asynq Worker] Safely closed down.")

		// If this worker runs as a completely standalone microservice binary,
		// you can uncomment the line below to exit the OS process immediately:
		os.Exit(0)
	}()

	// 3. Start server processing block (Blocks main execution string as before)
	if err := srv.Run(mux); err != nil {
		workerLog.Fatal().Err(err).Msg("could not run server")
	}
}

func (w *Worker) handleUploadFileTask(ctx context.Context, t *asynq.Task) (taskErr error) {
	fileUUID := ""
	workerLog.Info().Str("task_type", t.Type()).Msg("worker task started")
	defer func() {
		if taskErr != nil {
			workerLog.Error().
				Err(taskErr).
				Str("task_type", t.Type()).
				Str("file_uuid", fileUUID).
				Msg("worker task failed")
			return
		}
		workerLog.Info().
			Str("task_type", t.Type()).
			Str("file_uuid", fileUUID).
			Msg("worker task finished")
	}()

	var payload tasks.UploadPayload
	if err := payload.Deserialize(t.Payload()); err != nil {
		return fmt.Errorf("failed to deserialize payload: %v", err)
	}
	fileUUID = payload.FileUUID
	return processUploadFileTask(ctx, payload)
}

func processUploadFileTask(ctx context.Context, payload tasks.UploadPayload) error {
	eventData := uploadEventData(payload)
	if err := storeUploadEvent(ctx, payload.FileUUID, eventData); err != nil {
		return err
	}

	width, err1 := strconv.ParseUint(payload.Width, 10, 16)
	height, err2 := strconv.ParseUint(payload.Height, 10, 16)
	if err1 != nil {
		return fmt.Errorf("invalid image width %q: %v", payload.Width, err1)
	}
	if err2 != nil {
		return fmt.Errorf("invalid image height %q: %v", payload.Height, err2)
	}

	cache.LruCacheNanoId.Add(payload.FileUUID, &cache.ImageResponse{
		Width:     uint16(width),
		Height:    uint16(height),
		FileUUID:  payload.FileUUID,
		BelongsTo: payload.BelongsTo,
		OwnerId:   payload.UserID,
		TweetId:   payload.TweetID,
		FileType:  payload.FileType,
	})

	return insertImageMetadata(payload, width, height)
}

func uploadEventData(payload tasks.UploadPayload) map[string]any {
	return map[string]any{
		"user_id":    payload.UserID,
		"tweet_id":   payload.TweetID,
		"file_uuid":  payload.FileUUID,
		"belongs_to": payload.BelongsTo,
		"file_type":  payload.FileType,
		"width":      payload.Width,
		"height":     payload.Height,
	}
}

func storeUploadEvent(ctx context.Context, fileUUID string, eventData map[string]any) error {
	redisKey := fmt.Sprintf("nano:%v", fileUUID)
	pipe := redis.RedisClient.TxPipeline()
	pipe.XAdd(ctx, &goredis.XAddArgs{
		Stream: "uploads:stream",
		ID:     "*",
		Values: eventData,
	})
	pipe.HSet(ctx, redisKey, eventData)
	pipe.Expire(ctx, redisKey, 24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute pipeline: %v", err)
	}
	return nil
}

func insertImageMetadata(payload tasks.UploadPayload, width, height uint64) error {
	img, inserted := mongodb.InsertImage(&schema.Image{
		ID:        primitive.NewObjectID(),
		NanoId:    payload.FileUUID,
		BelongsTo: payload.BelongsTo,
		TweetId:   payload.TweetID,
		OwnerId:   payload.UserID,
		FileType:  payload.FileType,
		Width:     int(width),
		Height:    int(height),
		CheckSum:  payload.CheckSum, // Placeholder: Provide actual checksum if available in payload
		Phash:     "nil",            // Placeholder: Provide actual phash if available in payload
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// 3. Optional: Recommended handling if the insert fails (e.g., due to duplicate index conflict)
	if !inserted {
		return fmt.Errorf("failed to insert image metadata into mongodb or image duplicate exists")
	}

	_ = img // Kept reference to prevent unused variable error if you plan to log it
	return nil
}
