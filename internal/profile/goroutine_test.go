package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/profile"
)

const testGoroutineText = `goroutine 1 [running]:
main.main()
	/home/user/app/main.go:15 +0x1a0
runtime.main()
	/usr/local/go/src/runtime/proc.go:250 +0x1c8

goroutine 18 [IO wait, 5 minutes]:
internal/poll.runtime_pollWait(0x1400012e1e0, 0x72)
	/usr/local/go/src/runtime/netpoll.go:345 +0xa0
net.(*netFD).Read(0x14000120180, {0x14000148000, 0x1000, 0x1000})
	/usr/local/go/src/net/fd_posix.go:55 +0x28

goroutine 22 [select]:
net/http.(*persistConn).writeLoop(0x140001a6000)
	/usr/local/go/src/net/http/transport.go:2392 +0x94

goroutine 33 [IO wait]:
internal/poll.runtime_pollWait(0x1400012e2a0, 0x72)
	/usr/local/go/src/runtime/netpoll.go:345 +0xa0
net.(*netFD).Read(0x14000120280, {0x14000149000, 0x1000, 0x1000})
	/usr/local/go/src/net/fd_posix.go:55 +0x28

goroutine 44 [chan receive]:
main.worker(0x0?)
	/home/user/app/worker.go:10 +0x34

goroutine 45 [chan receive]:
main.worker(0x0?)
	/home/user/app/worker.go:10 +0x34
`

func TestParseGoroutineText(t *testing.T) {
	gs, err := profile.ParseGoroutineText([]byte(testGoroutineText))
	require.NoError(t, err)
	assert.Len(t, gs, 6)

	// Check first goroutine.
	assert.Equal(t, 1, gs[0].ID)
	assert.Equal(t, "running", gs[0].State)
	assert.Len(t, gs[0].Stack, 2)
	assert.Equal(t, "main.main", gs[0].Stack[0].Func)
	assert.Equal(t, "/home/user/app/main.go", gs[0].Stack[0].File)
	assert.Equal(t, 15, gs[0].Stack[0].Line)

	// IO wait goroutine — state should strip the duration.
	assert.Equal(t, 18, gs[1].ID)
	assert.Equal(t, "IO wait", gs[1].State)

	// select goroutine.
	assert.Equal(t, 22, gs[2].ID)
	assert.Equal(t, "select", gs[2].State)
}

func TestGroupByState(t *testing.T) {
	gs, err := profile.ParseGoroutineText([]byte(testGoroutineText))
	require.NoError(t, err)

	groups := profile.GroupByState(gs)

	// Should be sorted by count descending.
	for i := 1; i < len(groups); i++ {
		assert.GreaterOrEqual(t, groups[i-1].Count, groups[i].Count,
			"groups should be sorted by count descending")
	}

	byState := make(map[string]int)
	for _, g := range groups {
		byState[g.State] = g.Count
	}

	assert.Equal(t, 2, byState["IO wait"])
	assert.Equal(t, 2, byState["chan receive"])
	assert.Equal(t, 1, byState["running"])
	assert.Equal(t, 1, byState["select"])
}

func TestGoroutineStates(t *testing.T) {
	gs, err := profile.ParseGoroutineText([]byte(testGoroutineText))
	require.NoError(t, err)

	states := profile.GoroutineStates(gs)
	require.Len(t, states, 4)
	// First two should be the ones with count=2 (order between them is stable by sort).
	assert.Contains(t, states[:2], "IO wait")
	assert.Contains(t, states[:2], "chan receive")
}

func TestBuildSyntheticProfile(t *testing.T) {
	gs, err := profile.ParseGoroutineText([]byte(testGoroutineText))
	require.NoError(t, err)

	prof := profile.BuildSyntheticProfile(gs)
	assert.NotNil(t, prof)
	assert.Len(t, prof.SampleType, 1)
	assert.Equal(t, "goroutines", prof.SampleType[0].Type)
	assert.Equal(t, "count", prof.SampleType[0].Unit)
	// One sample per goroutine.
	assert.Len(t, prof.Sample, 6)
}

func TestParseGoroutineText_Empty(t *testing.T) {
	gs, err := profile.ParseGoroutineText([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, gs)
}
