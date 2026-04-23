package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/profile"
)

func TestCallGraph(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	roots := p.CallGraph(0)
	require.NotEmpty(t, roots)

	// The only root should be main.main (all stacks start there).
	assert.Len(t, roots, 1)
	assert.Equal(t, "main.main", roots[0].Func)
	assert.Equal(t, int64(0), roots[0].Self, "main.main is never a leaf")

	// main.main should have two children: foo and bar.
	children := roots[0].Children
	assert.Len(t, children, 2)

	byName := make(map[string]*profile.Node)
	for _, c := range children {
		byName[c.Func] = c
	}

	// foo: cum = 1024 + 512 = 1536, self = 1024 (leaf in sample 1)
	foo := byName["main.foo"]
	require.NotNil(t, foo)
	assert.Equal(t, int64(1536), foo.Cum)
	assert.Equal(t, int64(1024), foo.Self)

	// bar (direct child of main): cum = 2048, self = 2048
	bar := byName["main.bar"]
	require.NotNil(t, bar)
	assert.Equal(t, int64(2048), bar.Cum)
	assert.Equal(t, int64(2048), bar.Self)

	// foo should have one child: bar (from sample 3: main → foo → bar)
	require.Len(t, foo.Children, 1)
	fooBar := foo.Children[0]
	assert.Equal(t, "main.bar", fooBar.Func)
	assert.Equal(t, int64(512), fooBar.Cum)
	assert.Equal(t, int64(512), fooBar.Self)
}

func TestCallGraph_MaxRoots(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	// With maxRoots=1 we should still get main.main (only root anyway).
	roots := p.CallGraph(1)
	assert.Len(t, roots, 1)
}

func TestCallGraph_ChildrenSortedByCum(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	roots := p.CallGraph(0)
	require.Len(t, roots, 1)

	children := roots[0].Children
	for i := 1; i < len(children); i++ {
		assert.GreaterOrEqual(t, children[i-1].Cum, children[i].Cum,
			"children should be sorted by Cum descending")
	}
}

func TestTotalValue(t *testing.T) {
	path := buildTestProfile(t)
	p, err := profile.Load(path)
	require.NoError(t, err)

	// alloc_space total: 1024 + 2048 + 512 = 3584
	assert.Equal(t, int64(3584), p.TotalValue())

	p.SetSampleType("alloc_objects")
	// alloc_objects total: 10 + 5 + 3 = 18
	assert.Equal(t, int64(18), p.TotalValue())
}
