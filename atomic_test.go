/*
These set of benchmarks are aimed to test the cost of accessing
shared pointers. There are three tests:

- loopLocalNoPtr: does not increment the shared pointer in the loop.
This is expected to have best performance.

- loopLocal: increments shared pointer in the loop,
but without using atomic package.

- loopAtomic: increments shared pointer in the loop using atomic package.

It is expected that performance of functions accessing shared pointer will
degrade with increased number of goroutines due to CPU cache contention and
invalidation.

Results on my local MBP (8x core i9 CPU, hyper-threaded)

	Philips-MBP:go-bench philip$ go test -cpu="1,2,4,8,16,24" -bench=. -benchtime=10s
	goos: darwin
	goarch: amd64
	pkg: github.com/ppanyukov/go-bench
	BenchmarkLoopNoPtr        	    1617	   7278949 ns/op
	BenchmarkLoopNoPtr-2      	    1564	   7699908 ns/op
	BenchmarkLoopNoPtr-4      	    1484	   8940401 ns/op
	BenchmarkLoopNoPtr-8      	    1238	  10630713 ns/op
	BenchmarkLoopNoPtr-16     	     603	  20165222 ns/op
	BenchmarkLoopNoPtr-24     	     400	  29864253 ns/op

	BenchmarkLoop             	    1297	   8095006 ns/op
	BenchmarkLoop-2           	     462	  26689070 ns/op
	BenchmarkLoop-4           	     193	  61540155 ns/op
	BenchmarkLoop-8           	     100	 111132392 ns/op
	BenchmarkLoop-16          	      80	 144831500 ns/op
	BenchmarkLoop-24          	      61	 178581242 ns/op

	BenchmarkLoopAtomic       	    1098	   9719518 ns/op
	BenchmarkLoopAtomic-2     	     345	  37010750 ns/op
	BenchmarkLoopAtomic-4     	     175	  68657897 ns/op
	BenchmarkLoopAtomic-8     	      98	 121333444 ns/op
	BenchmarkLoopAtomic-16    	      70	 181610799 ns/op
	BenchmarkLoopAtomic-24    	      55	 219883166 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	253.595s

These show rapid degradation in performance when shared pointers get modified.

*/
package go_bench

import (
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func routineCount() int {
	res := runtime.GOMAXPROCS(-1)
	return res
}

var loopCount = 1000000

func newRand() *rand.Rand {
	return rand.New(rand.NewSource(443))
}

// loopLocalNoPtr increments local counter and the total counter without using atomic primitives.
// It increments total counter by taking a local copy first, and adding to it in the end.
func loopLocalNoPtr(loopCount int, totalCounter *int64) int64 {
	var random = newRand()
	var localCounter = int64(0)
	var totalCounterCopy = int64(0)
	defer func() {
		*totalCounter += totalCounterCopy
	}()
	for i := 0; i <= loopCount; i++ {
		localCounter += random.Int63()
		totalCounterCopy += random.Int63()
		if localCounter > math.MaxInt64 || totalCounterCopy > math.MaxInt64 {
			return localCounter
		}
	}

	return localCounter
}

// loopLocal increments local counter and the total counter without using atomic primitives.
// It increments total counter directly using pointer dereference.
func loopLocal(loopCount int, totalCounter *int64) int64 {
	var random = newRand()
	var localCounter = int64(0)
	for i := 0; i <= loopCount; i++ {
		localCounter += random.Int63()
		*totalCounter += random.Int63()
		if localCounter > math.MaxInt64 || *totalCounter > math.MaxInt64 {
			return localCounter
		}
	}

	return localCounter
}

// loopLocal increments local counter. It uses atomic primitives to increment total counter.
func loopAtomic(loopCount int, totalCounter *int64) int64 {
	var random = newRand()
	var localCounter = int64(0)
	for i := 0; i <= loopCount; i++ {
		localCounter += random.Int63()
		totalCounterNew := atomic.AddInt64(totalCounter, random.Int63())
		if localCounter > math.MaxInt64 || totalCounterNew > math.MaxInt64 {
			return localCounter
		}
	}

	return localCounter
}

func BenchmarkLoopNoPtr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wg := sync.WaitGroup{}
		routineCount := routineCount()
		for r := 0; r < routineCount; r++ {
			wg.Add(1)
			totalCounter := int64(0)
			go func() {
				defer wg.Done()
				loopLocalNoPtr(loopCount, &totalCounter)
			}()
		}
		wg.Wait()
	}
}

func BenchmarkLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wg := sync.WaitGroup{}
		routineCount := routineCount()
		for r := 0; r < routineCount; r++ {
			wg.Add(1)
			totalCounter := int64(0)
			go func() {
				defer wg.Done()
				loopLocal(loopCount, &totalCounter)
			}()
		}
		wg.Wait()
	}
}

func BenchmarkLoopAtomic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wg := sync.WaitGroup{}
		routineCount := routineCount()
		for r := 0; r < routineCount; r++ {
			wg.Add(1)
			totalCounter := int64(0)
			go func() {
				defer wg.Done()
				loopAtomic(loopCount, &totalCounter)
			}()
		}
		wg.Wait()
	}
}
