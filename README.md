# lazypprof

A keyboard-driven TUI for exploring Go pprof profiles. Faster than `go tool pprof -http`, no browser needed — works in any terminal, including over SSH.

## Install

```bash
go install github.com/owenrumney/lazypprof/cmd/lazypprof@latest
```

### Homebrew

```bash
brew tap owenrumney/tools
brew install --cask lazypprof
```

Or grab a binary from [Releases](https://github.com/owenrumney/lazypprof/releases).

## Usage

```bash
# Load a profile from disk
lazypprof cpu.prof
lazypprof heap.pb.gz

# Diff two profiles (positive = regression, negative = improvement)
lazypprof base.prof current.prof

# Connect to a live service
lazypprof http://localhost:6060                      # CPU (default)
lazypprof -type heap http://localhost:6060            # heap
lazypprof -type allocs http://localhost:6060          # allocs
lazypprof -type goroutine http://localhost:6060       # goroutines (with state filtering)
lazypprof -type mutex http://localhost:6060           # mutex contention
lazypprof -type block http://localhost:6060           # blocking operations

# Custom poll interval (CPU capture duration follows this, clamped to 1s-30s)
lazypprof -interval 3s -type allocs http://localhost:6060

# Protected live services
lazypprof -H 'Authorization: Bearer token' https://service.internal:6060
lazypprof -user alice -password "$TOKEN" https://service.internal:6060

# Auto-detect: if nothing is specified, probes localhost:6060
lazypprof
```

Profile types: `cpu` (default), `heap`, `allocs`, `goroutine`, `mutex`, `block`

Live headers can be repeated with `-H` or `--header`. Basic auth is available with `-user` and `-password`.

## Demo

### File profile

![prof file](./.github/images/cpu_file.gif)

### Live service

![live](./.github/images/live.gif)

## Views

The header includes compact metadata such as sample count, total value, profile duration when present, live update time, and CPU capture duration.

**Top** — functions ranked by cumulative value. Standard pprof top view. In diff mode, rows are coloured red (regression) or green (improvement) with delta columns.

**Tree** — collapsible call tree. Expand/collapse nodes, navigate with keyboard. Filter matches are highlighted and paths auto-expanded.

**Flame** — terminal-rendered flame graph. Zoom in/out, navigate frames. Filter matches are highlighted; non-matching frames are dimmed.

**Goroutines** — goroutines grouped by state (running, IO wait, select, etc). Drill into a state to see unique stacks with counts.

**Source** — hot source lines for a selected function. If the file exists locally, lazypprof shows source text; otherwise it still shows file/line aggregates from the profile.

## Keys

| Key   | Action                          |
| ----- | ------------------------------- |
| `tab` | Cycle views                     |
| `s`   | Cycle sample type               |
| `/`   | Filter by function name (regex) |
| `esc` | Clear filter                    |
| `[`   | Step back through history        |
| `]`   | Step forward through history     |
| `p`   | Pause/resume live updates       |
| `m`   | Switch profile type (live mode) |
| `?`   | Help overlay                    |
| `q`   | Quit                            |

### Top

| Key         | Action                         |
| ----------- | ------------------------------ |
| `j/k` `↑/↓` | Navigate rows                  |
| `c`         | Sort by cumulative value       |
| `f`         | Sort by flat value             |
| `n`         | Sort by function name          |
| `enter/l`   | Open Source view for function  |

### Tree

| Key         | Action            |
| ----------- | ----------------- |
| `j/k` `↑/↓` | Navigate          |
| `l/→/enter` | Expand            |
| `h/←`       | Collapse / parent |
| `space`     | Toggle            |
| `*`         | Expand subtree    |
| `0`         | Collapse all      |
| `shift+l`   | Open Source view  |

### Flame

| Key              | Action          |
| ---------------- | --------------- |
| `h/j/k/l` `←↓↑→` | Navigate frames |
| `enter`          | Zoom in         |
| `backspace`      | Zoom out        |
| `0`              | Reset zoom      |
| `shift+l`        | Open Source view |

### Goroutines

| Key         | Action             |
| ----------- | ------------------ |
| `j/k` `↑/↓` | Navigate groups    |
| `g`         | Cycle state filter |
| `enter`     | Drill into state   |
| `backspace` | Back to groups     |

### Source

| Key         | Action             |
| ----------- | ------------------ |
| `j/k` `↑/↓` | Navigate hot lines |

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
