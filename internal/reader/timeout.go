package reader
import (
 "fmt"
 "io"
 "time"
)
// TimeoutReader wraps an io.Reader and enforces a timeout for each read operation.
type TimeoutReader struct {
 r       io.Reader
 timeout time.Duration
}

func NewTimeoutReader(r io.Reader, timeout time.Duration) *TimeoutReader {
 return &TimeoutReader{r: r, timeout: timeout}
}

func (tr *TimeoutReader) Read(p []byte) (int, error) {
 type result struct {
  n   int
  err error
 }
 ch := make(chan result, 1)

 go func() {
  n, err := tr.r.Read(p)
  select {
  case ch <- result{n: n, err: err}:
  default:
  }
 }()
// Create a timer that will trigger after the specified timeout duration
 timer := time.NewTimer(tr.timeout)
 defer timer.Stop()

 select { 
 case <-timer.C:
  return 0, fmt.Errorf("chunk read stall timeout exceeded")
 case res := <-ch:
  return res.n, res.err
 }
}