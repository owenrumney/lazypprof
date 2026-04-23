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
	URL    string
	Client *http.Client
}

// ProfileType selects which pprof endpoint to hit.
type ProfileType string

const (
	ProfileCPU    ProfileType = "cpu"
	ProfileHeap   ProfileType = "heap"
	ProfileAllocs ProfileType = "allocs"
)

// pprofPath returns the /debug/pprof/... path for a profile type.
func (pt ProfileType) pprofPath() string {
	switch pt {
	case ProfileHeap:
		return "/debug/pprof/heap"
	case ProfileAllocs:
		return "/debug/pprof/allocs"
	default:
		return "/debug/pprof/profile?seconds=5"
	}
}

// NewHTTPSource creates an HTTPSource for the given base URL and profile type.
func NewHTTPSource(baseURL string, pt ProfileType) *HTTPSource {
	base := strings.TrimRight(baseURL, "/")
	return &HTTPSource{
		URL: base + pt.pprofPath(),
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *HTTPSource) Load() (*profile.Profile, error) {
	resp, err := s.Client.Get(s.URL)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d from %s", resp.StatusCode, s.URL)
	}

	return parseProfile(resp.Body)
}

// Poller periodically fetches profiles from an HTTPSource and delivers them
// on a channel.
type Poller struct {
	Source   *HTTPSource
	Interval time.Duration
	C        chan *profile.Profile // new profiles arrive here
}

// NewPoller creates a Poller that fetches from src every interval.
func NewPoller(src *HTTPSource, interval time.Duration) *Poller {
	return &Poller{
		Source:   src,
		Interval: interval,
		C:        make(chan *profile.Profile, 1),
	}
}

// Run starts polling. Blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	// Fetch immediately on start.
	if prof, err := p.Source.Load(); err == nil {
		select {
		case p.C <- prof:
		default:
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prof, err := p.Source.Load()
			if err != nil {
				continue // keep showing the previous profile
			}
			select {
			case p.C <- prof:
			default:
			}
		}
	}
}

// DefaultInterval returns a sensible poll interval for the profile type.
func DefaultInterval(pt ProfileType) time.Duration {
	switch pt {
	case ProfileHeap, ProfileAllocs:
		return 5 * time.Second
	default:
		return 10 * time.Second
	}
}

// Detect returns true if the argument looks like an HTTP URL.
func Detect(arg string) bool {
	return strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://")
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
