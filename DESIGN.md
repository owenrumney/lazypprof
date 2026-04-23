# lazypprof — v0.1 Design

> Status: draft
> Scope: v0.1 only

## Goal

A keyboard-driven TUI for exploring Go pprof profiles. Faster feedback loop than `go tool pprof -http`, usable over SSH, no browser.

## Non-goals (v0.1)

- Diff between profiles
- Source-annotated view
- Profile types beyond CPU and heap (no goroutine, mutex, block, allocs)
- Symbol resolution against a binary (rely on what's in the profile)
- Persistent config / themes
- Tests as a deliverable (smoke tests only)

## In-scope (v0.1)

- Load CPU + heap profiles from local files
- Pull profiles from a live `/debug/pprof` endpoint
- Three views: **Top**, **Tree**, **Flame**
- Switch active sample type within a profile (e.g. `inuse_space` ↔ `alloc_space`)

---

## Architecture

```
main
 └─ tui.Model (Bubble Tea)
     ├─ profile.Profile        ← google/pprof/profile wrapper + aggregations
     ├─ views/top              ← bubbles/table
     ├─ views/tree             ← custom (collapsible)
     ├─ views/flame            ← custom (canvas render)
     └─ source/                ← local file | http poller
```

Single binary. No daemon. No persisted state in v0.1.

### Package layout

```
cmd/lazypprof/             # main, flag parsing
internal/profile/          # load, aggregate (Top/Tree/Flame inputs)
internal/source/           # file source, http poller
internal/tui/              # Bubble Tea model + key routing
internal/tui/top/          # Top view
internal/tui/tree/         # Tree view
internal/tui/flame/        # Flame view
```

(Currently flatter — to be migrated when the views land.)

---

## Data model

`profile.Profile` wraps `*pprof.Profile` plus the **active sample type** (a single string, e.g. `cpu`, `inuse_space`).

All view-specific aggregations are derived on demand from the active sample type. No cached intermediate state in v0.1 — re-aggregate on switch. Profiles are small enough (<10 MB typical) that this is fine.

### Aggregations

| View  | Shape                                                |
| ----- | ---------------------------------------------------- |
| Top   | `[]FunctionStat{Name, File, Flat, Cum}`              |
| Tree  | rooted call graph: `Node{Func, Self, Cum, Children}` |
| Flame | same as Tree, rendered as nested rectangles          |

Tree and Flame share the same underlying call-graph build. Build once per (profile, sample-type) tuple; invalidate on switch.

---

## Sources

### File source

Open path → `pprof.Parse` → done. Supports gzipped and raw.

### HTTP source (live mode)

Poll `GET <base>/debug/pprof/profile?seconds=N` (CPU) or `/debug/pprof/heap` (heap) on a configurable interval (default: 10s for CPU, 5s for heap). Replace the active `Profile` on successful fetch; keep showing the previous one on transient failure.

CLI:

```
lazypprof http://localhost:6060
lazypprof http://localhost:6060/debug/pprof/heap
```

Heuristic: if arg starts with `http://` or `https://`, treat as live; else as file.

Rendering policy: don't blow away the user's selection/focus on refresh. Each view is responsible for preserving cursor across data updates (match by function name).

---

## Views

### Top

`bubbles/table` with columns: `Flat | Flat% | Cum | Cum% | Function`.

- Sorted by Cum descending (default)
- Up to 200 rows (configurable later)
- Function names truncated to fit column with `…`
- Percentages computed against `sum(Flat)` for the active sample type

Open question: support sort-by-flat with a keybind? Probably yes, costs nothing.

### Tree

Collapsible call tree, callees-of-roots:

- Roots = functions with no caller (entrypoints) **or** top-K by Cum if there are too many roots
- Each node shows: `[+/-] Cum (Cum%)  Function`
- `→` / `enter` expand, `←` collapse, `space` toggle
- `0` collapse all, `*` expand current subtree

Implementation: depth-first build of unique (function, parent) edges from sample stacks, summing values. Bubble Tea viewport for scrolling.

### Flame

Classic flame graph, terminal-rendered.

**Layout:**

- X axis: cumulative samples (proportional width)
- Y axis: stack depth (root at bottom — same as Brendan Gregg's convention)
- Each cell: one terminal cell wide minimum

**Rendering:**

- Build the call graph (shared with Tree)
- For each row, lay out children left-to-right proportional to Cum
- Frames narrower than `minLabelWidth` (default 4 cells) collapse to a colored sliver; sibling slivers merge into a `…` block on hover/focus
- Color: hash function name → 256-color warm palette (deterministic across renders)

**Navigation:**

- Arrow keys move a focus cursor between adjacent frames
- `enter` zooms — focused frame becomes new root
- `backspace` zooms out one level
- `0` zoom to original root
- Status line shows full function name + Flat/Cum for focused frame

**Hard parts (call out):**

- Label truncation: `pkg/long/path.(*Type).Method` → progressively shorter forms as cell width shrinks
- Focus persistence across live-mode refresh (match by function name + parent path)

---

## Keymap (global)

| Key   | Action                      |
| ----- | --------------------------- |
| `tab` | Cycle Top → Tree → Flame    |
| `s`   | Cycle sample type           |
| `/`   | Filter (regex on func name) |
| `?`   | Help overlay                |
| `q`   | Quit                        |

View-local keys documented per view.

`/` filter scope: applies to Top rows, Tree visibility (hide non-matching subtrees with no matching descendants), Flame highlight (matching frames stay colored, others desaturate).

---

## Dependencies

- `github.com/charmbracelet/bubbletea` — runtime
- `github.com/charmbracelet/bubbles` — table, viewport, textinput
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/google/pprof/profile` — parsing

No others in v0.1. Resist the urge.

---

## Open questions

1. **Tree roots when there are many entrypoints** — top-K by Cum, or show a "roots" picker first? Lean: top-K with a key to expand the rest.
2. **Live mode auth** — do we need to support bearer tokens / basic auth on the `/debug/pprof` endpoint? Common in production. Probably defer to v0.2 unless trivial.
3. **Sample-type default for heap** — currently picks the last one (`inuse_space` for typical heap profiles). Verify this holds across Go versions.
4. **Flame graph color scheme** — warm palette by default. Add a `--cold` flag or theme later, not v0.1.
5. **Window too narrow** — flame graph degrades badly under ~80 cols. Show a "resize me" placeholder, or just render and let it be ugly?

---

## Milestones

- **M1 — Skeleton (done):** Bubble Tea wired, Top view working off file source
- **M2 — Tree view:** Call graph build + collapsible tree
- **M3 — Flame view:** Layout + render + focus/zoom
- **M4 — Live mode:** HTTP poller + state preservation
- **M5 — Polish:** Filter bar, help overlay, README + demo gif, release
