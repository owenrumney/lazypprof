// Command mkprof generates a CPU profile with a non-trivial call graph.
// Run: go run ./internal/testutil/mkprof.go -o /tmp/cpu.prof
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
)

var out = flag.String("o", "cpu.prof", "output file")

func main() {
	flag.Parse()

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); heavyMath() }()
	go func() { defer wg.Done(); stringWork() }()
	go func() { defer wg.Done(); sortWork() }()
	go func() { defer wg.Done(); allocWork() }()
	wg.Wait()

	pprof.StopCPUProfile()
	fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
}

func heavyMath() {
	for i := 0; i < 5_000_000; i++ {
		sinkF = math.Sqrt(float64(i)) * math.Log(float64(i+1))
		sinkF += trig(float64(i))
	}
}

func trig(x float64) float64 {
	return math.Sin(x) * math.Cos(x) * math.Tan(x/(math.Pi*1000)+1)
}

func stringWork() {
	for i := 0; i < 200_000; i++ {
		s := fmt.Sprintf("item-%d-payload-%d", i, i*31)
		s = strings.ToUpper(s)
		s = strings.ReplaceAll(s, "PAYLOAD", "DATA")
		sinkS = reverseString(s)
	}
}

func reverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func sortWork() {
	for i := 0; i < 100; i++ {
		data := makeData(50_000)
		sort.Ints(data)
		sinkI = data[0]
	}
}

func makeData(n int) []int {
	d := make([]int, n)
	for i := range d {
		d[i] = n - i + (i * 7 % 13)
	}
	return d
}

func allocWork() {
	for i := 0; i < 500_000; i++ {
		b := make([]byte, 256)
		for j := range b {
			b[j] = byte(j ^ i)
		}
		sinkB = b
	}
}

// Sinks to prevent compiler from optimising away work.
var (
	sinkF float64
	sinkS string
	sinkI int
	sinkB []byte
)
