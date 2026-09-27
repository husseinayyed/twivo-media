package utils

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func encodeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()

	var encoded bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("encode %dx%d PNG: %v", width, height, err)
	}
	return encoded.Bytes()
}

func TestValidateImageDimensions(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		wantError string
	}{
		{name: "minimum dimensions accepted", width: MinImageWidth, height: MinImageHeight},
		{name: "maximum dimensions accepted", width: MaxImageWidth, height: MaxImageHeight},
		{name: "width below minimum", width: MinImageWidth - 1, height: MinImageHeight, wantError: "image dimensions too small"},
		{name: "height below minimum", width: MinImageWidth, height: MinImageHeight - 1, wantError: "image dimensions too small"},
		{name: "width above maximum", width: MaxImageWidth + 1, height: MinImageHeight, wantError: "image dimensions too large"},
		{name: "height above maximum", width: MinImageWidth, height: MaxImageHeight + 1, wantError: "image dimensions too large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ValidateImageDimensions(bytes.NewReader(encodeTestPNG(t, tt.width, tt.height)))
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if config.Width != tt.width || config.Height != tt.height {
				t.Fatalf("expected dimensions %dx%d, got %dx%d", tt.width, tt.height, config.Width, config.Height)
			}
		})
	}
}

func TestValidateImageDimensionsRejectsInvalidImage(t *testing.T) {
	_, err := ValidateImageDimensions(strings.NewReader("not an image"))
	if err == nil {
		t.Fatal("expected invalid image data to be rejected")
	}
	if !strings.Contains(err.Error(), "failed to decode image config") {
		t.Fatalf("unexpected error: %v", err)
	}
}
