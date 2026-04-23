// Command mkprof generates CPU profiles with non-trivial call graphs.
//
//	go run ./cmd/mkprof -o cpu.prof
//	go run ./cmd/mkprof -diff -o /tmp/prof    # writes /tmp/prof.base and /tmp/prof.target
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

var (
	out  = flag.String("o", "cpu.prof", "output file (diff mode appends .base and .target)")
	diff = flag.Bool("diff", false, "generate a base/target pair for diff testing")
)

func main() {
	flag.Parse()

	if *diff {
		runDiff()
	} else {
		runSingle(*out, 1.0)
	}
}

func runDiff() {
	base := *out + ".base"
	target := *out + ".target"

	runSingle(base, 1.0)
	runSingle(target, 2.0)

	fmt.Fprintf(os.Stderr, "\ndiff test:\n  lazypprof %s %s\n", base, target)
}

func runSingle(path string, scale float64) {
	f, err := os.Create(path)
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
	go func() { defer wg.Done(); heavyMath(scale) }()
	go func() { defer wg.Done(); stringWork(scale) }()
	go func() { defer wg.Done(); sortWork(scale) }()
	go func() { defer wg.Done(); allocWork(scale) }()
	wg.Wait()

	pprof.StopCPUProfile()
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
}

func heavyMath(scale float64) {
	n := int(5_000_000 * scale)
	for i := 0; i < n; i++ {
		sinkF = math.Sqrt(float64(i)) * math.Log(float64(i+1))
		sinkF += trig(float64(i))
	}
}

func trig(x float64) float64 {
	return math.Sin(x) * math.Cos(x) * math.Tan(x/(math.Pi*1000)+1)
}

func stringWork(scale float64) {
	n := int(200_000 * scale)
	for i := 0; i < n; i++ {
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

func sortWork(scale float64) {
	n := int(100 * scale)
	for i := 0; i < n; i++ {
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

func allocWork(scale float64) {
	n := int(500_000 * scale)
	for i := 0; i < n; i++ {
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
