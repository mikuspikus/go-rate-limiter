package go_rate_limiter

import (
	"context"
	"fmt"
	"time"
)

// ErrStopped should be returned when the storage is stopped.
var ErrStopped = fmt.Errorf("store is stopped")

type Storage interface {
	// Take takes the token from a memstorage by a given key if available and returning:
	// 	- limit size
	//	- number of remaining tokens for the interval
	// 	- server time when token will be available
	// 	- whether the take was successful
	// 	- any error that occurred during take (its supposed to be  backend errors)
	// If "ok" was false you should not serve request further
	Take(ctx context.Context, key string) (tokens, remaining, reset uint64, ok bool, err error)
	// Get returns current limit and remaining tokens for given key.
	// Does not change state of the memstorage
	Get(ctx context.Context, key string) (tokens, remaining uint64, err error)
	// Set setups the limit and interval for a provided key.
	// If key already exists then it would be overwrittten
	Set(ctx context.Context, key string, tokens uint64, interval time.Duration) error
	// Burst add more tokens to the the key's current bucket until next interval tick.
	// This may lead current bucket interval tick to exceed the maximum number of ticks until next interval.
	Burst(ctx context.Context, key string, tokens uint64) error
	// Close terminates the memstorage and cleans up any data structures or connections.
	// After Close(), Take() should always return zero
	Close(ctx context.Context) error
}
