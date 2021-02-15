package redisstorage

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	redis "github.com/go-redis/redis/v8"
	limiter "github.com/mikuspikus/go-rate-limiter"
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
	client   *redis.Client
	script   *redis.Script

	stopped uint32
}

type Config struct {
	Tokens   uint64
	Interval time.Duration
}

func NewRedisStorage(cfg *Config, rc *redis.Client) (*RedisStorage, error) {
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

	script := redis.NewScript(script)

	rs := &RedisStorage{
		tokens:   tokens,
		interval: interval,
		client:   rc,
		script:   script,
	}
	return rs, nil
}
