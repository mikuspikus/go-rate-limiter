package redisstorage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"os"
	"testing"
	"time"
)

func key(tb testing.TB) string {
	tb.Helper()

	var bytes [512]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		tb.Fatalf("failed to generate random string: %v", err)
	}

	key := fmt.Sprintf("%x", sha256.Sum256(bytes[:]))
	return key[:64]
}

func TestRedisStorage_All(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		t.Fatal("missing \"REDIS_HOST\"")
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		t.Fatal("missing \"REDIS_PORT\"")
	}
	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		t.Fatal("missing \"REDIS_PASSWORD\"")
	}

	storage, err := NewRS(&Config{
		Tokens:   15,
		Interval: 1 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", host+":"+port, redis.DialPassword(password))
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := storage.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})

	key := key(t)

	// first get -- should return defaults
	limit, remaining, err := storage.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := limit, uint64(0); got != want {
		t.Errorf("limit: got %d, want %d", got, want)
	}
	if got, want := remaining, uint64(0); got != want {
		t.Errorf("remaining: got %d, want %d", got, want)
	}

	// first take -- should use defaults
	limit, remaining, reset, ok, err := storage.Take(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected ok")
	}
	if got, want := limit, uint64(15); got != want {
		t.Errorf("limit: got %d, want: %d", got, want)
	}
	if got, want := remaining, uint64(14); got != want {
		t.Errorf("remaining: got %d, want %d", got, want)
	}
	if got, want := time.Until(time.Unix(0, int64(reset))), 1*time.Second; got > want {
		t.Errorf("reset: got %v, want to be less than %v", got, want)
	}
	// second get -- now should be real values
	limit, remaining, err = storage.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := limit, uint64(15); got != want {
		t.Errorf("limit: got %d, want %d", got, want)
	}
	if got, want := remaining, uint64(14); got != want {
		t.Errorf("remaining: got %d, want %d", got, want)
	}
	// set
	if err := storage.Set(ctx, key, 10, 3*time.Second); err != nil {
		t.Fatal(err)
	}

	// third get -- now should be used previously setted values
	limit, remaining, err = storage.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := limit, uint64(10); got != want {
		t.Errorf("limit: got %d, want %d", got, want)
	}
	if got, want := remaining, uint64(10); got != want {
		t.Errorf("remaining: got %d, want %d", got, want)
	}

	// second take -- now should be used previously setted values
	limit, remaining, reset, ok, err = storage.Take(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected ok")
	}
	if got, want := limit, uint64(10); got != want {
		t.Errorf("limit: got %d, want: %d", got, want)
	}
	if got, want := remaining, uint64(9); got != want {
		t.Errorf("remaining: got %d, want %d", got, want)
	}
	if got, want := time.Until(time.Unix(0, int64(reset))), 3*time.Second; got > want {
		t.Errorf("reset: got %v, want to be less than %v", got, want)
	}

	// burst
	if err := storage.Burst(ctx, key, 5); err != nil {
		t.Fatal(err)
	}
	// third take -- now should be changed values after burst
	limit, remaining, reset, ok, err = storage.Take(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected ok")
	}
	if got, want := limit, uint64(10); got != want {
		t.Errorf("limit: got %d, want: %d", got, want)
	}
	if got, want := remaining, uint64(13); got != want {
		t.Errorf("remaining: got %d, want %d", got, want)
	}
	if got, want := time.Until(time.Unix(0, int64(reset))), 3*time.Second; got > want {
		t.Errorf("reset: got %v, want to be less than %v", got, want)
	}
}
