package profile

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	pp "github.com/google/pprof/profile"
)

// Goroutine represents a single goroutine parsed from debug=2 output.
type Goroutine struct {
	ID    int
	State string
	Stack []StackFrame
}

// StackFrame is one frame in a goroutine's stack.
type StackFrame struct {
	Func string
	File string
	Line int
}

// GoroutineGroup is a set of goroutines sharing the same state.
type GoroutineGroup struct {
	State      string
	Count      int
	Goroutines []Goroutine
}

// ParseGoroutineText parses the output of /debug/pprof/goroutine?debug=2
// into individual goroutine records.
func ParseGoroutineText(data []byte) ([]Goroutine, error) {
	var goroutines []Goroutine
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "goroutine ") {
			continue
		}

		g, err := parseGoroutineHeader(line)
		if err != nil {
			continue
		}

		// Read stack frames: alternating function line / file:line line.
		for scanner.Scan() {
			funcLine := strings.TrimSpace(scanner.Text())
			if funcLine == "" {
				break // blank line = end of this goroutine
			}

			// Next line is file:line.
			if !scanner.Scan() {
				break
			}
			fileLine := strings.TrimSpace(scanner.Text())

			frame := parseFrame(funcLine, fileLine)
			g.Stack = append(g.Stack, frame)
		}

		goroutines = append(goroutines, g)
	}

	return goroutines, scanner.Err()
}

// parseGoroutineHeader parses "goroutine 18 [IO wait, 5 minutes]:"
func parseGoroutineHeader(line string) (Goroutine, error) {
	// Strip "goroutine " prefix and trailing ":"
	line = strings.TrimPrefix(line, "goroutine ")
	line = strings.TrimSuffix(line, ":")

	// Split into "18 [IO wait, 5 minutes]" or "18 [running]"
	bracketIdx := strings.Index(line, "[")
	if bracketIdx < 0 {
		return Goroutine{}, fmt.Errorf("no state bracket in: %s", line)
	}

	idStr := strings.TrimSpace(line[:bracketIdx])
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return Goroutine{}, fmt.Errorf("bad goroutine id %q: %w", idStr, err)
	}

	stateStr := line[bracketIdx+1:]
	stateStr = strings.TrimSuffix(stateStr, "]")

	// State may include a duration: "IO wait, 5 minutes" — we keep just the state.
	state := stateStr
	if commaIdx := strings.Index(stateStr, ","); commaIdx >= 0 {
		state = stateStr[:commaIdx]
	}
	state = strings.TrimSpace(state)

	return Goroutine{ID: id, State: state}, nil
}

// parseFrame parses a function line and file:line line into a StackFrame.
func parseFrame(funcLine, fileLine string) StackFrame {
	// funcLine: "runtime.gopark(0x0?, 0x0?, ...)"
	// Strip args.
	funcName := funcLine
	if parenIdx := strings.Index(funcLine, "("); parenIdx >= 0 {
		funcName = funcLine[:parenIdx]
	}

	// fileLine: "/path/to/file.go:381 +0xb4"
	file := fileLine
	line := 0
	// Strip offset like " +0xb4"
	if plusIdx := strings.LastIndex(fileLine, " +0x"); plusIdx >= 0 {
		file = fileLine[:plusIdx]
	}
	// Split file:line
	if colonIdx := strings.LastIndex(file, ":"); colonIdx >= 0 {
		if n, err := strconv.Atoi(file[colonIdx+1:]); err == nil {
			line = n
			file = file[:colonIdx]
		}
	}

	return StackFrame{Func: funcName, File: file, Line: line}
}

// GroupByState groups goroutines by their state, sorted by count descending.
func GroupByState(gs []Goroutine) []GoroutineGroup {
	byState := make(map[string][]Goroutine)
	for _, g := range gs {
		byState[g.State] = append(byState[g.State], g)
	}

	groups := make([]GoroutineGroup, 0, len(byState))
	for state, goroutines := range byState {
		groups = append(groups, GoroutineGroup{
			State:      state,
			Count:      len(goroutines),
			Goroutines: goroutines,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Count > groups[j].Count
	})
	return groups
}

// StackGroup is a set of goroutines sharing the same stack trace.
type StackGroup struct {
	Stack []StackFrame
	Count int
	IDs   []int // goroutine IDs in this group
}

// GroupByStack groups goroutines by unique stack trace, sorted by count descending.
func GroupByStack(gs []Goroutine) []StackGroup {
	type key string
	byStack := make(map[key]*StackGroup)
	var order []key

	for _, g := range gs {
		k := key(stackKey(g.Stack))
		sg, ok := byStack[k]
		if !ok {
			sg = &StackGroup{Stack: g.Stack}
			byStack[k] = sg
			order = append(order, k)
		}
		sg.Count++
		sg.IDs = append(sg.IDs, g.ID)
	}

	groups := make([]StackGroup, 0, len(byStack))
	for _, k := range order {
		groups = append(groups, *byStack[k])
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Count > groups[j].Count
	})
	return groups
}

func stackKey(frames []StackFrame) string {
	var b strings.Builder
	for _, f := range frames {
		b.WriteString(f.Func)
		b.WriteByte('\n')
	}
	return b.String()
}

// GoroutineStates returns the unique states, sorted by frequency descending.
func GoroutineStates(gs []Goroutine) []string {
	groups := GroupByState(gs)
	states := make([]string, len(groups))
	for i, g := range groups {
		states[i] = g.State
	}
	return states
}

// BuildSyntheticProfile constructs a *pp.Profile from parsed goroutines
// so the standard Top/Tree/Flame views can render goroutine data.
func BuildSyntheticProfile(gs []Goroutine) *pp.Profile {
	prof := &pp.Profile{
		SampleType: []*pp.ValueType{
			{Type: "goroutines", Unit: "count"},
		},
	}

	funcMap := make(map[string]*pp.Function)
	locMap := make(map[string]*pp.Location)
	var nextFuncID, nextLocID uint64

	getFunc := func(name, file string) *pp.Function {
		key := name + "\x00" + file
		if f, ok := funcMap[key]; ok {
			return f
		}
		nextFuncID++
		f := &pp.Function{ID: nextFuncID, Name: name, Filename: file}
		funcMap[key] = f
		prof.Function = append(prof.Function, f)
		return f
	}

	getLoc := func(fn *pp.Function, line int) *pp.Location {
		key := fmt.Sprintf("%d:%d", fn.ID, line)
		if l, ok := locMap[key]; ok {
			return l
		}
		nextLocID++
		l := &pp.Location{
			ID:   nextLocID,
			Line: []pp.Line{{Function: fn, Line: int64(line)}},
		}
		locMap[key] = l
		prof.Location = append(prof.Location, l)
		return l
	}

	for _, g := range gs {
		if len(g.Stack) == 0 {
			continue
		}
		// Build locations leaf-first (pprof convention).
		locs := make([]*pp.Location, len(g.Stack))
		for i, frame := range g.Stack {
			fn := getFunc(frame.Func, frame.File)
			locs[i] = getLoc(fn, frame.Line)
		}
		prof.Sample = append(prof.Sample, &pp.Sample{
			Location: locs,
			Value:    []int64{1},
		})
	}

	return prof
}
