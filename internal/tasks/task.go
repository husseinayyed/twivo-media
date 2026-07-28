package tasks

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/hibiken/asynq"
)

type UploadPayload struct {
	UserID   string
	TweetID  string
	FileUUID string
	FileType string
	Width string
	Height string
}

func (p *UploadPayload) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(p); err != nil {
		return nil, fmt.Errorf("failed to encode payload: %v", err)
	}
	return buf.Bytes(), nil
}

func (p *UploadPayload) Deserialize(data []byte) error {
	buf := bytes.NewBuffer(data)
	return gob.NewDecoder(buf).Decode(p)
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