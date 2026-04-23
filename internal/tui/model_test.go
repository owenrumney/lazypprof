package tui

import (
	"errors"
	"testing"

	pp "github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/profile"
)

func newTestProfile() *profile.Profile {
	fn := &pp.Function{ID: 1, Name: "main.main", Filename: "main.go"}
	loc := &pp.Location{ID: 1, Line: []pp.Line{{Function: fn}}}
	raw := &pp.Profile{
		SampleType: []*pp.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		Sample:     []*pp.Sample{{Location: []*pp.Location{loc}, Value: []int64{100}}},
		Function:   []*pp.Function{fn},
		Location:   []*pp.Location{loc},
	}
	return &profile.Profile{Raw: raw, SampleType: "cpu"}
}

func TestUpdate_ModeSwitchErr_ClearsSwitching(t *testing.T) {
	m := New(newTestProfile(), nil)
	m.switching = "Switching to goroutine..."

	updated, cmd := m.Update(modeSwitchErrMsg{err: errors.New("connection refused")})

	result, ok := updated.(Model)
	require.True(t, ok)
	assert.Empty(t, result.switching)
	assert.Nil(t, cmd)
}
