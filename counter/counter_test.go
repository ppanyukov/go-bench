// go test -cpu="1,2,4,8,16,24,48,96,192,256" -bench=.  ./counter
// See readme.md for details.
package counter

import (
	"runtime"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// CounterNoLock is a completely no-lock and no-atomic counter. Basically this
// is the fastest way to increment something within a goroutine. It's here to
// provide "rock bottom" figures -- you can't go any faster than this.
type CounterNoLock struct {
	localCounter uint64
}

func (b *CounterNoLock) Inc() {
	b.localCounter += 1
}

func (b *CounterNoLock) Add(val uint64) {
	b.localCounter += val
}


// CounterBuffer is buffered implementation of counter.
// It accumulates into local counter an flushes to shared
// prometheus counter once every flushInterval.
type CounterBuffer struct {
	inner         prometheus.Counter
	flushInterval uint64
	localCounter  uint64
}

func (b *CounterBuffer) Inc() {
	b.localCounter += 1
	if b.localCounter > b.flushInterval {
		b.inner.Add(float64(b.localCounter))
		b.localCounter = 0
	}
}

func (b *CounterBuffer) Add(val uint64) {
	b.localCounter += val
	if b.localCounter > b.flushInterval {
		b.inner.Add(float64(b.localCounter))
		b.localCounter = 0
	}
}

func (b *CounterBuffer) Flush() {
	if b.localCounter > 0 {
		b.inner.Add(float64(b.localCounter))
		b.localCounter = 0
	}
}


// The fastest kind of counter. Everything is local to goroutine.
func BenchmarkPromCounterLocalNoLock(b *testing.B) {
	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		counter := &CounterNoLock{
			localCounter: 0,
		}

		for pb.Next() {
			counter.Add(1)
			counter.Inc()
		}
	})
}

// Using vanilla Prometheus counter: one for each goroutine.
// The purpose of this test is to show the cost of atomic even
// if we are not changing shared state at all.
func BenchmarkPromCounterLocalVanilla(b *testing.B) {
	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		opts := prometheus.CounterOpts{
			Namespace:   "",
			Subsystem:   "",
			Name:        "",
			Help:        "",
			ConstLabels: nil,
		}

		counter := prometheus.NewCounter(opts)

		for pb.Next() {
			counter.Add(1)
			counter.Inc()
		}
	})
}

// Using buffered version of counter: one for each goroutine.
// The purpose of this test is to show the cost improvements
// when we don't use atomic for non-shared state.
func BenchmarkPromCounterLocalBuf(b *testing.B) {
	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		opts := prometheus.CounterOpts{
			Namespace:   "",
			Subsystem:   "",
			Name:        "",
			Help:        "",
			ConstLabels: nil,
		}

		counter := CounterBuffer{
			inner:         prometheus.NewCounter(opts),
			flushInterval: 1000,
			localCounter:  0,
		}

		for pb.Next() {
			counter.Add(1)
			counter.Inc()
		}
	})
}

// Using vanilla Prometheus counter shared across several goroutines.
// This is expected to be most expensive.
func BenchmarkPromCounterSharedVanilla(b *testing.B) {
	opts := prometheus.CounterOpts{
		Namespace:   "",
		Subsystem:   "",
		Name:        "",
		Help:        "",
		ConstLabels: nil,
	}

	counter := prometheus.NewCounter(opts)

	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
			counter.Inc()
		}
	})
}

// Using buffered counter across several goroutines.
// This is expected to show big perf improvements, close to
// the BenchmarkPromCounterLocalNoLock baseline.
func BenchmarkPromCounterSharedBuf(b *testing.B) {
	opts := prometheus.CounterOpts{
		Namespace:   "",
		Subsystem:   "",
		Name:        "",
		Help:        "",
		ConstLabels: nil,
	}

	shared := prometheus.NewCounter(opts)

	b.SetParallelism(runtime.GOMAXPROCS(-1))
	b.RunParallel(func(pb *testing.PB) {
		counter := CounterBuffer{
			inner:         shared,
			flushInterval: 1000,
			localCounter:  0,
		}

		for pb.Next() {
			counter.Add(1)
			counter.Inc()
		}
		counter.Flush()
	})
}
