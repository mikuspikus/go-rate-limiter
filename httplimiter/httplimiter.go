package httplimiter

import (
	"fmt"
	limiter "github.com/mikuspikus/go-rate-limiter"
	"net"
	"net/http"
	"strconv"
	"time"
)

var (
	ErrNoHeaderFound = fmt.Errorf("no specified header found")
	ErrNilStorage    = fmt.Errorf("storage is nil")
	ErrNilKeyFunc    = fmt.Errorf("keyfunc is nil")
)

const (
	// HTTP Headers from https://tools.ietf.org/id/draft-polli-ratelimit-headers-02.html
	HeaderRateLimitLimit     = "X-RateLimit-Limit"
	HeaderRateLimitRemaining = "X-RateLimit-Remaining"
	// in UTC time
	HeaderRateLimitReset = "X-RateLimit-Reset"
	// Custom header indicating when client can retry his request in UTC
	HeaderRetryAfter = "RetryAfter"
)

// KeyFunc is function template that will be used to get string key from request.
// The key is used to ID request for rate limiting.
// This function would be called on each request, so caching and performance magic advised.
// If KeyFunc returns error than Internal Server Error would be raised and no take from Storage.
type KeyFunc func(r *http.Request) (string, error)

// IPKeyFunc returns key based on incoming IP address.
func IPKeyFunc() KeyFunc {
	return func(r *http.Request) (string, error) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return "", err
		}

		return ip, nil
	}
}

// HeadersKeyFunc returns key based on list of specified headers.
// Because of r.Header.Get() they are case dependant.
func HeadersKeyFunc(headers ...string) KeyFunc {
	return func(r *http.Request) (string, error) {
		for _, header := range headers {
			// Get() makes header case dependant
			if value := r.Header.Get(header); value != "" {
				return value, nil
			}
		}
		return "", ErrNoHeaderFound
	}
}

// LimiterMiddleware is a mux that implements rate limiting and can wrap other middleware.
type LimiterMiddleware struct {
	storage limiter.Storage
	keyFunc KeyFunc
}

func NewLimiterMiddleware(s limiter.Storage, f KeyFunc) (*LimiterMiddleware, error) {
	if s == nil {
		return nil, ErrNilStorage
	}

	if f == nil {
		return nil, ErrNilKeyFunc
	}

	return &LimiterMiddleware{
		storage: s,
		keyFunc: f,
	}, nil
}

func (lm *LimiterMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		key, err := lm.keyFunc(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		limit, remaining, reset, ok, err := lm.storage.Take(ctx, key)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		resetFormatted := time.Unix(0, int64(reset)).UTC().Format(time.RFC822)

		w.Header().Set(HeaderRateLimitLimit, strconv.FormatUint(limit, 10))
		w.Header().Set(HeaderRateLimitRemaining, strconv.FormatUint(remaining, 10))
		w.Header().Set(HeaderRateLimitReset, resetFormatted)

		if !ok {
			w.Header().Set(HeaderRetryAfter, resetFormatted)
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
