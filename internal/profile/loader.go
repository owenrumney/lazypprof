// Package profile wraps github.com/google/pprof/profile with the helpers
// lazypprof needs: loading, sample-type switching, and Top aggregation.
package profile

import (
	"fmt"
	"os"
	"sort"
	"time"

	pp "github.com/google/pprof/profile"
)

// Profile is a loaded pprof profile plus the currently selected sample type.
type Profile struct {
	Raw        *pp.Profile
	SampleType string      // e.g. "cpu", "inuse_space", "alloc_objects"
	Goroutines []Goroutine // populated for goroutine profiles (debug=2 parse)
	FetchedAt  time.Time   // when this profile was fetched (zero for file-loaded profiles)
}

// Load reads a pprof file (gzipped or not) from disk.
func Load(path string) (*Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	p, err := pp.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if err := p.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid profile: %w", err)
	}

	sampleType := ""
	if len(p.SampleType) > 0 {
		// Default to the *last* sample type — for CPU profiles this is "cpu"
		// (nanoseconds), and for heap profiles it's "inuse_space", which is
		// what people usually want first.
		sampleType = p.SampleType[len(p.SampleType)-1].Type
	}

	return &Profile{Raw: p, SampleType: sampleType}, nil
}

// Empty reports whether the profile contains no meaningful sample data.
func (p *Profile) Empty() bool {
	idx := p.sampleIndex()
	if idx < 0 {
		return true
	}
	for _, s := range p.Raw.Sample {
		if s.Value[idx] != 0 {
			return false
		}
	}
	return true
}

// FunctionStat is the per-function aggregate displayed in the Top view.
type FunctionStat struct {
	Name string
	File string
	Flat int64 // value attributed directly to this function (leaf only)
	Cum  int64 // value attributed to this function or anything it called
}

// TopFunctions aggregates samples by leaf/cumulative for the active sample type.
func (p *Profile) TopFunctions() []FunctionStat {
	idx := p.sampleIndex()
	if idx < 0 {
		return nil
	}

	flat := make(map[uint64]int64)
	cum := make(map[uint64]int64)
	nameOf := make(map[uint64]string)
	fileOf := make(map[uint64]string)

	for _, s := range p.Raw.Sample {
		if len(s.Location) == 0 {
			continue
		}
		v := s.Value[idx]

		if leaf := firstFunction(s.Location[0]); leaf != nil {
			flat[leaf.ID] += v
			nameOf[leaf.ID] = leaf.Name
			fileOf[leaf.ID] = leaf.Filename
		}

		// Cumulative: every unique function appearing in the stack.
		seen := make(map[uint64]bool)
		for _, loc := range s.Location {
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				fid := line.Function.ID
				if seen[fid] {
					continue
				}
				seen[fid] = true
				cum[fid] += v
				if _, ok := nameOf[fid]; !ok {
					nameOf[fid] = line.Function.Name
					fileOf[fid] = line.Function.Filename
				}
			}
		}
	}

	out := make([]FunctionStat, 0, len(cum))
	for fid, c := range cum {
		out = append(out, FunctionStat{
			Name: nameOf[fid],
			File: fileOf[fid],
			Flat: flat[fid],
			Cum:  c,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cum > out[j].Cum })
	return out
}

// SampleTypes returns the available sample type names in this profile.
func (p *Profile) SampleTypes() []string {
	names := make([]string, len(p.Raw.SampleType))
	for i, st := range p.Raw.SampleType {
		names[i] = st.Type
	}
	return names
}

// SetSampleType switches the active sample type. Returns false if not present.
func (p *Profile) SetSampleType(name string) bool {
	for _, st := range p.Raw.SampleType {
		if st.Type == name {
			p.SampleType = name
			return true
		}
	}
	return false
}

// Unit returns the unit string for the active sample type
// (e.g. "nanoseconds", "bytes", "count").
func (p *Profile) Unit() string {
	idx := p.sampleIndex()
	if idx < 0 {
		return ""
	}
	return p.Raw.SampleType[idx].Unit
}

func (p *Profile) sampleIndex() int {
	for i, st := range p.Raw.SampleType {
		if st.Type == p.SampleType {
			return i
		}
	}
	return -1
}

func firstFunction(loc *pp.Location) *pp.Function {
	if loc == nil || len(loc.Line) == 0 {
		return nil
	}
	return loc.Line[0].Function
}
