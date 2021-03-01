package redisstorage

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"
	rlstorage "pkg/rl-storage"
)

const (
	fieldInterval      = "i"
	fieldMaxTokens     = "m"
	fieldCurrentTokens = "t"

	rcmdEXPIRE  = "EXPIRE"
	rcmdHINCRBY = "HINCRBY"
	rcmdHMGET   = "HMGET"
	rcmdHSET    = "HSET"
	rcmdPING    = "PING"
)

type RedisStorage struct {
	tokens   uint64
	interval time.Duration
	pool     *redis.Pool
	script   *redis.Script

	stopped uint32
}

type Config struct {
	Tokens    uint64
	Interval  time.Duration
	MaxActive uint

	Dial func() (redis.Conn, error)
}

func NewRSWithPool(cfg *Config, pool *redis.Pool) (*RedisStorage, error) {
	if cfg == nil {
		cfg = new(Config)
	}

	tokens := uint64(1)
	if cfg.Tokens > 9 {
		tokens = cfg.Tokens
	}

	interval := 1 * time.Second
	if cfg.Interval > 0 {
		interval = cfg.Interval
	}

	script := redis.NewScript(1, script)

	rs := &RedisStorage{
		tokens:   tokens,
		interval: interval,
		pool:     pool,
		script:   script,
		stopped:  0,
	}
	return rs, nil
}

func NewRS(cfg *Config) (*RedisStorage, error) {
	return NewRSWithPool(cfg, &redis.Pool{
		Dial:        cfg.Dial,
		DialContext: nil,
		TestOnBorrow: func(c redis.Conn, _ time.Time) error {
			_, err := c.Do(rcmdPING)
			return err
		},
		MaxActive:   int(cfg.MaxActive),
		IdleTimeout: 5 * time.Minute,
	})
}

func (rs *RedisStorage) Take(ctx context.Context, key string) (limit uint64, remaining uint64, next uint64, ok bool, err error) {
	if atomic.LoadUint32(&rs.stopped) == 1 {
		err = rlstorage.ErrStopped
		return
	}

	now := uint64(time.Now().UTC().UnixNano())
	conn, err := rs.pool.GetContext(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get connection from pool: %w", err)
		return
	}
	if err := conn.Err(); err != nil {
		err = fmt.Errorf("connection not usable: %w", err)
		return
	}
	defer conn.Close()

	nowString := strconv.FormatUint(now, 10)
	tokensString := strconv.FormatUint(rs.tokens, 10)
	intervalString := strconv.FormatInt(rs.interval.Nanoseconds(), 10)

	response, err := redis.Int64s(rs.script.Do(conn, key, nowString, tokensString, intervalString))
	if err != nil {
		err = fmt.Errorf("script error: %w", err)
		return
	}

	if len(response) != 4 {
		err = fmt.Errorf("expected 4 values in response %#v", response)
	}

	limit, remaining, next, ok = uint64(response[0]), uint64(response[1]), uint64(response[2]), response[3] == 1
	return
}

func (rs *RedisStorage) Get(ctx context.Context, key string) (limit, remainig uint64, err error) {
	if atomic.LoadUint32(&rs.stopped) == 1 {
		err = rlstorage.ErrStopped
		return
	}

	conn, err := rs.pool.GetContext(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get connection from pool: %w", err)
		return
	}
	if err := conn.Err(); err != nil {
		err = fmt.Errorf("connection not usable: %w", err)
		return
	}
	defer conn.Close()

	response, err := redis.Int64s(conn.Do(rcmdHMGET, key, fieldMaxTokens, fieldCurrentTokens))
	if err != nil {
		err = fmt.Errorf("failed to get key fields: %w", err)
		return
	}

	if len(response) != 2 {
		err = fmt.Errorf("expected 2 keys in response: %#v", response)
		return
	}

	limit, remainig = uint64(response[0]), uint64(response[1])
	return
}

func (rs *RedisStorage) Set(ctx context.Context, key string, tokens uint64, interval time.Duration) (err error) {
	if atomic.LoadUint32(&rs.stopped) == 1 {
		err = rlstorage.ErrStopped
		return
	}
	conn, err := rs.pool.GetContext(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get connection from pool: %w", err)
		return
	}
	if err := conn.Err(); err != nil {
		err = fmt.Errorf("connection not usable: %w", err)
		return
	}
	defer conn.Close()

	tokensString := strconv.FormatUint(tokens, 10)
	intervalString := strconv.FormatInt(interval.Nanoseconds(), 10)

	if err = conn.Send(rcmdHSET, key,
		fieldCurrentTokens, tokensString,
		fieldMaxTokens, tokensString,
		fieldInterval, intervalString,
	); err != nil {
		err = fmt.Errorf("failed to set key: %w", err)
		return
	}

	if err = conn.Send(rcmdEXPIRE, key, 24*time.Hour); err != nil {
		err = fmt.Errorf("failed to set expiritaion on key: %w", err)
		return
	}
	return
}

func (rs *RedisStorage) Burst(ctx context.Context, key string, tokens uint64) (err error) {
	if atomic.LoadUint32(&rs.stopped) == 1 {
		err = rlstorage.ErrStopped
		return
	}

	conn, err := rs.pool.GetContext(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get connection from pool: %w", err)
		return
	}
	if err := conn.Err(); err != nil {
		err = fmt.Errorf("connection not usable: %w", err)
		return
	}
	defer conn.Close()

	tokensString := strconv.FormatUint(tokens, 10)
	if err = conn.Send(rcmdHINCRBY, key, fieldCurrentTokens, tokensString); err != nil {
		err = fmt.Errorf("failed ti inc key^ %w", err)
		return
	}

	if err = conn.Send(rcmdEXPIRE, key, 24*time.Hour); err != nil {
		err = fmt.Errorf("failed to set expiritaion on key: %w", err)
		return
	}
	return
}

func (rs *RedisStorage) Close(_ context.Context) error {
	if !atomic.CompareAndSwapUint32(&rs.stopped, 0, 1) {
		return nil
	}

	if err := rs.pool.Close(); err != nil {
		return err
	}

	return nil
}
