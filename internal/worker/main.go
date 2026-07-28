package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/hibiken/asynq"
	"github.com/husseinayyed/twivo-media/internal/database/redis"
	"github.com/husseinayyed/twivo-media/internal/tasks"
	goredis "github.com/redis/go-redis/v9"
)
type Worker struct {
	Client *asynq.Client
}


func NewWorker() (*Worker, error) {
	redisClient, err := redis.ConnectRedis()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	client := asynq.NewClientFromRedisClient(redisClient)
	return &Worker{Client: client}, nil
}

func (w *Worker) Start() {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redis.RedisClient.Options().Addr},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"high":     3,
				"default":  1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc("upload_file", w.handleUploadFileTask)

	// 1. Intercept OS signals inside a background thread before running the server
	go func() {
		quit := make(chan os.Signal, 1)
		// Listen for standard kill/exit signals (Ctrl+C and termination events)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// This blocks the background goroutine until you trigger a signal
		<-quit
		log.Println("\n--> [Asynq Worker] Shutdown signal detected. Cleaning up...")

		// 2. Coordinated Internal Shutdown Sequence
		log.Println("[Asynq Worker] Halting queue polling loops...")
		srv.Stop() // Instantly stops workers from grabbing NEW tasks

		log.Println("[Asynq Worker] Waiting for running pipelines to finish...")
		srv.Shutdown() // Blocks until currently processing items hit 100% completion

		log.Println("[Asynq Worker] Safely closed down.")
		
		// If this worker runs as a completely standalone microservice binary,
		// you can uncomment the line below to exit the OS process immediately:
		os.Exit(0)
	}()

	// 3. Start server processing block (Blocks main execution string as before)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}

func (w *Worker) handleUploadFileTask(ctx context.Context, t *asynq.Task) error {
    var payload tasks.UploadPayload

    if err := payload.Deserialize(t.Payload()); err != nil {
        return fmt.Errorf("failed to deserialize payload: %v", err)
    }
    streamKey := "uploads:stream"
    
    // Prepare the event payload
    eventData := map[string]any{
        "user_id":   payload.UserID,
        "tweet_id":  payload.TweetID,
        "file_uuid": payload.FileUUID,
        "file_type": payload.FileType,
        "width":     payload.Width,
        "height":    payload.Height,
    }

    // Append data to the stream using XAdd
    err := redis.RedisClient.XAdd(ctx, &goredis.XAddArgs{
        Stream: streamKey,
        ID:     "*",
        Values: eventData,
    }).Err()
    if err != nil {
        return fmt.Errorf("failed to append upload event to stream: %v", err)
    }
    
    nanoKey := fmt.Sprintf("nano:%v", payload.FileUUID)

    // Store the hash data safely
    if err := redis.RedisClient.HSet(ctx, nanoKey, eventData).Err(); err != nil {
        return fmt.Errorf("failed to set hash data: %v", err)
    }
    
    // Set a 24-hour TTL on the hash key so Nginx can read it within that window
    if err := redis.RedisClient.Expire(ctx, nanoKey, 24*time.Hour).Err(); err != nil {
        return fmt.Errorf("failed to set hash TTL: %v", err)
    }

    return nil
}