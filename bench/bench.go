
package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var (
		target = flag.String("url", "http://app:8080/ping", "target URL")
		conns  = flag.Int("conns", 32, "number of concurrent workers")
		dur    = flag.Duration("dur", 10*time.Second, "duration (e.g. 10s, 1m)")
		qps    = flag.Int("qps", 0, "global QPS limit (0 = unlimited)")
	)
	flag.Parse()

	if _, err := url.ParseRequestURI(*target); err != nil {
		fmt.Fprintf(os.Stderr, "invalid url: %v\n", err)
		os.Exit(2)
	}

	client := &http.Client{ Timeout: 5 * time.Second }

	var wg sync.WaitGroup
	stop := make(chan struct{})
	latencies := make([]time.Duration, 0, 100000)
	var mu sync.Mutex
	var count uint64

	var tick <-chan time.Time
	if *qps > 0 { tick = time.NewTicker(time.Second / time.Duration(*qps)).C }

	work := func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if tick != nil { <-tick }
				start := time.Now()
				resp, err := client.Get(*target)
				if err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				lat := time.Since(start)
				mu.Lock(); latencies = append(latencies, lat); mu.Unlock()
				atomic.AddUint64(&count, 1)
			}
		}
	}

	for i := 0; i < *conns; i++ { wg.Add(1); go work() }
	time.Sleep(*dur); close(stop); wg.Wait()

	mu.Lock(); defer mu.Unlock()
	if len(latencies) == 0 { fmt.Println("no samples collected"); return }
	p50 := percentile(latencies, 50); p95 := percentile(latencies, 95); p99 := percentile(latencies, 99)
	rps := float64(count) / dur.Seconds()
	fmt.Printf("Samples: %d\nRPS: %.2f\np50: %s\np95: %s\np99: %s\n", len(latencies), rps, p50, p95, p99)
}

func percentile(latencies []time.Duration, p float64) time.Duration {
	copyLats := make([]time.Duration, len(latencies)); copy(copyLats, latencies)
	k := int(math.Ceil((p / 100.0) * float64(len(copyLats)))) - 1
	if k < 0 { k = 0 }
	selectK(copyLats, k)
	return copyLats[k]
}

func selectK(a []time.Duration, k int) {
	lo, hi := 0, len(a)-1
	for lo < hi {
		pivot := partition(a, lo, hi)
		if k <= pivot { hi = pivot } else { lo = pivot + 1 }
	}
}
func partition(a []time.Duration, lo, hi int) int {
	pivot := a[(lo+hi)/2]
	for lo <= hi {
		for a[lo] < pivot { lo++ }
		for a[hi] > pivot { hi-- }
		if lo <= hi { a[lo], a[hi] = a[hi], a[lo]; lo++; hi-- }
	}
	return hi
}
