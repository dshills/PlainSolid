package trimmed

import "math"

type compactRefinement struct{ edges map[int]bool }

func (*compactRefinement) Error() string { return "trimmed: compact shared edges need refinement" }

// Opposing rims share one angular grid. Coupling only these curved edges
// avoids imposing a fillet's resolution on unrelated small bores.
func compactEdgeGroups(faces []Face, curved []bool) []int {
	parent := make([]int, len(curved))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		if parent[i] != i {
			parent[i] = root(parent[i])
		}
		return parent[i]
	}
	for _, f := range faces {
		_, axial := compactAxialStart(f)
		if !axial && !compactTorus(f) {
			continue
		}
		first := -1
		for _, u := range f.Outer {
			if !curved[u.Edge] {
				continue
			}
			if axial && !compactRim(u) {
				continue
			}
			if first < 0 {
				first = u.Edge
			} else {
				a, b := root(first), root(u.Edge)
				parent[b] = a
			}
		}
	}
	for i := range parent {
		parent[i] = root(i)
	}
	return parent
}
func (s Solid) compactEdgeCounts(levels []int, curved []bool) []int {
	counts := make([]int, len(curved))
	for i := range counts {
		counts[i] = 1
		if curved[i] {
			counts[i] = levels[i]
		}
	}
	for _, f := range s.faces {
		if _, ok := compactAxialStart(f); ok {
			for _, u := range f.Outer {
				if compactRim(u) {
					counts[u.Edge] = compactAngularCount(levels[u.Edge], math.Abs(u.UVEnd.X-u.UVStart.X))
				}
			}
		}
	}
	return counts
}
