// Command serve runs a test HTTP server with /debug/pprof endpoints and
// synthetic workload. Use it to test lazypprof in live mode.
//
//	go run ./internal/testutil/serve.go [-addr :6060]
//
// Then in another terminal:
//
//	go run ./cmd/lazypprof http://localhost:6060
package main

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	_ "net/http/pprof"
	"sort"
	"strings"
	"sync"
	"time"
)

var addr = flag.String("addr", ":6060", "listen address")

func main() {
	flag.Parse()
	fmt.Printf("test service listening on %s\n", *addr)
	fmt.Println("endpoints:")
	fmt.Println("  /debug/pprof/profile?seconds=5  (CPU)")
	fmt.Println("  /debug/pprof/heap               (heap)")
	fmt.Println("  /debug/pprof/allocs             (allocs)")
	fmt.Println("  /debug/pprof/goroutine?debug=2  (goroutines)")

	// Spin up background work to generate interesting profiles.
	go mathLoop()
	go stringLoop()
	go sortLoop()
	go allocLoop()

	if err := http.ListenAndServe(*addr, nil); err != nil {
		fmt.Printf("listen error: %v\n", err)
	}
}

func mathLoop() {
	for {
		for i := 0; i < 2_000_000; i++ {
			sinkF = math.Sqrt(float64(i)) * math.Log(float64(i+1))
			sinkF += math.Sin(float64(i)) * math.Cos(float64(i))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func stringLoop() {
	for {
		for i := 0; i < 50_000; i++ {
			s := fmt.Sprintf("item-%d-payload-%d", i, i*31)
			s = strings.ToUpper(s)
			s = strings.ReplaceAll(s, "PAYLOAD", "DATA")
			sinkS = reverse(s)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func sortLoop() {
	for {
		data := make([]int, 100_000)
		for i := range data {
			data[i] = len(data) - i + (i * 7 % 13)
		}
		sort.Ints(data)
		sinkI = data[0]
		time.Sleep(150 * time.Millisecond)
	}
}

func allocLoop() {
	var mu sync.Mutex
	var heap [][]byte
	for {
		for i := 0; i < 10_000; i++ {
			b := make([]byte, 256+(i%1024))
			for j := range b {
				b[j] = byte(j ^ i)
			}
			mu.Lock()
			heap = append(heap, b)
			if len(heap) > 50_000 {
				heap = heap[len(heap)/2:]
			}
			mu.Unlock()
		}
		time.Sleep(100 * time.Millisecond)
	}
}

var (
	sinkF float64
	sinkS string
	sinkI int
)
