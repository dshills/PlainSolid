package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// CoalesceCardinalPlanar removes representation seams between connected
// coplanar cardinal faces. The validated source boundary is preserved exactly;
// disjoint supports remain separate and interior wires retain their orientation.
func CoalesceCardinalPlanar(ctx context.Context, base brep.Solid, tol float64) (Solid, error) {
	if ctx == nil {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if len(base.Faces()) > 4096 || len(base.Edges()) > 16384 {
		return Solid{}, geom.ErrDomain
	}
	s, e := FromPlanar(base, tol)
	if e != nil {
		return Solid{}, e
	}
	if len(s.faces) > 4096 || len(s.edges) > 16384 {
		return Solid{}, geom.ErrDomain
	}
	keys := make([][4]float64, len(s.faces))
	owners := make([][]int, len(s.edges))
	for i, f := range s.faces {
		p := f.Surface.(geom.PlaneSurface)
		n := p.Frame().ZAxis()
		if (n.X != 0 && n.X != 1 && n.X != -1) || (n.Y != 0 && n.Y != 1 && n.Y != -1) || (n.Z != 0 && n.Z != 1 && n.Z != -1) || n.Dot(n) != 1 {
			return Solid{}, geom.ErrDomain
		}
		keys[i] = [4]float64{n.X, n.Y, n.Z, n.Dot(p.Frame().Origin())}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				owners[u.Edge] = append(owners[u.Edge], i)
			}
		}
	}
	seen := make([]bool, len(s.faces))
	var output []Face
	for first := range s.faces {
		if seen[first] {
			continue
		}
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		queue := []int{first}
		seen[first] = true
		counts := map[int]int{}
		var uses []Use
		for k := 0; k < len(queue); k++ {
			id := queue[k]
			for _, loop := range append([][]Use{s.faces[id].Outer}, s.faces[id].Holes...) {
				for _, u := range loop {
					uses = append(uses, u)
					counts[u.Edge]++
					for _, other := range owners[u.Edge] {
						if !seen[other] && keys[other] == keys[first] {
							seen[other] = true
							queue = append(queue, other)
						}
					}
				}
			}
		}
		plane := s.faces[first].Surface.(geom.PlaneSurface)
		var boundary []Use
		starts := map[int]int{}
		for _, u := range uses {
			if counts[u.Edge] == 2 {
				continue
			}
			if counts[u.Edge] != 1 {
				return Solid{}, geom.ErrDomain
			}
			ed := s.edges[u.Edge]
			a := ed.Start
			if u.Reversed {
				a = ed.End
			}
			if _, ok := starts[a]; ok {
				return Solid{}, geom.ErrDomain
			}
			starts[a] = len(boundary)
			boundary = append(boundary, u)
		}
		visited := make([]bool, len(boundary))
		out := Face{Surface: plane}
		for first := range boundary {
			if visited[first] {
				continue
			}
			var loop []Use
			at := first
			for {
				if visited[at] {
					if at != first {
						return Solid{}, geom.ErrDomain
					}
					break
				}
				visited[at] = true
				u := boundary[at]
				ed := s.edges[u.Edge]
				a, b := ed.Start, ed.End
				if u.Reversed {
					a, b = b, a
				}
				p, q := plane.Frame().ToLocal(s.vertices[a]), plane.Frame().ToLocal(s.vertices[b])
				u.UVStart, u.UVEnd = geom.Vec2{X: p.X, Y: p.Y}, geom.Vec2{X: q.X, Y: q.Y}
				u.Curve = nil
				loop = append(loop, u)
				next, ok := starts[b]
				if !ok {
					return Solid{}, geom.ErrDomain
				}
				at = next
			}
			area := new(big.Rat)
			for _, u := range loop {
				a, b := u.UVStart, u.UVEnd
				area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(ratInset(a.X), ratInset(b.Y)), new(big.Rat).Mul(ratInset(a.Y), ratInset(b.X))))
			}
			if area.Sign() > 0 {
				if out.Outer != nil {
					return Solid{}, geom.ErrDomain
				}
				out.Outer = loop
			} else if area.Sign() < 0 {
				out.Holes = append(out.Holes, loop)
			} else {
				return Solid{}, geom.ErrDomain
			}
		}
		if out.Outer == nil {
			return Solid{}, geom.ErrDomain
		}
		output = append(output, out)
	}
	return compactSteppedTopology(ctx, s.vertices, s.edges, output, tol)
}
