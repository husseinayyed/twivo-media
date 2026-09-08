package utils

import (
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

func MakeNano() (string,error) {
	time := fmt.Sprintf("%x", time.Now().UnixMilli())
	
	nano,err := gonanoid.New(8)
	if err != nil {
		return "",err
	}
	return time + nano, nil
}