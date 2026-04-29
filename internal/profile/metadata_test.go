package profile_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/profile"
)

func TestMetadata(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 3, p.SampleCount())
	assert.Equal(t, time.Duration(0), p.Duration())
	assert.Equal(t, int64(0), p.Period())
	assert.Empty(t, p.PeriodUnit())
}
