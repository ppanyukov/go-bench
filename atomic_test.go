/*
These set of benchmarks are aimed to test the cost of accessing
shared pointers. There are three tests:

- loopLocalNoPtr: does not increment the shared pointer in the loop.
This is expected to have best performance.

- loopLocal: increments shared pointer in the loop,
but without using atomic package.

- loopAtomic: increments shared pointer in the loop using atomic package.

Variable 'routineCount' set the number of goroutines which run in parallel.
Use 'GOMAXPROCS=N' env var to set the these when running the tests.

It is expected that performance of functions accessing shared pointer will
degrade with increased number of goroutines due to CPU cache contention and
invalidation.

Results on my local MBP:

	Philips-MBP:go-bench philip$ GOMAXPROCS=1 go test -v -bench=. -benchtime 100x
	2019/11/18 11:28:13 Number of goroutines: 1
	goos: darwin
	goarch: amd64
	pkg: github.com/ppanyukov/go-bench
	BenchmarkLoopNoPtr  	     100	   6799168 ns/op
	BenchmarkLoop       	     100	   6689403 ns/op
	BenchmarkLoopAtomic 	     100	  10230860 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	2.407s

	Philips-MBP:go-bench philip$ GOMAXPROCS=2 go test -v -bench=. -benchtime 100x
	2019/11/18 11:28:19 Number of goroutines: 2
	goos: darwin
	goarch: amd64
	pkg: github.com/ppanyukov/go-bench
	BenchmarkLoopNoPtr-2    	     100	   7133741 ns/op
	BenchmarkLoop-2         	     100	  25968322 ns/op
	BenchmarkLoopAtomic-2   	     100	  34951884 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	6.889s

	Philips-MBP:go-bench philip$ GOMAXPROCS=4 go test -v -bench=. -benchtime 100x
	2019/11/18 11:28:32 Number of goroutines: 4
	goos: darwin
	goarch: amd64
	pkg: github.com/ppanyukov/go-bench
	BenchmarkLoopNoPtr-4    	     100	   7275700 ns/op
	BenchmarkLoop-4         	     100	  60480862 ns/op
	BenchmarkLoopAtomic-4   	     100	  69067172 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	13.853s

	Philips-MBP:go-bench philip$ GOMAXPROCS=8 go test -v -bench=. -benchtime 100x
	2019/11/18 11:28:57 Number of goroutines: 8
	goos: darwin
	goarch: amd64
	pkg: github.com/ppanyukov/go-bench
	BenchmarkLoopNoPtr-8    	     100	   8194569 ns/op
	BenchmarkLoop-8         	     100	 112466232 ns/op
	BenchmarkLoopAtomic-8   	     100	 123464570 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	24.651s

	Philips-MBP:go-bench philip$ GOMAXPROCS=16 go test -v -bench=. -benchtime 100x
	2019/11/18 11:29:34 Number of goroutines: 16
	goos: darwin
	goarch: amd64
	pkg: github.com/ppanyukov/go-bench
	BenchmarkLoopNoPtr-16     	     100	  15091715 ns/op
	BenchmarkLoop-16          	     100	 142919302 ns/op
	BenchmarkLoopAtomic-16    	     100	 184546588 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	34.631s

	Philips-MBP:go-bench philip$ GOMAXPROCS=24 go test -v -bench=. -benchtime 100x
	2019/11/18 11:30:17 Number of goroutines: 24
	goos: darwin
	goarch: amd64
	pkg: github.com/ppanyukov/go-bench
	BenchmarkLoopNoPtr-24     	     100	  24720759 ns/op
	BenchmarkLoop-24          	     100	 176428833 ns/op
	BenchmarkLoopAtomic-24    	     100	 236355064 ns/op
	PASS
	ok  	github.com/ppanyukov/go-bench	44.192s

*/
package go_bench

import (
	"log"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var routineCount = func() int {
	res := runtime.GOMAXPROCS(-1)
	log.Printf("Number of goroutines: %d\n", res)
	return res
}()

var loopCount = 1000000

// loopLocalNoPtr increments local counter and the total counter without using atomic primitives.
// It increments total counter by taking a local copy first, and adding to it in the end.
func loopLocalNoPtr(loopCount int, totalCounter *int64) {
	var random = rand.New(rand.NewSource(time.Now().Unix()))
	var localCounter = int64(0)
	var totalCounterCopy = int64(0)
	defer func() {
		*totalCounter += totalCounterCopy
	}()
	for i := 0; i <= loopCount; i++ {
		localCounter += random.Int63()
		totalCounterCopy += random.Int63()
		if localCounter > math.MaxInt64 || totalCounterCopy > math.MaxInt64 {
			return
		}
	}
}

// loopLocal increments local counter and the total counter without using atomic primitives.
// It increments total counter directly using pointer dereference.
func loopLocal(loopCount int, totalCounter *int64) {
	var random = rand.New(rand.NewSource(time.Now().Unix()))
	var localCounter = int64(0)
	for i := 0; i <= loopCount; i++ {
		localCounter += random.Int63()
		*totalCounter += random.Int63()
		if localCounter > math.MaxInt64 || *totalCounter > math.MaxInt64 {
			return
		}
	}
}

// loopLocal increments local counter. It uses atomic primitives to increment total counter.
func loopAtomic(loopCount int, totalCounter *int64) {
	var random = rand.New(rand.NewSource(time.Now().Unix()))
	var localCounter = int64(0)
	for i := 0; i <= loopCount; i++ {
		localCounter += random.Int63()
		totalCounterNew := atomic.AddInt64(totalCounter, random.Int63())
		if localCounter > math.MaxInt64 || totalCounterNew > math.MaxInt64 {
			return
		}
	}
}

func BenchmarkLoopNoPtr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wg := sync.WaitGroup{}
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
