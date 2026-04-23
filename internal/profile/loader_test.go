package profile_test

import (
	"os"
	"testing"

	pp "github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/profile"
)

// buildTestProfile creates a minimal pprof profile in a temp file and returns
// the path. The profile has two sample types (alloc_objects/count and
// alloc_space/bytes) and a handful of samples across three functions.
func buildTestProfile(t *testing.T) string {
	t.Helper()

	fnMain := &pp.Function{ID: 1, Name: "main.main", Filename: "main.go"}
	fnFoo := &pp.Function{ID: 2, Name: "main.foo", Filename: "foo.go"}
	fnBar := &pp.Function{ID: 3, Name: "main.bar", Filename: "bar.go"}

	loc1 := &pp.Location{ID: 1, Line: []pp.Line{{Function: fnMain}}}
	loc2 := &pp.Location{ID: 2, Line: []pp.Line{{Function: fnFoo}}}
	loc3 := &pp.Location{ID: 3, Line: []pp.Line{{Function: fnBar}}}

	raw := &pp.Profile{
		SampleType: []*pp.ValueType{
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
		},
		Sample: []*pp.Sample{
			// main → foo (leaf)
			{Location: []*pp.Location{loc2, loc1}, Value: []int64{10, 1024}},
			// main → bar (leaf)
			{Location: []*pp.Location{loc3, loc1}, Value: []int64{5, 2048}},
			// main → foo → bar (leaf)
			{Location: []*pp.Location{loc3, loc2, loc1}, Value: []int64{3, 512}},
		},
		Function: []*pp.Function{fnMain, fnFoo, fnBar},
		Location: []*pp.Location{loc1, loc2, loc3},
	}

	f, err := os.CreateTemp(t.TempDir(), "test-*.prof")
	require.NoError(t, err)
	require.NoError(t, raw.Write(f))
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoad(t *testing.T) {
	path := buildTestProfile(t)

	p, err := profile.Load(path)
	require.NoError(t, err)
	assert.NotNil(t, p.Raw)
	// Default sample type should be the last one.
	assert.Equal(t, "alloc_space", p.SampleType)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := profile.Load("/tmp/nonexistent-profile-abc123.prof")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open")
}

func TestSampleTypes(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	types := p.SampleTypes()
	assert.Equal(t, []string{"alloc_objects", "alloc_space"}, types)
}

func TestSetSampleType(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	assert.True(t, p.SetSampleType("alloc_objects"))
	assert.Equal(t, "alloc_objects", p.SampleType)

	assert.False(t, p.SetSampleType("nonexistent"))
	assert.Equal(t, "alloc_objects", p.SampleType)
}

func TestUnit(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "bytes", p.Unit())

	p.SetSampleType("alloc_objects")
	assert.Equal(t, "count", p.Unit())
}

func TestTopFunctions(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	// Using alloc_space (bytes): default sample type.
	top := p.TopFunctions()
	require.NotEmpty(t, top)

	// Should be sorted by Cum descending.
	for i := 1; i < len(top); i++ {
		assert.GreaterOrEqual(t, top[i-1].Cum, top[i].Cum,
			"top functions should be sorted by Cum descending")
	}

	// main.main appears in every sample cumulatively.
	byName := make(map[string]profile.FunctionStat)
	for _, s := range top {
		byName[s.Name] = s
	}

	// main.main: cum = 1024 + 2048 + 512 = 3584, flat = 0 (never a leaf)
	assert.Equal(t, int64(3584), byName["main.main"].Cum)
	assert.Equal(t, int64(0), byName["main.main"].Flat)

	// main.bar: leaf in samples 2 and 3 → flat = 2048 + 512 = 2560
	assert.Equal(t, int64(2560), byName["main.bar"].Flat)

	// main.foo: leaf in sample 1 → flat = 1024; cum includes sample 3 → 1024 + 512 = 1536
	assert.Equal(t, int64(1024), byName["main.foo"].Flat)
	assert.Equal(t, int64(1536), byName["main.foo"].Cum)
}
