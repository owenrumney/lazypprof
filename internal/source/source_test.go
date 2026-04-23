package source_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	pp "github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/owenrumney/lazypprof/internal/source"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		arg  string
		want bool
	}{
		{"http://localhost:6060", true},
		{"https://example.com/debug/pprof/heap", true},
		{"/tmp/cpu.prof", false},
		{"./profile.pb.gz", false},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			assert.Equal(t, tt.want, source.Detect(tt.arg))
		})
	}
}

func TestFileSource_Load(t *testing.T) {
	path := buildTestProfileFile(t)
	src := &source.FileSource{Path: path}

	p, err := src.Load()
	require.NoError(t, err)
	assert.NotNil(t, p.Raw)
	assert.NotEmpty(t, p.SampleType)
}

func TestFileSource_Load_NotFound(t *testing.T) {
	src := &source.FileSource{Path: "/tmp/nonexistent-abc123.prof"}
	_, err := src.Load()
	require.Error(t, err)
}

func TestHTTPSource_Load(t *testing.T) {
	profData := buildTestProfileBytes(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(profData)
	}))
	defer srv.Close()

	src := &source.HTTPSource{
		URL:    srv.URL,
		Client: srv.Client(),
	}

	p, err := src.Load()
	require.NoError(t, err)
	assert.NotNil(t, p.Raw)
	assert.NotEmpty(t, p.SampleType)
}

func TestHTTPSource_Load_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	src := &source.HTTPSource{
		URL:    srv.URL,
		Client: srv.Client(),
	}

	_, err := src.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestNewHTTPSource_CPU(t *testing.T) {
	src := source.NewHTTPSource("http://localhost:6060", source.ProfileCPU)
	assert.Contains(t, src.URL, "/debug/pprof/profile")
}

func TestNewHTTPSource_Heap(t *testing.T) {
	src := source.NewHTTPSource("http://localhost:6060", source.ProfileHeap)
	assert.Equal(t, "http://localhost:6060/debug/pprof/heap", src.URL)
}

func TestNewHTTPSource_Allocs(t *testing.T) {
	src := source.NewHTTPSource("http://localhost:6060", source.ProfileAllocs)
	assert.Equal(t, "http://localhost:6060/debug/pprof/allocs", src.URL)
}

func TestDefaultInterval(t *testing.T) {
	assert.Equal(t, 10*time.Second, source.DefaultInterval(source.ProfileCPU))
	assert.Equal(t, 5*time.Second, source.DefaultInterval(source.ProfileHeap))
	assert.Equal(t, 5*time.Second, source.DefaultInterval(source.ProfileAllocs))
	assert.Equal(t, 5*time.Second, source.DefaultInterval(source.ProfileGoroutine))
}

func TestNewHTTPSource_Goroutine(t *testing.T) {
	src := source.NewHTTPSource("http://localhost:6060", source.ProfileGoroutine)
	assert.Equal(t, "http://localhost:6060/debug/pprof/goroutine?debug=2", src.URL)
}

const minimalGoroutineText = `goroutine 1 [running]:
main.main()
	/home/user/app/main.go:10 +0x1a0

goroutine 2 [IO wait]:
net.(*netFD).Read(0x14000120180, {0x14000148000, 0x1000, 0x1000})
	/usr/local/go/src/net/fd_posix.go:55 +0x28

`

func TestHTTPSource_Load_Goroutine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(minimalGoroutineText))
	}))
	defer srv.Close()

	src := &source.HTTPSource{
		URL:         srv.URL,
		Client:      srv.Client(),
		ProfileType: source.ProfileGoroutine,
	}

	p, err := src.Load()
	require.NoError(t, err)
	assert.NotNil(t, p.Raw)
	assert.Len(t, p.Goroutines, 2)
	assert.Equal(t, "running", p.Goroutines[0].State)
	assert.Equal(t, "IO wait", p.Goroutines[1].State)
}

func TestPoller_DeliversProfile(t *testing.T) {
	profData := buildTestProfileBytes(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(profData)
	}))
	defer srv.Close()

	src := &source.HTTPSource{
		URL:    srv.URL,
		Client: srv.Client(),
	}

	poller := source.NewPoller(src, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go poller.Run(ctx)

	select {
	case p := <-poller.C:
		require.NotNil(t, p)
		assert.NotEmpty(t, p.SampleType)
	case <-ctx.Done():
		t.Fatal("timed out waiting for profile from poller")
	}
}

// helpers

func buildTestProfileFile(t *testing.T) string {
	t.Helper()
	raw := minimalProfile()
	f, err := os.CreateTemp(t.TempDir(), "test-*.prof")
	require.NoError(t, err)
	require.NoError(t, raw.Write(f))
	require.NoError(t, f.Close())
	return f.Name()
}

func buildTestProfileBytes(t *testing.T) []byte {
	t.Helper()
	raw := minimalProfile()
	var buf bytes.Buffer
	require.NoError(t, raw.Write(&buf))
	return buf.Bytes()
}

func minimalProfile() *pp.Profile {
	fn := &pp.Function{ID: 1, Name: "main.main", Filename: "main.go"}
	loc := &pp.Location{ID: 1, Line: []pp.Line{{Function: fn}}}
	return &pp.Profile{
		SampleType: []*pp.ValueType{
			{Type: "cpu", Unit: "nanoseconds"},
		},
		Sample: []*pp.Sample{
			{Location: []*pp.Location{loc}, Value: []int64{100}},
		},
		Function: []*pp.Function{fn},
		Location: []*pp.Location{loc},
	}
}
