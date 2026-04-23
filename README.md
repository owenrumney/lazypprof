# lazypprof

A keyboard-driven TUI for exploring Go pprof profiles. Faster than `go tool pprof -http`, works over SSH, no browser needed.

## Install

```bash
go install github.com/owenrumney/lazypprof/cmd/lazypprof@latest
```

Or grab a binary from [Releases](https://github.com/owenrumney/lazypprof/releases).

## Usage

```bash
# Load a profile from disk
lazypprof cpu.prof
lazypprof heap.pb.gz

# Connect to a live service
lazypprof http://localhost:6060              # CPU (default)
lazypprof http://localhost:6060 heap         # heap
lazypprof http://localhost:6060 allocs       # allocs
lazypprof http://localhost:6060 goroutine    # goroutines (with state filtering)

# Custom poll interval
lazypprof -interval 3s http://localhost:6060 heap
```

## Demo

### File profile

<video src=".github/images/cpu_file.mp4" controls width="100%"></video>

### Live service

<video src=".github/images/live.mp4" controls width="100%"></video>

## Views

**Top** — functions ranked by cumulative value. Standard pprof top view.

**Tree** — collapsible call tree. Expand/collapse nodes, navigate with keyboard.

**Flame** — terminal-rendered flame graph. Zoom in/out, navigate frames.

**Goroutines** — goroutines grouped by state (running, IO wait, select, etc). Drill into a state to see unique stacks with counts.

## Keys

| Key | Action |
|-----|--------|
| `tab` | Cycle views |
| `s` | Cycle sample type |
| `/` | Filter by function name (regex) |
| `esc` | Clear filter |
| `?` | Help overlay |
| `q` | Quit |

### Tree

| Key | Action |
|-----|--------|
| `j/k` `↑/↓` | Navigate |
| `l/→/enter` | Expand |
| `h/←` | Collapse / parent |
| `space` | Toggle |
| `*` | Expand subtree |
| `0` | Collapse all |

### Flame

| Key | Action |
|-----|--------|
| `h/j/k/l` `←↓↑→` | Navigate frames |
| `enter` | Zoom in |
| `backspace` | Zoom out |
| `0` | Reset zoom |

### Goroutines

| Key | Action |
|-----|--------|
| `j/k` `↑/↓` | Navigate groups |
| `g` | Cycle state filter |
| `enter` | Drill into state |
| `backspace` | Back to groups |

## Building

```bash
go build ./cmd/lazypprof
```

## Test Service

A built-in test service generates realistic workload with pprof endpoints:

```bash
go run ./cmd/testservice
# then: lazypprof http://localhost:6060
```
