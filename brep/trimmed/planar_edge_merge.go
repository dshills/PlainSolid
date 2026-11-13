package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Merge only coplanar cardinal source faces, removing internal triangulation
// edges. Disconnected outer loops remain separate; holes and ambiguous vertex
// junctions are rejected in this local edge-treatment path.
func mergeEdgeTreatmentPlanes(s Solid) ([]Face, error) {
	if len(s.faces) > 4096 || len(s.edges) > 16384 {
		return nil, geom.ErrDomain
	}
	type group struct {
		plane  geom.PlaneSurface
		uses   []Use
		counts map[int]int
	}
	groups := []group{}
	ids := map[[4]float64]int{}
	for _, f := range s.faces {
		p, ok := f.Surface.(geom.PlaneSurface)
		if !ok || f.Reversed || len(f.Holes) != 0 {
			return nil, geom.ErrDomain
		}
		n := p.Frame().ZAxis()
		if (n.X != 0 && n.X != 1 && n.X != -1) || (n.Y != 0 && n.Y != 1 && n.Y != -1) || (n.Z != 0 && n.Z != 1 && n.Z != -1) || n.Dot(n) != 1 {
			return nil, geom.ErrDomain
		}
		key := [4]float64{n.X, n.Y, n.Z, n.Dot(p.Frame().Origin())}
		id, ok := ids[key]
		if !ok {
			id = len(groups)
			ids[key] = id
			groups = append(groups, group{plane: p, counts: map[int]int{}})
		}
		for _, u := range f.Outer {
			groups[id].uses = append(groups[id].uses, u)
			groups[id].counts[u.Edge]++
		}
	}
	var faces []Face
	for _, g := range groups {
		var boundary []Use
		starts := map[int]int{}
		for _, u := range g.uses {
			if g.counts[u.Edge] == 2 {
				continue
			}
			if g.counts[u.Edge] != 1 {
				return nil, geom.ErrDomain
			}
			ed := s.edges[u.Edge]
			a := ed.Start
			if u.Reversed {
				a = ed.End
			}
			if _, ok := starts[a]; ok {
				return nil, geom.ErrDomain
			}
			starts[a] = len(boundary)
			boundary = append(boundary, u)
		}
		seen := make([]bool, len(boundary))
		for first := range boundary {
			if seen[first] {
				continue
			}
			var loop []Use
			index := first
			for {
				if seen[index] {
					if index != first {
						return nil, geom.ErrDomain
					}
					break
				}
				seen[index] = true
				u := boundary[index]
				ed := s.edges[u.Edge]
				a, b := ed.Start, ed.End
				if u.Reversed {
					a, b = b, a
				}
				p, q := g.plane.Frame().ToLocal(s.vertices[a]), g.plane.Frame().ToLocal(s.vertices[b])
				u.UVStart, u.UVEnd = geom.Vec2{X: p.X, Y: p.Y}, geom.Vec2{X: q.X, Y: q.Y}
				u.Curve = nil
				loop = append(loop, u)
				next, ok := starts[b]
				if !ok {
					return nil, geom.ErrDomain
				}
				index = next
			}
			if len(loop) < 3 {
				return nil, geom.ErrDomain
			}
			area := new(big.Rat)
			for _, u := range loop {
				a, b := u.UVStart, u.UVEnd
				area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(ratInset(a.X), ratInset(b.Y)), new(big.Rat).Mul(ratInset(a.Y), ratInset(b.X))))
			}
			if area.Sign() <= 0 {
				return nil, geom.ErrDomain
			}
			faces = append(faces, Face{Surface: g.plane, Outer: loop})
		}
	}
	return faces, nil
}

func collapseTreatmentEdges(ctx context.Context, s Solid, faces []Face) (Solid, error) {
	edges := []parametric.Edge{}
	lookup := map[[2]int]int{}
	for i := range faces {
		loop := faces[i].Outer
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
		faces[i].Outer = out
	}
	return compactSteppedTopology(ctx, s.Vertices(), edges, faces, s.tolerance)
}
