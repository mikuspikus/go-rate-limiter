package memstorage

import (
	"testing"
	"time"
)

func TestBucket_tick(t *testing.T) {
	type case_ struct {
		name     string
		start    uint64
		current  uint64
		interval time.Duration
		expected uint64
	}

	t.Parallel()

	cases := []case_{
		{
			name:     "no difference",
			start:    0,
			current:  0,
			interval: time.Second,
			expected: 0,
		},
		{
			name:     "half",
			start:    0,
			current:  uint64(500 * time.Millisecond),
			interval: time.Second,
			expected: 0,
		},
		{
			name:     "almost",
			start:    0,
			current:  uint64(1*time.Second - time.Millisecond),
			interval: time.Second,
			expected: 0,
		},
		{
			name:     "exact",
			start:    0,
			current:  uint64(1 * time.Second),
			interval: 1 * time.Second,
			expected: 1,
		},
		{
			name:     "multiple",
			start:    0,
			current:  uint64(10*time.Second - 500*time.Millisecond),
			interval: 1 * time.Second,
			expected: 9,
		},
		{
			name:     "milli",
			start:    0,
			current:  uint64(10*time.Second - 500*time.Millisecond),
			interval: time.Millisecond,
			expected: 9500,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got, want := tick(c.start, c.current, c.interval), c.expected; got != want {
				t.Errorf("tick: expected %v, got %v", want, got)
			}
		})
	}
}

func TestBucket_availableTokens(t *testing.T) {
	type case_ struct {
		name     string
		last     uint64
		current  uint64
		max      uint64
		fillRate float64
		expected uint64
	}

	t.Parallel()
	cases := []case_{
		{
			name:     "zero",
			last:     0,
			current:  0,
			max:      1,
			fillRate: 1.0,
			expected: 0,
		},
		{
			name:     "one",
			last:     0,
			current:  1,
			max:      1,
			fillRate: 1.0,
			expected: 1,
		},
		{
			name:     "max",
			last:     0,
			current:  5,
			max:      2,
			fillRate: 1.0,
			expected: 2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got, want := availableTokens(c.last, c.current, c.max, c.fillRate), c.expected; got != want {
				t.Fatalf("availableTokens: expected %v got %v", want, got)
			}
		})
	}
}
