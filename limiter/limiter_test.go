package limiter

import (
	"errors"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
)

var limitReached error = errors.New("limit reached")

type limiterNoLock struct {
	limit   int64
	current int64
}

func (l *limiterNoLock) Add(val int64) error {
	l.current += val
	if l.current > l.limit {
		return limitReached
	}

	return nil
}

type limiterAtomic struct {
	limit   int64
	current int64
}

func (l *limiterAtomic) Add(val int64) error {
	current := atomic.AddInt64(&l.current, val)
	if current > l.limit {
		return limitReached
	}

	return nil
}

type limiterAtomicBuffered struct {
	limit         int64
	flushInterval int64
	currentLocal  int64
	currentShared *int64
}

func (l *limiterAtomicBuffered) Add(val int64) error {
	// buffer into local counter until we buffered enough,
	// then flush to shared counter using atomic increment
	l.currentLocal += val
	if l.currentLocal > l.flushInterval {
		newCurrentShared := atomic.AddInt64(l.currentShared, l.currentLocal)
		l.currentLocal = 0
		if newCurrentShared > l.limit {
			return limitReached
		}
	}

	return nil
}

func BenchmarkLimiterLocalNoLock(b *testing.B) {
	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		limiter := &limiterNoLock{
			limit:   math.MaxInt64,
			current: 0,
		}

		for pb.Next() {
			if err := limiter.Add(1); err != nil {
				continue
			}
		}
	})
}

func BenchmarkLimiterSharedNoLockRace(b *testing.B) {
	limiter := &limiterNoLock{
		limit:   math.MaxInt64,
		current: 0,
	}

	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = limiter.Add(1)
		}
	})
}

func BenchmarkLimiterSharedAtomic(b *testing.B) {
	limiter := &limiterAtomic{
		limit:   math.MaxInt64,
		current: 0,
	}

	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := limiter.Add(1); err != nil {
				continue
			}
		}
	})
}

func BenchmarkLimiterSharedAtomicBuf(b *testing.B) {
	var shared = int64(0)

	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		limiter := &limiterAtomicBuffered{
			limit:         math.MaxInt64,
			flushInterval: 1000,
			currentLocal:  0,
			currentShared: &shared,
		}

		for pb.Next() {
			if err := limiter.Add(1); err != nil {
				continue
			}
		}
	})
}


