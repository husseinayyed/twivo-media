package breaker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker/v2"
)

// ServiceName identifies a distinct external dependency that should be protected by a dedicated
// circuit breaker. This prevents one unhealthy upstream service from cascading into unrelated
// request paths.
type ServiceName string

const (
	ServiceMongoDB   ServiceName = "mongodb"
	ServiceRedis     ServiceName = "redis"
	ServiceSeaweedFS ServiceName = "seaweedfs"
	ServiceImgProxy  ServiceName = "imgproxy"
)

// Breakers holds the process-wide set of circuit breakers for the app's external dependencies.
// Keeping them in a registry gives us one shared state across the whole service lifecycle.
type Breakers struct {
	breakers map[ServiceName]*gobreaker.CircuitBreaker[any]
}

var (
	// MongoDB is the metadata store for image records. It is the slowest dependency in the chain,
	// so its window is broader and it opens more conservatively.
	MongoDB = newBreaker("mongodb", 60*time.Second, 30*time.Second)
	// Redis is the fast cache layer. It must fail fast under bursty load because the cache is meant
	// to absorb pressure and protect the slower downstream layers.
	Redis = newBreaker("redis", 30*time.Second, 15*time.Second)
	// SeaweedFS stores the uploaded media blobs. It is critical to retrieval but still a networked
	// dependency that should be protected from repeated retry storms.
	SeaweedFS = newBreaker("seaweedfs", 60*time.Second, 30*time.Second)
	// ImgProxy handles resize and format conversion. It is a networked transformation layer, so it
	// benefits from a tighter recovery cycle than the metadata store.
	ImgProxy = newBreaker("imgproxy", 30*time.Second, 15*time.Second)
)

// Init initializes the shared breaker set used by the app. Rebuilding these values on startup keeps
// the process state deterministic before the handlers begin issuing upstream calls.
func Init() {
	MongoDB = newBreaker("mongodb", 60*time.Second, 30*time.Second)
	Redis = newBreaker("redis", 30*time.Second, 15*time.Second)
	SeaweedFS = newBreaker("seaweedfs", 60*time.Second, 30*time.Second)
	ImgProxy = newBreaker("imgproxy", 30*time.Second, 15*time.Second)
}

// New creates and returns a registry of circuit breakers keyed by dependency name.
func New() *Breakers {
	return &Breakers{
		breakers: map[ServiceName]*gobreaker.CircuitBreaker[any]{
			ServiceMongoDB:   MongoDB,
			ServiceRedis:     Redis,
			ServiceSeaweedFS: SeaweedFS,
			ServiceImgProxy:  ImgProxy,
		},
	}
}

// Execute runs fn under the protection of the breaker for the given service.
// The underlying gobreaker is typed to any, so callers perform the final type assertion
// after the call returns.
func (b *Breakers) Execute(service ServiceName, fn func() (any, error)) (any, error) {
	cb, ok := b.breakers[service]
	if !ok {
		return nil, fmt.Errorf("breaker: no circuit registered for service %q", service)
	}

	result, err := cb.Execute(fn)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// State returns the current state of a service's breaker (for metrics/debugging).
func (b *Breakers) State(service ServiceName) gobreaker.State {
	cb, ok := b.breakers[service]
	if !ok {
		return gobreaker.StateClosed
	}
	return cb.State()
}

// newBreaker is an internal helper to construct a breaker with shared defaults.
func newBreaker(name string, interval, timeout time.Duration) *gobreaker.CircuitBreaker[any] {
	return gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:         name,
		MaxRequests:  3, // tests allowed in Half-Open
		Interval:     interval,
		BucketPeriod: 5 * time.Second,
		Timeout:      timeout,
		OnStateChange: func(name string, from, to gobreaker.State) {
			event := log.Info()
			if to == gobreaker.StateOpen {
				event = log.Warn()
			}
			event.
				Str("service", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("circuit state changed")
		},
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 4
		},
		IsExcluded: func(err error) bool {
			// Only exclude client cancellation.
			// DeadlineExceeded may indicate a real service failure.
			return errors.Is(err, context.Canceled)
		},
	})
}