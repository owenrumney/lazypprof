package profile

import "sort"

// LineStat is the per-source-line aggregate used by the Source view.
type LineStat struct {
	File     string
	Line     int
	Function string
	Flat     int64
	Cum      int64
}

// SourceLines aggregates flat and cumulative values by source line. If
// functionName is non-empty, only lines for that function are returned.
func (p *Profile) SourceLines(functionName string) []LineStat {
	idx := p.sampleIndex()
	if idx < 0 {
		return nil
	}

	type key struct {
		file     string
		line     int
		function string
	}

	stats := make(map[key]*LineStat)
	for _, s := range p.Raw.Sample {
		if len(s.Location) == 0 {
			continue
		}
		v := s.Value[idx]
		seen := make(map[key]bool)

		for locIdx, loc := range s.Location {
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				fn := line.Function.Name
				if functionName != "" && fn != functionName {
					continue
				}
				k := key{file: line.Function.Filename, line: int(line.Line), function: fn}
				st := stats[k]
				if st == nil {
					st = &LineStat{File: k.file, Line: k.line, Function: k.function}
					stats[k] = st
				}
				if !seen[k] {
					st.Cum += v
					seen[k] = true
				}
				if locIdx == 0 {
					st.Flat += v
				}
			}
		}
	}

	out := make([]LineStat, 0, len(stats))
	for _, st := range stats {
		out = append(out, *st)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cum != out[j].Cum {
			return out[i].Cum > out[j].Cum
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
