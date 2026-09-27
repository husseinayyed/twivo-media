package reader

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestRecordingReaderRecordsAllReadBytes(t *testing.T) {
	reader := NewRecordingReader(strings.NewReader("record this"))

	read, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if string(read) != "record this" {
		t.Fatalf("unexpected read data: %q", read)
	}
	if got := reader.GetRecorded().String(); got != "record this" {
		t.Fatalf("unexpected recorded data: %q", got)
	}
	if got := reader.Len(); got != int64(len("record this")) {
		t.Fatalf("expected recorded length %d, got %d", len("record this"), got)
	}
}

func TestTimeoutReaderReturnsUnderlyingRead(t *testing.T) {
	reader := NewTimeoutReader(strings.NewReader("ready"), time.Second)
	buf := make([]byte, 5)

	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf[:n]) != "ready" {
		t.Fatalf("expected %q, got %q", "ready", buf[:n])
	}
}

type delayedReader struct {
	delay time.Duration
}

func (r delayedReader) Read([]byte) (int, error) {
	time.Sleep(r.delay)
	return 0, io.EOF
}

func TestTimeoutReaderReturnsTimeoutError(t *testing.T) {
	reader := NewTimeoutReader(delayedReader{delay: 30 * time.Millisecond}, time.Millisecond)

	_, err := reader.Read(make([]byte, 1))
	if err == nil || !strings.Contains(err.Error(), "chunk read stall timeout exceeded") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}
