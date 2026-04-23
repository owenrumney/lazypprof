package profile

import (
	"sort"

	pp "github.com/google/pprof/profile"
)

// Node is one frame in the call graph. Children are callees.
type Node struct {
	Func     string
	File     string
	Self     int64 // value from samples where this function is the leaf
	Cum      int64 // value from all samples passing through this function
	Children []*Node
}

// CallGraph builds a rooted call tree from the profile's samples for the
// active sample type. Stacks are walked root-to-leaf; each unique
// (function, parent) path gets its own Node. If there are more than maxRoots
// root nodes, only the top maxRoots by Cum are kept.
func (p *Profile) CallGraph(maxRoots int) []*Node {
	idx := p.sampleIndex()
	if idx < 0 {
		return nil
	}

	// Synthetic root to collect all stacks under one parent.
	root := &Node{}

	for _, s := range p.Raw.Sample {
		if len(s.Location) == 0 {
			continue
		}
		v := s.Value[idx]

		// pprof locations are leaf-first; walk in reverse for root-to-leaf.
		funcs := stackFunctions(s)
		if len(funcs) == 0 {
			continue
		}

		cur := root
		for i, fn := range funcs {
			child := findChild(cur, fn.name)
			if child == nil {
				child = &Node{Func: fn.name, File: fn.file}
				cur.Children = append(cur.Children, child)
			}
			child.Cum += v
			if i == len(funcs)-1 {
				child.Self += v
			}
			cur = child
		}
	}

	roots := root.Children
	sortNodes(roots)
	for _, r := range roots {
		sortTree(r)
	}

	if maxRoots > 0 && len(roots) > maxRoots {
		roots = roots[:maxRoots]
	}

	return roots
}

// TotalValue returns the sum of all sample values for the active sample type.
func (p *Profile) TotalValue() int64 {
	idx := p.sampleIndex()
	if idx < 0 {
		return 0
	}
	var total int64
	for _, s := range p.Raw.Sample {
		total += s.Value[idx]
	}
	if total == 0 {
		total = 1
	}
	return total
}

type funcInfo struct {
	name string
	file string
}

func stackFunctions(s *pp.Sample) []funcInfo {
	var out []funcInfo
	// Locations are leaf-first in pprof; reverse to get root-first.
	for i := len(s.Location) - 1; i >= 0; i-- {
		loc := s.Location[i]
		for _, line := range loc.Line {
			if line.Function == nil {
				continue
			}
			out = append(out, funcInfo{
				name: line.Function.Name,
				file: line.Function.Filename,
			})
		}
	}
	return out
}

func findChild(parent *Node, name string) *Node {
	for _, c := range parent.Children {
		if c.Func == name {
			return c
		}
	}
	return nil
}

func sortNodes(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Cum > nodes[j].Cum
	})
}

func sortTree(n *Node) {
	sortNodes(n.Children)
	for _, c := range n.Children {
		sortTree(c)
	}
}
