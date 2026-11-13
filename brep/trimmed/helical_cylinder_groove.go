package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// GrooveCylinderWithHelicalPolygon subtracts a blind-ended constant-lead
// groove from cylindrical stock. The convex meridian must meet the stock
// radius in precisely one side, with all remaining vertices strictly inside.
// The open groove must stay inside one cylinder chart period and clear both
// original caps. The result retains the original cylinder, its cap supports,
// both helical rims and the exact analytic cutter faces inside the stock.
// Seam-crossing grooves and lead-in or runout features remain unresolved.
func GrooveCylinderWithHelicalPolygon(ctx context.Context, frame geom.Frame, stockRadius, height float64,
	profile []geom.Vec2, lead, start, end float64, hand geom.HelixHandedness) (Solid, error) {
	if ctx == nil || !finite(stockRadius) || stockRadius <= 0 || !finite(height) || height <= 0 ||
		!finite(lead) || lead <= 0 || !finite(start) || !finite(end) || end <= start || end-start >= 1 ||
		hand != geom.LeftHanded && hand != geom.RightHanded || len(profile) > 32 {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	p, _, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	n := len(p)
	outer := -1
	for i, a := range p {
		b, c := p[(i+1)%n], p[(i+2)%n]
		sign, e := geom.Orient2D(a, b, c)
		if e != nil || sign <= 0 || a.X <= 0 || a.X > stockRadius {
			return Solid{}, geom.ErrDomain
		}
		if a.X == stockRadius && b.X == stockRadius {
			if outer >= 0 {
				return Solid{}, geom.ErrDomain
			}
			outer = i
		}
		z0 := new(big.Rat).Add(ratInset(a.Y), new(big.Rat).Mul(ratInset(lead), ratInset(start)))
		z1 := new(big.Rat).Add(ratInset(a.Y), new(big.Rat).Mul(ratInset(lead), ratInset(end)))
		if z0.Sign() <= 0 || z1.Cmp(ratInset(height)) >= 0 {
			return Solid{}, geom.ErrConvergence
		}
	}
	if outer < 0 {
		return Solid{}, geom.ErrDomain
	}
	for i, a := range p {
		if i != outer && i != (outer+1)%n && a.X == stockRadius {
			return Solid{}, geom.ErrDomain
		}
	}
	phaseStart, phaseEnd := float64(hand)*start, float64(hand)*end
	period := math.Floor(min(phaseStart, phaseEnd))
	if math.Abs(period) > 1<<20 || min(phaseStart, phaseEnd) <= period || max(phaseStart, phaseEnd) >= period+1 {
		return Solid{}, geom.ErrConvergence
	}
	stock, e := parametric.Cylinder(frame, stockRadius, height)
	if e != nil {
		return Solid{}, e
	}
	base, e := FromParametric(stock)
	if e != nil {
		return Solid{}, e
	}
	tool, e := SweepHelicalPolygon(ctx, frame, p, lead, start, end, hand)
	if e != nil {
		return Solid{}, e
	}
	v, edges, faces := base.Vertices(), base.Edges(), base.Faces()
	vertexBase, edgeBase := len(v), len(edges)
	v = append(v, tool.vertices...)
	for _, e := range tool.edges {
		e.Start += vertexBase
		e.End += vertexBase
		edges = append(edges, e)
	}
	// The removed outer cutter side bounds the opening on the stock wall.
	// Its model edges already belong to the retained neighboring faces.
	// The exact shared model edges are authoritative for both owners.
	opening := tool.faces[outer+2]
	project := func(uv geom.Vec2) geom.Vec2 {
		var z float64
		if uv.Y == 0 {
			z = p[outer].Y
		} else {
			z = p[(outer+1)%n].Y
		}
		return geom.Vec2{X: 2 * math.Pi * (float64(hand)*uv.X - period), Y: lead*uv.X + z}
	}
	var hole []Use
	for i := range opening.Outer {
		k := i
		if hand == geom.RightHanded {
			k = len(opening.Outer) - 1 - i
		}
		u := opening.Outer[k]
		if hand == geom.RightHanded {
			u.UVStart, u.UVEnd, u.Reversed = u.UVEnd, u.UVStart, !u.Reversed
		}
		u.Edge += edgeBase
		u.UVStart, u.UVEnd = project(u.UVStart), project(u.UVEnd)
		if u.UVStart.X <= 0 || u.UVStart.X >= 2*math.Pi || u.UVStart.Y <= 0 || u.UVStart.Y >= height {
			return Solid{}, geom.ErrUnrepresentable
		}
		hole = append(hole, u)
	}
	faces[0].Holes = append(faces[0].Holes, hole)
	for i, f := range tool.faces {
		if i == outer+2 {
			continue
		}
		f = cloneFace(f)
		f.Reversed = !f.Reversed
		for i := range f.Outer {
			f.Outer[i].Edge += edgeBase
		}
		faces = append(faces, f)
	}
	return NewSolidContext(ctx, v, edges, faces, max(base.tolerance, tool.tolerance))
}
