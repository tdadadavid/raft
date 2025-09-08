package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	url           = flag.String("url", "http://127.0.0.1:9001", "target node")
	conns         = flag.Int("conns", 50, "number of concurrent workers")
	ops           = flag.Int("ops", 1000, "total operations")
	writePct      = flag.Int("writepct", 50, "percentage of writes")
	collectLatency = flag.Bool("collect-latency", false, "collect latency percentiles")
)

var latencyHist *Histogram

func main() {
	flag.Parse()
	client := &http.Client{Timeout: 2 * time.Second}

	if *collectLatency {
		latencyHist = NewHistogram(1*time.Millisecond, 10*time.Second, 3)
	}

	var wg sync.WaitGroup
	var successes int64
	var failures int64

	jobs := make(chan int, *ops)
	for i := 0; i < *ops; i++ {
		jobs <- i
	}
	close(jobs)

	start := time.Now()

	for w := 0; w < *conns; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				opType := "read"
				if rand.Intn(100) < *writePct {
					opType = "write"
				}

				key := fmt.Sprintf("key-%d", j)
				value := fmt.Sprintf("val-%d", j)

				var req *http.Request
				var err error
				if opType == "write" {
					req, err = http.NewRequest("PUT", *url+"/kv/"+key, strings.NewReader(value))
				} else {
					req, err = http.NewRequest("GET", *url+"/kv/"+key, nil)
				}

				if err != nil {
					atomic.AddInt64(&failures, 1)
					continue
				}

				t0 := time.Now()
				resp, err := client.Do(req)
				dur := time.Since(t0)

				if *collectLatency {
					latencyHist.RecordValue(dur)
				}


				if err != nil {
					atomic.AddInt64(&failures, 1)
				} else {
					if resp.StatusCode >= 400 {
						atomic.AddInt64(&failures, 1)
					} else {
						atomic.AddInt64(&successes, 1)
					}

					if resp.Body != nil {
						stringRes, _ := io.ReadAll(resp.Body)
						log.Printf("response = %v", string(stringRes))
					}
				}

				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	rps := float64(*ops) / elapsed.Seconds()

	fmt.Printf("Total ops: %d, Success: %d, Failures: %d, Elapsed: %v, Throughput: %.2f ops/sec\n",
		*ops, successes, failures, elapsed, rps)

	if *collectLatency {
		fmt.Println("Latency percentiles:")
		fmt.Printf("  p50 = %v\n", latencyHist.ValueAtQuantile(50))
		fmt.Printf("  p95 = %v\n", latencyHist.ValueAtQuantile(95))
		fmt.Printf("  p99 = %v\n", latencyHist.ValueAtQuantile(99))
		fmt.Printf("  max = %v\n", latencyHist.Max())
	}
}