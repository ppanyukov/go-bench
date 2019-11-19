package counter

import (
	"runtime"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

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
