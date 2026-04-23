package profile_test

import (
	"testing"

	pp "github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/profile"
)

func TestDiff(t *testing.T) {
	fn := &pp.Function{ID: 1, Name: "main.hot", Filename: "main.go"}
	loc := &pp.Location{ID: 1, Line: []pp.Line{{Function: fn}}}

	makeProf := func(value int64) *profile.Profile {
		raw := &pp.Profile{
			SampleType: []*pp.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
			Sample:     []*pp.Sample{{Location: []*pp.Location{loc}, Value: []int64{value}}},
			Function:   []*pp.Function{fn},
			Location:   []*pp.Location{loc},
		}
		return &profile.Profile{Raw: raw, SampleType: "cpu"}
	}

	base := makeProf(100)
	target := makeProf(250)

	diff, err := profile.Diff(base, target)
	require.NoError(t, err)
	assert.Equal(t, "cpu", diff.SampleType)

	stats := diff.TopFunctions()
	require.NotEmpty(t, stats)

	var found bool
	for _, s := range stats {
		if s.Name == "main.hot" {
			assert.Equal(t, int64(150), s.Flat, "expected target(250) - base(100) = 150")
			found = true
		}
	}
	assert.True(t, found, "expected main.hot in diff stats")
}

func TestDiff_Negative(t *testing.T) {
	fn := &pp.Function{ID: 1, Name: "main.improved", Filename: "main.go"}
	loc := &pp.Location{ID: 1, Line: []pp.Line{{Function: fn}}}

	makeProf := func(value int64) *profile.Profile {
		raw := &pp.Profile{
			SampleType: []*pp.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
			Sample:     []*pp.Sample{{Location: []*pp.Location{loc}, Value: []int64{value}}},
			Function:   []*pp.Function{fn},
			Location:   []*pp.Location{loc},
		}
		return &profile.Profile{Raw: raw, SampleType: "cpu"}
	}

	base := makeProf(500)
	target := makeProf(200)

	diff, err := profile.Diff(base, target)
	require.NoError(t, err)

	stats := diff.TopFunctions()
	require.NotEmpty(t, stats)

	var found bool
	for _, s := range stats {
		if s.Name == "main.improved" {
			assert.Equal(t, int64(-300), s.Flat, "expected target(200) - base(500) = -300")
			found = true
		}
	}
	assert.True(t, found, "expected main.improved in diff stats")
}
