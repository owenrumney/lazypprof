package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/owenrumney/lazypprof/internal/source"
	"github.com/owenrumney/lazypprof/internal/tui"
)

func main() {
	interval := flag.Duration("interval", 0, "poll interval for live mode (e.g. 5s, 10s); 0 = auto")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lazypprof [-interval N] <profile-file | http://host:port [cpu|heap|allocs|goroutine]>\n\n")
		fmt.Fprintf(os.Stderr, "If no argument is given, lazypprof probes localhost:6060 and connects automatically.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  lazypprof cpu.prof\n")
		fmt.Fprintf(os.Stderr, "  lazypprof http://localhost:6060\n")
		fmt.Fprintf(os.Stderr, "  lazypprof http://localhost:6060 heap\n")
		fmt.Fprintf(os.Stderr, "  lazypprof -interval 3s http://localhost:6060 allocs\n")
		fmt.Fprintf(os.Stderr, "  lazypprof http://localhost:6060 goroutine\n\n")
		fmt.Fprintf(os.Stderr, "Profile types (live mode): cpu (default), heap, allocs, goroutine\n")
		fmt.Fprintf(os.Stderr, "Press [m] in live mode to switch between profile types.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		if url := probeLocalhost(); url != "" {
			fmt.Fprintf(os.Stderr, "no target given; detected service at %s\n", url)
			runLive(url, source.ProfileCPU, *interval)
			return
		}
		flag.Usage()
		os.Exit(1)
	}

	arg := flag.Arg(0)

	if source.Detect(arg) {
		pt := parseProfileType(flag.Arg(1))
		runLive(arg, pt, *interval)
	} else {
		runFile(arg)
	}
}

// probeLocalhost checks whether a service is accepting TCP connections on
// localhost:6060. Returns the base URL to use, or empty string if nothing
// is listening.
func probeLocalhost() string {
	conn, err := net.DialTimeout("tcp", "localhost:6060", time.Second)
	if err != nil {
		return ""
	}
	conn.Close()
	return "http://localhost:6060"
}

func parseProfileType(s string) source.ProfileType {
	s = strings.TrimLeft(s, "-")
	switch s {
	case "heap":
		return source.ProfileHeap
	case "alloc", "allocs":
		return source.ProfileAllocs
	case "goroutine", "goroutines":
		return source.ProfileGoroutine
	default:
		return source.ProfileCPU
	}
}

func runFile(path string) {
	src := &source.FileSource{Path: path}
	prof, err := src.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load profile: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.New(prof, nil), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func runLive(baseURL string, pt source.ProfileType, interval time.Duration) {
	httpSrc := source.NewHTTPSource(baseURL, pt)
	fmt.Fprintf(os.Stderr, "fetching from %s ...\n", httpSrc.URL)

	prof, err := httpSrc.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initial fetch failed: %v\n", err)
		os.Exit(1)
	}

	pollInterval := interval
	if pollInterval == 0 {
		pollInterval = source.DefaultInterval(pt)
	}
	poller := source.NewPoller(httpSrc, pollInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go poller.Run(ctx)

	cfg := &tui.LiveConfig{
		BaseURL:     baseURL,
		Interval:    interval, // 0 = auto per type
		ProfileType: pt,
	}

	model := tui.New(prof, poller.C, tui.WithLiveConfig(cfg, cancel))
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}
