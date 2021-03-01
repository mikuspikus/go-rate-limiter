package memstorage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sort"
	"testing"
	"time"
)

func testKey(tb testing.TB) string {
	tb.Helper()

	var bytes [256]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		tb.Fatalf("string generating failed: %v", err)
	}

	shaed := fmt.Sprintf("%x", sha256.Sum256(bytes[:]))
	return shaed[:32]
}

func TestMemStorage_Take(t *testing.T) {
	type result struct {
		limit, remaining uint64
		reset            time.Duration
		ok               bool
		err              error
	}
	type case_ struct {
		name     string
		tokens   uint64
		interval time.Duration
	}
	t.Parallel()

	ctx := context.Background()

	cases := []case_{
		{
			name:     "milliseconds",
			tokens:   10,
			interval: 250 * time.Millisecond,
		},
		{
			name:     "seconds",
			tokens:   10,
			interval: 1 * time.Second,
		},
	}

	for _, c := range cases {
		case_ := c
		t.Run(case_.name, func(t *testing.T) {
			t.Parallel()
			key := testKey(t)
			storage, err := NewMemStorage(&Config{
				Tokens:        case_.tokens,
				Interval:      case_.interval,
				SweepInterval: 1 * time.Hour,
				SweepMinTTL:   1 * time.Hour,
			})

			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() {
				if err := storage.Close(ctx); err != nil {
					t.Fatal(err)
				}
			})

			take := make(chan *result, 2*c.tokens)
			for i := uint64(1); i <= 2*c.tokens; i++ {
				go func() {
					limit, remaining, reset, ok, err := storage.Take(ctx, key)
					take <- &result{limit: limit, remaining: remaining, reset: time.Duration(nanoNow() - reset), ok: ok, err: err}
				}()
			}

			var result_s []*result
			for i := uint64(1); i <= 2*c.tokens; i++ {
				select {
				case result := <-take:
					result_s = append(result_s, result)
				case <-time.After(10 * time.Second):
					t.Fatalf("timeout")
				}
			}

			// sort by errors and tokens left in storage
			sort.Slice(result_s, func(i, j int) bool {
				if result_s[i].remaining == result_s[j].remaining {
					return !result_s[j].ok
				}
				return result_s[i].remaining > result_s[j].remaining
			})

			for i, result := range result_s {
				if result.err != nil {
					t.Fatal(result.err)
				}

				if got, want := result.limit, c.tokens; got != want {
					t.Errorf("limit: expected %d got %d", want, got)
				}
				if got, want := result.reset, c.interval; got > want {
					t.Errorf("reset: expected %d, got %d", want, got)
				}

				if uint64(i) < c.tokens {
					if got, want := result.remaining, c.tokens-uint64(i)-1; got != want {
						t.Errorf("remaining: expected %d, got %d", want, got)
					}
					if got, want := result.ok, true; got != want {
						t.Errorf("ok: expected %t, got %t", want, got)
					}
				} else {
					if got, want := result.remaining, uint64(0); got != want {
						t.Errorf("remaining: expected %d, got %d", want, got)
					}
					if got, want := result.ok, false; got != want {
						t.Errorf("ok: expected %t, got %t", want, got)
					}
				}
			}
			time.Sleep(c.interval)
			_, _, _, ok, err := storage.Take(ctx, key)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Errorf("failed to take once more")
			}
		})
	}
}

func TestMemStorage_Basic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	storage, err := NewMemStorage(&Config{
		Tokens:        10,
		Interval:      1 * time.Second,
		SweepInterval: 1 * time.Hour,
		SweepMinTTL:   1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := storage.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	key := testKey(t)

	// Take
	limit, remaining, reset, ok, err := storage.Take(ctx, key)
	if err != nil {
		t.Error(err)
	}
	if !ok {
		t.Errorf("expected ok")
	}
	if got, want := limit, uint64(10); got != want {
		t.Errorf("limit: expected %d, got %d", want, got)
	}
	if got, want := remaining, uint64(9); got != want {
		t.Errorf("remaining: expected %d, got %d", want, got)
	}
	if got, want := time.Until(time.Unix(0, int64(reset))), 1*time.Second; got > want {
		t.Errorf("reset: expected %v, got %v", want, got)
	}
	// Get
	limit, remaining, err = storage.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := limit, uint64(10); got != want {
		t.Errorf("limit: expected %d, got %d", want, got)
	}
	if got, want := remaining, uint64(9); got != want {
		t.Errorf("remaining: expected %d, got %d", want, got)
	}
	// Set
	if err := storage.Set(ctx, key, 15, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	// Get again
	limit, remaining, err = storage.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := limit, uint64(15); got != want {
		t.Errorf("limit: expected %d, got %d", want, got)
	}
	if got, want := remaining, uint64(15); got != want {
		t.Errorf("remaining: expected %d, got %d", want, got)
	}
	// Burst
	if err := storage.Burst(ctx, key, 5); err != nil {
		t.Fatal(err)
	}
	// Take after burst
	limit, remaining, reset, ok, err = storage.Take(ctx, key)
	if err != nil {
		t.Error(err)
	}
	if !ok {
		t.Errorf("expected ok")
	}
	if got, want := limit, uint64(15); got != want {
		t.Errorf("limit: expected %d, got %d", want, got)
	}
	if got, want := remaining, uint64(19); got != want {
		t.Errorf("remaining: expected %d, got %d", want, got)
	}
	if got, want := time.Until(time.Unix(0, int64(reset))), 2*time.Second; got > want {
		t.Errorf("reset: expected %v, got %v", want, got)
	}
}
