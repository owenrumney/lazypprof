// Package source provides profile sources: local files and HTTP endpoints.
package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pp "github.com/google/pprof/profile"

	"github.com/owenrumney/lazypprof/internal/profile"
)

// Source loads a profile. File sources load once; HTTP sources poll.
type Source interface {
	// Load fetches the current profile.
	Load() (*profile.Profile, error)
}

// FileSource loads a profile from a local file path.
type FileSource struct {
	Path string
}

func (s *FileSource) Load() (*profile.Profile, error) {
	return profile.Load(s.Path)
}

// HTTPSource polls a /debug/pprof endpoint for profiles.
type HTTPSource struct {
	URL         string
	Client      *http.Client
	ProfileType ProfileType
	CPUSeconds  int
	Headers     http.Header
	Username    string
	Password    string
}

// HTTPConfig holds live HTTP source options.
type HTTPConfig struct {
	Interval time.Duration
	Headers  http.Header
	Username string
	Password string
}

// ProfileType selects which pprof endpoint to hit.
type ProfileType string

const (
	ProfileCPU       ProfileType = "cpu"
	ProfileHeap      ProfileType = "heap"
	ProfileAllocs    ProfileType = "allocs"
	ProfileGoroutine ProfileType = "goroutine"
	ProfileMutex     ProfileType = "mutex"
	ProfileBlock     ProfileType = "block"
)

// pprofPath returns the /debug/pprof/... path for a profile type.
func (pt ProfileType) pprofPath(cpuSeconds int) string {
	switch pt {
	case ProfileHeap:
		return "/debug/pprof/heap"
	case ProfileAllocs:
		return "/debug/pprof/allocs"
	case ProfileGoroutine:
		return "/debug/pprof/goroutine?debug=2"
	case ProfileMutex:
		return "/debug/pprof/mutex"
	case ProfileBlock:
		return "/debug/pprof/block"
	default:
		if cpuSeconds <= 0 {
			cpuSeconds = 5
		}
		return fmt.Sprintf("/debug/pprof/profile?seconds=%d", cpuSeconds)
	}
}

// NewHTTPSource creates an HTTPSource for the given base URL and profile type.
func NewHTTPSource(baseURL string, pt ProfileType) *HTTPSource {
	return NewHTTPSourceWithInterval(baseURL, pt, 0)
}

// NewHTTPSourceWithInterval creates an HTTPSource whose CPU capture duration
// follows the requested poll interval. Non-CPU profile URLs are unaffected.
func NewHTTPSourceWithInterval(baseURL string, pt ProfileType, interval time.Duration) *HTTPSource {
	return NewHTTPSourceWithConfig(baseURL, pt, HTTPConfig{Interval: interval})
}

// NewHTTPSourceWithConfig creates an HTTPSource for a live pprof endpoint.
func NewHTTPSourceWithConfig(baseURL string, pt ProfileType, cfg HTTPConfig) *HTTPSource {
	base := strings.TrimRight(baseURL, "/")
	cpuSeconds := cpuSecondsForInterval(cfg.Interval)
	return &HTTPSource{
		URL:         base + pt.pprofPath(cpuSeconds),
		ProfileType: pt,
		CPUSeconds:  cpuSeconds,
		Headers:     cloneHeader(cfg.Headers),
		Username:    cfg.Username,
		Password:    cfg.Password,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func cloneHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	return h.Clone()
}

func cpuSecondsForInterval(interval time.Duration) int {
	if interval <= 0 {
		return 5
	}
	if interval < time.Second {
		return 1
	}
	if interval > 30*time.Second {
		return 30
	}
	return int(interval / time.Second)
}

func (s *HTTPSource) Load() (*profile.Profile, error) {
	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	for name, values := range s.Headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if s.Username != "" || s.Password != "" {
		req.SetBasicAuth(s.Username, s.Password)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d from %s", resp.StatusCode, s.URL)
	}

	var prof *profile.Profile
	if s.ProfileType == ProfileGoroutine {
		prof, err = parseGoroutineProfile(resp.Body)
	} else {
		prof, err = parseProfile(resp.Body)
	}
	if err != nil {
		return nil, err
	}
	prof.FetchedAt = time.Now()
	return prof, nil
}

// Poller periodically fetches profiles from an HTTPSource and delivers them
// on a channel.
type Poller struct {
	Source   *HTTPSource
	Interval time.Duration
	C        chan PollEvent // new profiles and refresh errors arrive here
}

// PollEvent reports the outcome of one poll attempt.
type PollEvent struct {
	Profile *profile.Profile
	Err     error
	At      time.Time
}

// NewPoller creates a Poller that fetches from src every interval.
func NewPoller(src *HTTPSource, interval time.Duration) *Poller {
	return &Poller{
		Source:   src,
		Interval: interval,
		C:        make(chan PollEvent, 1),
	}
}

// Run starts polling. Blocks until ctx is cancelled. Closes p.C on exit.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()
	defer close(p.C)

	// Fetch immediately on start.
	p.deliver()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.deliver()
		}
	}
}

func (p *Poller) deliver() {
	prof, err := p.Source.Load()
	ev := PollEvent{Profile: prof, Err: err, At: time.Now()}
	select {
	case p.C <- ev:
	default:
	}
}

// DefaultInterval returns a sensible poll interval for the profile type.
func DefaultInterval(pt ProfileType) time.Duration {
	switch pt {
	case ProfileHeap, ProfileAllocs, ProfileGoroutine, ProfileMutex, ProfileBlock:
		return 5 * time.Second
	default:
		return 10 * time.Second
	}
}

// Detect returns true if the argument looks like an HTTP URL.
func Detect(arg string) bool {
	return strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://")
}

func parseGoroutineProfile(r io.Reader) (*profile.Profile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read goroutine data: %w", err)
	}
	goroutines, err := profile.ParseGoroutineText(data)
	if err != nil {
		return nil, fmt.Errorf("parse goroutine text: %w", err)
	}
	if len(goroutines) == 0 {
		return nil, fmt.Errorf("no goroutines found in response")
	}

	synthetic := profile.BuildSyntheticProfile(goroutines)
	sampleType := ""
	if len(synthetic.SampleType) > 0 {
		sampleType = synthetic.SampleType[len(synthetic.SampleType)-1].Type
	}

	return &profile.Profile{
		Raw:        synthetic,
		SampleType: sampleType,
		Goroutines: goroutines,
	}, nil
}

func parseProfile(r io.Reader) (*profile.Profile, error) {
	raw, err := pp.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if err := raw.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}

	sampleType := ""
	if len(raw.SampleType) > 0 {
		sampleType = raw.SampleType[len(raw.SampleType)-1].Type
	}

	return &profile.Profile{Raw: raw, SampleType: sampleType}, nil
}
