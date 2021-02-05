package memstorage

import (
	"sync"
	"time"
)

// supposed to be slow
func nanoNow() uint64 {
	return uint64(time.Now().UnixNano())
}

// tick returns the total number of times the interval has occurred between start and current
func tick(start, current uint64, interval time.Duration) uint64 {
	return (current - start) / uint64(interval)
}

func availableTokens(lastTick, currentTick, max uint64, fillRate float64) uint64 {
	delta := currentTick - lastTick

	available := uint64(float64(delta) * fillRate)
	if available > max {
		return max
	}
	return available
}

// bucket is supposed to be internal usage only implementation of leaky bucket
type bucket struct {
	// startTime is the number of nanoseconds from epoch when the bucket was  created.
	startTime uint64
	// maxTokens is the maximum number of tokens available for this bucket at any time.
	// 	The actual number og available tokens should never exceed this value.
	maxTokens uint64
	// interval is the time at which occurs tick
	interval time.Duration
	// fillRate is the number of tokens to add per nanosecond.
	//	Its calculated based on maxTokens and interval.
	fillRate float64
	// availableTokens is the number of current remaining tokens.
	availableTokens uint64
	// lastTick is the last clock tick which is used to recalculate the number
	//of bucket tokens.
	lastTick uint64
	// lock is used to guard the mutable fields
	lock sync.Mutex
}

func newBucket(tokens uint64, interval time.Duration) *bucket {
	b := &bucket{

		startTime:       nanoNow(),
		maxTokens:       tokens,
		interval:        interval,
		fillRate:        float64(interval) / float64(tokens),
		availableTokens: tokens,
	}

	return b
}

// get returns info about the bucket
func (b *bucket) get() (tokens uint64, remaining uint64, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	tokens = b.maxTokens
	remaining = b.availableTokens
	return
}

func (b *bucket) take() (tokens uint64, remaining uint64, reset uint64, ok bool, err error) {
	now := nanoNow()
	currentTick := tick(b.startTime, now, b.interval)

	tokens = b.maxTokens
	reset = b.startTime + ((currentTick + 1) * uint64(b.interval))

	b.lock.Lock()
	if b.lastTick < currentTick {
		b.availableTokens = availableTokens(b.lastTick, currentTick, b.maxTokens, b.fillRate)
		b.lastTick = currentTick
	}

	if b.availableTokens > 0 {
		b.availableTokens--
		ok = true
		remaining = b.availableTokens
	}

	b.lock.Unlock()
	return
}
