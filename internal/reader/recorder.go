package reader
import (
	"bytes"
	"io"
)
type RecordingReader struct {
    r   io.Reader     // The underlying reader that we are wrapping
    buf *bytes.Buffer // Buffer to record the bytes read from the underlying reader
    len int64         // Total number of bytes read
}

func NewRecordingReader(r io.Reader) *RecordingReader {
    return &RecordingReader{
        r:   r,
        buf: &bytes.Buffer{},
        len: 0,
    }
}

func (rr *RecordingReader) Read(p []byte) (n int, err error) {
    n, err = rr.r.Read(p)
    if n > 0 {
        // Copy the read bytes into the buffer for recording
        rr.buf.Write(p[:n])
        rr.len += int64(n)
    }
    return n, err
}
// GetRecorded returns the recorded bytes as a bytes.Buffer.
func (rr *RecordingReader) GetRecorded() *bytes.Buffer {
    return rr.buf
}
func (rr *RecordingReader) Len() int64 {
    return rr.len
}