**TL;DR**: Prometheus counters use atomic primitives, which is slow. If I didn't do something stupid, the buffered counter implementation gives potenial speedup of 200x in hot loops across multiple goroutines, and up to 6x speedup for "local" use. The buffered version approaches "you can't do it faster" version.


These tests benchmark Prometheus client library's Counter and compares with the "buffered" implementation provided here in this file.

Why this is needed? 

Prometheus counter uses `atomic` package every time clients call `Add` or `Inc`. Here is the code:

```golang
// https://github.com/prometheus/client_golang/blob/333f01cef0d61f9ef05ada3d94e00e69c8d5cdda/prometheus/counter.go#L87
func (c *counter) Add(v float64) {
    if v < 0 {
        panic(errors.New("counter cannot decrease in value"))
    }
    ival := uint64(v)
    if float64(ival) == v {
        atomic.AddUint64(&c.valInt, ival)
        return
    }

    for {
        oldBits := atomic.LoadUint64(&c.valBits)
        newBits := math.Float64bits(math.Float64frombits(oldBits) + v)
        if atomic.CompareAndSwapUint64(&c.valBits, oldBits, newBits) {
            return
        }
    }
}

func (c *counter) Inc() {
    atomic.AddUint64(&c.valInt, 1)
}
```

The library's docs say this about `NewCounter` function:

> ```
> // NewCounter creates a new Counter based on the provided CounterOpts.
> //
> // The returned implementation tracks the counter value in two separate
> // variables, a float64 and a uint64. The latter is used to track calls of the
> // Inc method and calls of the Add method with a value that can be represented
> // as a uint64. This allows atomic increments of the counter with optimal
> // performance. (It is common to have an Inc call in very hot execution paths.)
> // Both internal tracking values are added up in the Write method. This has to
> // be taken into account when it comes to precision and overflow behavior.
>```

Notice two sentences:

1. "This allows atomic increments of the counter with optimal performance".
2. "(It is common to have an Inc call in very hot execution paths.)"

This certainly feels sub-optimal to call existing counter implementation in very hot
execution paths due to use of `atomic`. If the counter is called from several
hot *goroutines*, then it would be even worse.

One approach to make things better is:

- We don't need to publish the counter values **immediately**, especially if we have a hot loop.
- Even without loops, most systems don't need to immediately ensure that the global counter is incremented in atomic and consistent way. There is no need for "real-time" kind of metrics.
- What we need is to buffer the increments and additions in a variables local to goroutine and then flush them periodically to the global shared counter using atomic.
- The idea is to reduce the number of very expensive calls to `atomic` at the price of more coarse-grained counter values.

These tests benchmarks such buffered implementation and compares results with
vanilla implementation.

Results on MacBook Pro with 8-core Intel i9 CPU:


```
# go test -cpu="1,2,4,8,16,24,48,96,192,256" -bench=.  ./counter
goos: darwin
goarch: amd64
pkg: github.com/ppanyukov/go-bench/counter

# can't do it faster version of counter
BenchmarkPromCounterLocalNoLock           	918099999	         1.29 ns/op
BenchmarkPromCounterLocalNoLock-2         	1000000000	         0.650 ns/op
BenchmarkPromCounterLocalNoLock-4         	1000000000	         0.344 ns/op
BenchmarkPromCounterLocalNoLock-8         	1000000000	         0.175 ns/op
BenchmarkPromCounterLocalNoLock-16        	1000000000	         0.121 ns/op
BenchmarkPromCounterLocalNoLock-24        	1000000000	         0.121 ns/op
BenchmarkPromCounterLocalNoLock-48        	1000000000	         0.124 ns/op
BenchmarkPromCounterLocalNoLock-96        	1000000000	         0.125 ns/op
BenchmarkPromCounterLocalNoLock-192       	1000000000	         0.137 ns/op
BenchmarkPromCounterLocalNoLock-256       	1000000000	         0.146 ns/op

# vanilla Prometheus counter: one for each goroutine
BenchmarkPromCounterLocalVanilla          	86532709	        13.5 ns/op
BenchmarkPromCounterLocalVanilla-2        	148958888	         7.86 ns/op
BenchmarkPromCounterLocalVanilla-4        	320491408	         3.86 ns/op
BenchmarkPromCounterLocalVanilla-8        	617431320	         1.96 ns/op
BenchmarkPromCounterLocalVanilla-16       	834107912	         1.45 ns/op
BenchmarkPromCounterLocalVanilla-24       	872732851	         1.52 ns/op
BenchmarkPromCounterLocalVanilla-48       	802193522	         1.53 ns/op
BenchmarkPromCounterLocalVanilla-96       	783802598	         1.54 ns/op
BenchmarkPromCounterLocalVanilla-192      	627372313	         1.65 ns/op
BenchmarkPromCounterLocalVanilla-256      	615174196	         1.81 ns/op

# buffered counter: one for each goroutine
BenchmarkPromCounterLocalBuf              	427818747	         2.61 ns/op
BenchmarkPromCounterLocalBuf-2            	951812734	         1.40 ns/op
BenchmarkPromCounterLocalBuf-4            	1000000000	         0.693 ns/op
BenchmarkPromCounterLocalBuf-8            	1000000000	         0.341 ns/op
BenchmarkPromCounterLocalBuf-16           	1000000000	         0.234 ns/op
BenchmarkPromCounterLocalBuf-24           	1000000000	         0.231 ns/op
BenchmarkPromCounterLocalBuf-48           	1000000000	         0.231 ns/op
BenchmarkPromCounterLocalBuf-96           	1000000000	         0.236 ns/op
BenchmarkPromCounterLocalBuf-192          	1000000000	         0.241 ns/op
BenchmarkPromCounterLocalBuf-256          	1000000000	         0.260 ns/op

# vanilla Prometheus counter: one counter shared across all goroutines
BenchmarkPromCounterSharedVanilla         	79626826	        13.4 ns/op
BenchmarkPromCounterSharedVanilla-2       	25313673	        43.3 ns/op
BenchmarkPromCounterSharedVanilla-4       	30068022	        39.4 ns/op
BenchmarkPromCounterSharedVanilla-8       	28719399	        40.8 ns/op
BenchmarkPromCounterSharedVanilla-16      	29089785	        40.2 ns/op
BenchmarkPromCounterSharedVanilla-24      	29212226	        40.3 ns/op
BenchmarkPromCounterSharedVanilla-48      	28206663	        40.4 ns/op
BenchmarkPromCounterSharedVanilla-96      	28199472	        40.4 ns/op
BenchmarkPromCounterSharedVanilla-192     	29109798	        42.5 ns/op
BenchmarkPromCounterSharedVanilla-256     	26449758	        42.6 ns/op

# buffered counter: 
#  - one buffered counter for each routine
#  - counts are flushed to shared Prometheus counter periodically.
BenchmarkPromCounterSharedBuf             	467419723	         2.54 ns/op
BenchmarkPromCounterSharedBuf-2           	920180032	         1.45 ns/op
BenchmarkPromCounterSharedBuf-4           	1000000000	         0.728 ns/op
BenchmarkPromCounterSharedBuf-8           	1000000000	         0.405 ns/op
BenchmarkPromCounterSharedBuf-16          	1000000000	         0.248 ns/op
BenchmarkPromCounterSharedBuf-24          	1000000000	         0.237 ns/op
BenchmarkPromCounterSharedBuf-48          	1000000000	         0.236 ns/op
BenchmarkPromCounterSharedBuf-96          	1000000000	         0.241 ns/op
BenchmarkPromCounterSharedBuf-192         	1000000000	         0.246 ns/op
BenchmarkPromCounterSharedBuf-256         	1000000000	         0.255 ns/op
PASS
ok  	github.com/ppanyukov/go-bench/counter	47.784s
```