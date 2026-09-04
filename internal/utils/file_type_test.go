package utils

import (
    "testing"
)

func TestDetectFileType(t *testing.T) {
    tests := []struct {
        name     string
        input    []byte
        expected string
        hasError bool
    }{
        {
            name:     "Valid JPEG",
            input:    []byte{0xFF, 0xD8, 0xFF, 0xE0},
            expected: ".jpeg",
            hasError: false,
        },
        {
            name:     "Valid PNG",
            input:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
            expected: ".png",
            hasError: false,
        },
        {
            name:     "Valid WebP",
            input:    []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50},
            expected: ".webp",
            hasError: false,
        },
        {
            name:     "Invalid type (text)",
            input:    []byte("Hello, World!"),
            expected: "null",
            hasError: false,
        },
        {
            name:     "Empty input",
            input:    []byte{},
            expected: "null",
            hasError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := DetectFileType(tt.input)
            if tt.hasError {
                if err == nil {
                    t.Errorf("expected error, got nil")
                }
                return
            }
            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("expected %q, got %q", tt.expected, result)
            }
        })
    }
}