package profile

import (
	"bytes"
	"fmt"

	pp "github.com/google/pprof/profile"
)

// Diff computes target minus base, returning a profile whose sample values
// represent the delta. Positive values indicate regressions (grew from base
// to target), negative values indicate improvements.
func Diff(base, target *Profile) (*Profile, error) {
	// Round-trip both profiles through serialization so that pprof's Merge
	// sees fully initialized internal state (drop/period types, etc).
	baseClone, err := roundTrip(base.Raw)
	if err != nil {
		return nil, fmt.Errorf("clone base: %w", err)
	}
	targetClone, err := roundTrip(target.Raw)
	if err != nil {
		return nil, fmt.Errorf("clone target: %w", err)
	}

	baseClone.Scale(-1)

	merged, err := pp.Merge([]*pp.Profile{targetClone, baseClone})
	if err != nil {
		return nil, fmt.Errorf("merge diff: %w", err)
	}

	sampleType := ""
	if len(merged.SampleType) > 0 {
		sampleType = merged.SampleType[len(merged.SampleType)-1].Type
	}

	return &Profile{Raw: merged, SampleType: sampleType}, nil
}

func roundTrip(p *pp.Profile) (*pp.Profile, error) {
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		return nil, err
	}
	return pp.Parse(&buf)
}
