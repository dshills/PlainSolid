package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Collapse only exact cardinal collinear vertices, preserving all inner wires.
func collapseTreatmentEdges(ctx context.Context, s Solid, faces []Face) (Solid, error) {
	edges := []parametric.Edge{}
	lookup := map[[2]int]int{}
	for i := range faces {
		for loopIndex, loop := range append([][]Use{faces[i].Outer}, faces[i].Holes...) {
			ids := make([]int, len(loop))
			for j, u := range loop {
				ed := s.edges[u.Edge]
				ids[j] = ed.Start
				if u.Reversed {
					ids[j] = ed.End
				}
			}
			for changed := true; changed; {
				changed = false
				for j := range ids {
					a, b, c := s.vertices[ids[(j+len(ids)-1)%len(ids)]], s.vertices[ids[j]], s.vertices[ids[(j+1)%len(ids)]]
					axis := -1
					aa, bb, cc := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}, [3]float64{c.X, c.Y, c.Z}
					same := 0
					for k := 0; k < 3; k++ {
						if aa[k] == bb[k] && bb[k] == cc[k] {
							same++
						} else {
							axis = k
						}
					}
					if same == 2 && ((aa[axis] < bb[axis] && bb[axis] < cc[axis]) || (aa[axis] > bb[axis] && bb[axis] > cc[axis])) {
						ids = append(ids[:j], ids[j+1:]...)
						changed = true
						break
					}
				}
				if len(ids) < 3 {
					return Solid{}, geom.ErrDomain
				}
			}
			plane := faces[i].Surface.(geom.PlaneSurface)
			var out []Use
			for j, a := range ids {
				b := ids[(j+1)%len(ids)]
				key := [2]int{a, b}
				if a > b {
					key = [2]int{b, a}
				}
				id, ok := lookup[key]
				if !ok {
					id = len(edges)
					lookup[key] = id
					d := s.vertices[b].Sub(s.vertices[a])
					line, e := geom.NewLine(s.vertices[a], d)
					if e != nil {
						return Solid{}, e
					}
					edges = append(edges, parametric.Edge{Start: a, End: b, Curve: line, Range: parametric.Interval{Max: d.Length()}})
				}
				p, q := plane.Frame().ToLocal(s.vertices[a]), plane.Frame().ToLocal(s.vertices[b])
				out = append(out, Use{Edge: id, Reversed: edges[id].Start != a, UVStart: geom.Vec2{X: p.X, Y: p.Y}, UVEnd: geom.Vec2{X: q.X, Y: q.Y}})
			}
			if loopIndex == 0 {
				faces[i].Outer = out
			} else {
				faces[i].Holes[loopIndex-1] = out
			}
		}
	}
	return compactSteppedTopology(ctx, s.Vertices(), edges, faces, s.tolerance)
}
