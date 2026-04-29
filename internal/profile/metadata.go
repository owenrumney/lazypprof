package profile

import "time"

// SampleCount returns the number of samples in the raw profile.
func (p *Profile) SampleCount() int {
	if p == nil || p.Raw == nil {
		return 0
	}
	return len(p.Raw.Sample)
}

// Duration returns the profile duration when present.
func (p *Profile) Duration() time.Duration {
	if p == nil || p.Raw == nil || p.Raw.DurationNanos <= 0 {
		return 0
	}
	return time.Duration(p.Raw.DurationNanos)
}

// Period returns the active profile period when present.
func (p *Profile) Period() int64 {
	if p == nil || p.Raw == nil {
		return 0
	}
	return p.Raw.Period
}

// PeriodUnit returns the active profile period unit when present.
func (p *Profile) PeriodUnit() string {
	if p == nil || p.Raw == nil || p.Raw.PeriodType == nil {
		return ""
	}
	return p.Raw.PeriodType.Unit
}
