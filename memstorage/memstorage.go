package memstorage

import (
	"context"
	"fmt"
	ratelimiter "github.com/mikuspikus/go-rate-limiter"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrStoppedFlag = fmt.Errorf("setting stop flag failed")
)

type MemStorage struct {
	tokens   uint64
	interval time.Duration

	sweepInterval time.Duration
	sweepMinTTL   uint64

	buckets    map[string]*bucket
	bucketLock sync.RWMutex

	stopped  uint32
	stopChan chan struct{}
}

// Config is used to NewMemStorage. It setups the MemStorage
type Config struct {
	// Tokens is the number of tokens allowed per Interval. Default is 1.
	Tokens uint64
	// Interval is the time interval upon which rate limiting is enforced.
	// Default is 1 second.
	Interval time.Duration
	// SweepInterval is the rate at which clear stale entries. The lower,
	// the faster entries would be cleared, but it would reduce performance
	// because of locks. Default is 6 hours.
	SweepInterval time.Duration
	// SweepMinTTL is the minimum amount of time a session must be inactive
	// before it would be cleared from entry. Default if 12 hours.
	SweepMinTTL time.Duration
	// InitAlloc is the size to use for mem map. The buffer would be expanded
	// by compiler, but bigger values could trade memory for performance.
	// Default is 4096.
	InitAlloc int
}

func NewMemStorage(cfg *Config) (*MemStorage, error) {
	if cfg == nil {
		cfg = new(Config)
	}

	tokens := uint64(1)
	if cfg.Tokens > 0 {
		tokens = cfg.Tokens
	}

	interval := 1 * time.Second
	if cfg.Interval > 0 {
		interval = cfg.Interval
	}

	sweepInterval := 6 * time.Hour
	if cfg.SweepInterval > 0 {
		sweepInterval = cfg.SweepInterval
	}

	sweepMinTTL := 12 * time.Hour
	if cfg.SweepMinTTL > 0 {
		sweepMinTTL = cfg.SweepMinTTL
	}

	initAlloc := 4096
	if cfg.InitAlloc > 0 {
		initAlloc = cfg.InitAlloc
	}

	storage := &MemStorage{
		tokens:        tokens,
		interval:      interval,
		sweepInterval: sweepInterval,
		sweepMinTTL:   uint64(sweepMinTTL),
		buckets:       make(map[string]*bucket, initAlloc),
		stopChan:      make(chan struct{}),
	}

	go storage.purge()
	return storage, nil
}

// Close releases consumed memory and tickers
func (storage *MemStorage) Close(ctx context.Context) error {
	if !atomic.CompareAndSwapUint32(&storage.stopped, 0, 1) {
		return ErrStoppedFlag
	}

	close(storage.stopChan)

	storage.bucketLock.Lock()
	for key := range storage.buckets {
		delete(storage.buckets, key)
	}
	storage.bucketLock.Unlock()
	return nil
}

// purge is used to continually iterate over the buckets map and purge old values
// on the sweepInterval
func (storage *MemStorage) purge() {
	ticker := time.NewTicker(storage.sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-storage.stopChan:
			return
		case <-ticker.C:
		}

		storage.bucketLock.Lock()
		now := nanoNow()
		for key, bucket := range storage.buckets {
			bucket.lock.Lock()
			lastTime := bucket.startTime + (bucket.lastTick * uint64(bucket.interval))
			bucket.lock.Unlock()

			if now-lastTime > storage.sweepMinTTL {
				delete(storage.buckets, key)
			}
		}
		storage.bucketLock.Unlock()
	}
}

// Take attempts to remove a token from key. If take is successful, it returns true.
// Returns tokens limit, remaining tokens count, reset time, success flag and error.
func (storage *MemStorage) Take(ctx context.Context, key string) (uint64, uint64, uint64, bool, error) {
	if atomic.LoadUint32(&storage.stopped) == 1 {
		return 0, 0, 0, false, ratelimiter.ErrStopped
	}

	// read lock first for good scenario
	storage.bucketLock.RLock()
	if bucket, ok := storage.buckets[key]; ok {
		// lucky variant: bucket already exists
		storage.bucketLock.RUnlock()
		return bucket.take()
	}
	storage.bucketLock.RUnlock()

	// bucket was not found so the full lock should be taken
	storage.bucketLock.Lock()
	if bucket, ok := storage.buckets[key]; ok {
		// bucket was created by another goroutine during full lock
		storage.bucketLock.Unlock()
		return bucket.take()
	}

	// bucket does not exist (it was purged or key has been seen first time)
	bucket := newBucket(storage.tokens, storage.interval)
	storage.buckets[key] = bucket

	storage.bucketLock.Unlock()

	return bucket.take()

}

// Get retrieves the info by key if it exists
func (storage *MemStorage) Get(ctx context.Context, key string) (uint64, uint64, error) {
	if atomic.LoadUint32(&storage.stopped) == 1 {
		return 0, 0, ratelimiter.ErrStopped
	}

	storage.bucketLock.RLock()
	if bucket, ok := storage.buckets[key]; ok {
		storage.bucketLock.RUnlock()
		return bucket.get()
	}

	storage.bucketLock.RUnlock()
	return 0, 0, nil
}

// Set setups bucket by key and tokens and interval. Recreates bucket if needed.
func (storage *MemStorage) Set(ctx context.Context, key string, tokens uint64, interval time.Duration) error {
	storage.bucketLock.Lock()
	bucket := newBucket(tokens, interval)
	storage.buckets[key] = bucket
	storage.bucketLock.Unlock()
	return nil
}

// Burst add tokens to the available tokens of the bucket labeled by key.
// Creates a bucket with key if not found one.
func (storage *MemStorage) Burst(ctx context.Context, key string, tokens uint64) error {
	storage.bucketLock.Lock()

	if bucket, ok := storage.buckets[key]; ok {
		bucket.lock.Lock()
		storage.bucketLock.Unlock()

		bucket.availableTokens = bucket.availableTokens + tokens
		bucket.lock.Unlock()
		return nil
	}

	// record not found
	bucket := newBucket(storage.tokens+tokens, storage.interval)
	storage.buckets[key] = bucket
	storage.bucketLock.Unlock()
	return nil
}
