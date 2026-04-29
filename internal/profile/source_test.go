package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/profile"
)

func TestSourceLines(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	lines := p.SourceLines("main.foo")
	require.NotEmpty(t, lines)

	var foo profile.LineStat
	for _, line := range lines {
		if line.Function == "main.foo" {
			foo = line
			break
		}
	}

	assert.Equal(t, "foo.go", foo.File)
	assert.Equal(t, int64(1024), foo.Flat)
	assert.Equal(t, int64(1536), foo.Cum)
}
