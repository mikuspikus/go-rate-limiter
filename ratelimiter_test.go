package go_rate_limiter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"pkg/memstorage"
)

func TestNewLimiterMiddleware(t *testing.T) {
	t.Parallel()

	type case_ struct {
		name     string
		tokens   uint64
		interval time.Duration
	}

	cases := []case_{
		{
			name:     "milli",
			tokens:   10,
			interval: 500 * time.Millisecond,
		},
		{
			name:     "second",
			tokens:   5,
			interval: 1 * time.Second,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			storage, err := memstorage.NewMemStorage(&memstorage.Config{
				Tokens:   c.tokens,
				Interval: c.interval,
			})

			if err != nil {
				t.Fatal(err)
			}

			middleware, err := NewLimiterMiddleware(storage, IPKeyFunc())
			if err != nil {
				t.Fatal(err)
			}

			workPayload := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				fmt.Fprintf(w, "test test test...")
			})

			server := httptest.NewServer(middleware.Handle(workPayload))
			defer server.Close()

			client := server.Client()
			for i := uint64(0); i < c.tokens; i++ {
				response, err := client.Get(server.URL)
				if err != nil {
					t.Fatal(err)
				}
				limit, err := strconv.ParseUint(response.Header.Get(HeaderRateLimitLimit), 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := limit, c.tokens; got != want {
					t.Errorf("limit: expected %d, got %d", want, got)
				}

				reset, err := time.Parse(time.RFC822, response.Header.Get(HeaderRateLimitReset))
				if err != nil {
					t.Fatal(err)
				}
				if got, want := time.Until(reset), c.interval; got > want {
					t.Errorf("reset: expected %d, got %d", want, got)
				}

				remaining, err := strconv.ParseUint(response.Header.Get(HeaderRateLimitRemaining), 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := remaining, c.tokens-i-1; got != want {
					t.Errorf("remaining: expected %d, got %d", want, got)
				}
			}

			response, err := client.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := response.StatusCode, http.StatusTooManyRequests; got != want {
				t.Errorf("status code: expected %d, got %d", want, got)
			}

			limit, err := strconv.ParseUint(response.Header.Get(HeaderRateLimitLimit), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := limit, c.tokens; got != want {
				t.Errorf("limit: expected %d, got %d", want, got)
			}

			reset, err := time.Parse(time.RFC822, response.Header.Get(HeaderRateLimitReset))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := time.Until(reset), c.interval; got > want {
				t.Errorf("reset: expected %d, got %d", want, got)
			}

			remaining, err := strconv.ParseUint(response.Header.Get(HeaderRateLimitRemaining), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := remaining, uint64(0); got != want {
				t.Errorf("remaining: expected %d, got %d", want, got)
			}
		})
	}
}
