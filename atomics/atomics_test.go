// go test -cpu="1,2,4,8,16,24" -bench=. -benchtime=100x ./atomics

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
	BenchmarkLoopNoPtr        	   25954	    464182 ns/op
	BenchmarkLoopNoPtr-2      	   23108	    522946 ns/op
	BenchmarkLoopNoPtr-4      	   18465	    669737 ns/op
	BenchmarkLoopNoPtr-8      	   13980	    871386 ns/op
	BenchmarkLoopNoPtr-16     	    8292	   1516591 ns/op
	BenchmarkLoopNoPtr-24     	    5558	   2237993 ns/op

	BenchmarkLoop             	    7165	   1430208 ns/op
	BenchmarkLoop-2           	    6310	   1857615 ns/op
	BenchmarkLoop-4           	    5374	   2331765 ns/op
	BenchmarkLoop-8           	    3256	   3696851 ns/op
	BenchmarkLoop-16          	    1819	   6774960 ns/op
	BenchmarkLoop-24          	    1380	   9080867 ns/op

	BenchmarkLoopAtomic       	    1929	   5865269 ns/op
	BenchmarkLoopAtomic-2     	     348	  32108845 ns/op
	BenchmarkLoopAtomic-4     	     190	  63751175 ns/op
	BenchmarkLoopAtomic-8     	     100	 111437548 ns/op
	BenchmarkLoopAtomic-16    	      84	 166631953 ns/op
	BenchmarkLoopAtomic-24    	      55	 209213433 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	265.088s

These show rapid degradation in performance when shared pointers get modified.

*/
package atomics

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func routineCount() int {
	res := runtime.GOMAXPROCS(-1)
	return res
}

var array = func() []int64 {
	const loopCount = 1000000
	res := make([]int64, loopCount, loopCount)
	for i := int64(0); i < loopCount; i++ {
		res[i] = i
	}
	return res
}()

// loopLocalNoPtr increments local counter and the total counter without using atomic primitives.
// It increments total counter by taking a local copy first, and adding to it in the end.
func loopLocalNoPtr(array []int64, _ *int64) int64 {
	var localCounter = int64(0)
	for _, val := range array {
		localCounter += val
		if localCounter > math.MaxInt64 {
			return localCounter
		}
	}

	return localCounter
}

// loopLocal increments local counter and the total counter without using atomic primitives.
// It increments total counter directly using pointer dereference.
func loopLocal(array []int64, totalCounter *int64) int64 {
	var localCounter = int64(0)
	for _, val := range array {
		localCounter += val
		*totalCounter += val
		if localCounter > math.MaxInt64 || *totalCounter > math.MaxInt64 {
			return localCounter
		}
	}

	return localCounter
}

// loopAtomic increments local counter. It uses atomic primitives to increment total counter.
func loopAtomic(array []int64, totalCounter *int64) int64 {
	var localCounter = int64(0)
	for _, val := range array {
		localCounter += val
		totalCounterNew := atomic.AddInt64(totalCounter, val)
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
				loopLocalNoPtr(array, &totalCounter)
			}()
		}
		wg.Wait()
	}
}

func BenchmarkLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wg := sync.WaitGroup{}
		routineCount := routineCount()
		totalCounter := int64(0)
		for r := 0; r < routineCount; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				loopLocal(array, &totalCounter)
			}()
		}
		wg.Wait()
	}
}

func BenchmarkLoopAtomic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wg := sync.WaitGroup{}
		routineCount := routineCount()
		totalCounter := int64(0)
		for r := 0; r < routineCount; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				loopAtomic(array, &totalCounter)
			}()
		}
		wg.Wait()
	}
}
