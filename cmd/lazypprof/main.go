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
	"github.com/owenrumney/lazypprof/internal/profile"
	"github.com/owenrumney/lazypprof/internal/source"
	"github.com/owenrumney/lazypprof/internal/tui"
)

func main() {
	interval := flag.Duration("interval", 0, "poll interval for live mode (e.g. 5s, 10s); 0 = auto")
	profileType := flag.String("type", "cpu", "profile type: cpu, heap, allocs, goroutine, mutex, block")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lazypprof [flags] <profile-file | base.prof target.prof | http://host:port>\n\n")
		fmt.Fprintf(os.Stderr, "If no argument is given, lazypprof probes localhost:6060 and connects automatically.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  lazypprof cpu.prof                                  # view a profile file\n")
		fmt.Fprintf(os.Stderr, "  lazypprof base.prof current.prof                     # diff two profiles\n")
		fmt.Fprintf(os.Stderr, "  lazypprof http://localhost:6060                      # live CPU (default)\n")
		fmt.Fprintf(os.Stderr, "  lazypprof -type heap http://localhost:6060            # live heap\n")
		fmt.Fprintf(os.Stderr, "  lazypprof -type mutex http://localhost:6060           # live mutex\n")
		fmt.Fprintf(os.Stderr, "  lazypprof -interval 3s -type allocs http://host:6060  # live allocs, custom interval\n\n")
		fmt.Fprintf(os.Stderr, "Profile types: cpu (default), heap, allocs, goroutine, mutex, block\n")
		fmt.Fprintf(os.Stderr, "Press [m] in live mode to switch between profile types.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	pt := parseProfileType(*profileType)

	if flag.NArg() < 1 {
		if url := probeLocalhost(); url != "" {
			fmt.Fprintf(os.Stderr, "no target given; detected service at %s\n", url)
			runLive(url, pt, *interval)
			return
		}
		flag.Usage()
		os.Exit(1)
	}

	arg := flag.Arg(0)

	if source.Detect(arg) {
		// Backward compat: lazypprof http://... heap
		if flag.NArg() >= 2 && *profileType == "cpu" {
			if legacy := parseProfileType(flag.Arg(1)); legacy != source.ProfileCPU || flag.Arg(1) == "cpu" {
				fmt.Fprintf(os.Stderr, "hint: use -type %s instead of positional argument\n", flag.Arg(1))
				pt = legacy
			}
		}
		runLive(arg, pt, *interval)
	} else if flag.NArg() >= 2 && !source.Detect(flag.Arg(1)) {
		runDiff(arg, flag.Arg(1))
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
	case "mutex":
		return source.ProfileMutex
	case "block":
		return source.ProfileBlock
	default:
		return source.ProfileCPU
	}
}

func runDiff(basePath, targetPath string) {
	baseSrc := &source.FileSource{Path: basePath}
	baseProf, err := baseSrc.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load base profile: %v\n", err)
		os.Exit(1)
	}

	targetSrc := &source.FileSource{Path: targetPath}
	targetProf, err := targetSrc.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load target profile: %v\n", err)
		os.Exit(1)
	}

	diffProf, err := profile.Diff(baseProf, targetProf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to compute diff: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		tui.New(diffProf, nil, tui.WithDiffMode()),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
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
