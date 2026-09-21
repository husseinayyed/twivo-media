package tasks

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

type UploadPayload struct {
	UserID    string `json:"user_id"`
	TweetID   string `json:"tweet_id"`
	FileUUID  string `json:"file_uuid"`
	CheckSum  string `json:"check_sum"`
	Phash     string `json:"phash"`
	BelongsTo string `json:"belongs_to"`
	FileType  string `json:"file_type"`
	Width     string `json:"width"`
	Height    string `json:"height"`
}

func (p *UploadPayload) Serialize() ([]byte, error) {
	return sonic.Marshal(p)
}

func (p *UploadPayload) Deserialize(data []byte) error {
	return sonic.Unmarshal(data, p)
}
func ScheduleUploadTask(ac *asynq.Client, data UploadPayload) error {
	payload, err := data.Serialize()
	if err != nil {
		return err
	}

	task := asynq.NewTask("upload_file", payload)

	info, err := ac.Enqueue(task)
	if err != nil {
		return fmt.Errorf("could not enqueue task: %v", err)
	}

	log.Info().
		Str("task_id", info.ID).
		Str("queue", info.Queue).
		Msg("scheduled upload task")
	return nil
}
