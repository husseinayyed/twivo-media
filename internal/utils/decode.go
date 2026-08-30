package utils
import (
 "fmt"
 "image"
 "io"
)
const (

 MinImageWidth  = 100
 MinImageHeight = 100
 MaxImageWidth  = 2048
 MaxImageHeight = 2048
)
func ValidateImageDimensions(recReader io.Reader) (image.Config, error) {
  config, _, err := image.DecodeConfig(recReader)
  if err != nil {
	return config, fmt.Errorf("failed to decode image config: %w", err)
  }
  if config.Width < MinImageWidth || config.Height < MinImageHeight {
	return config, fmt.Errorf("image dimensions too small: %dx%d (minimum: %dx%d)", config.Width, config.Height, MinImageWidth, MinImageHeight)
  }
  if config.Width > MaxImageWidth || config.Height > MaxImageHeight {
	return config, fmt.Errorf("image dimensions too large: %dx%d (maximum: %dx%d)", config.Width, config.Height, MaxImageWidth, MaxImageHeight)
  }
  return config, nil
}
