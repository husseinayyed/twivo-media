package tasks

import (
	"github.com/bytedance/sonic"
	"fmt"
	"github.com/hibiken/asynq"
)

type UploadPayload struct {
    UserID   string `json:"user_id"`
    TweetID  string `json:"tweet_id"`
    FileUUID string `json:"file_uuid"`
    FileType string `json:"file_type"`
    Width    string `json:"width"`
    Height   string `json:"height"`
}

func (p *UploadPayload) Serialize() ([]byte, error) {
    return sonic.Marshal(p)
}

func (p *UploadPayload) Deserialize(data []byte) error {
    return sonic.Unmarshal(data, p)
}
func ScheduleUploadTask(ac *asynq.Client,data UploadPayload) error {
	payload, err := data.Serialize()
	if err != nil {
		return err
	}

	task := asynq.NewTask("upload_file", payload)

	info, err := ac.Enqueue(task)
	if err != nil {
		return fmt.Errorf("could not enqueue task: %v", err)
	}

	fmt.Printf("Scheduled upload task: id=%s queue=%s\n", info.ID, info.Queue)
	return nil
}