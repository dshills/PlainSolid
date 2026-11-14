package document

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// SideOpenDualDiskRecipe defines two authored circles in a right-handed
// signed coordinate frame. Local z=0 is fixed to the tool entry cap.
// Through follows the current stock height; for a blind cut Depth must lie
// strictly between the current caps. Tolerance zero selects the native default.
type SideOpenDualDiskRecipe struct {
	Origin, U, Inward geom.Vec3
	Discs             [2]trimmed.ExtrusionCircle
	Depth, Tolerance  float64
	Through           bool
}

func evaluateSideOpenDualDisk(ctx context.Context, stock brep.Solid, p *SideOpenDualDiskRecipe) (trimmed.Solid, error) {
	return defineSideOpenDualDisk(ctx, stock, p, nil)
}

func defineSideOpenDualDisk(ctx context.Context, stock brep.Solid, p *SideOpenDualDiskRecipe, out *step.SideOpenDualDiskDescriptor) (trimmed.Solid, error) {
	if p == nil || len(stock.Vertices()) != 8 || len(stock.Edges()) != 12 || len(stock.Faces()) != 6 || p.Through && p.Depth != 0 {
		return trimmed.Solid{}, fmt.Errorf("exact dual disk cut needs rectangular stock and an authored cap recipe")
	}
	cardinal := func(a geom.Vec3) bool {
		for _, v := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
			if a == v || a == v.Scale(-1) {
				return true
			}
		}
		return false
	}
	if !cardinal(p.U) || !cardinal(p.Inward) || p.U.Dot(p.Inward) != 0 {
		return trimmed.Solid{}, geom.ErrDomain
	}
	f, e := geom.NewFrame(p.Origin, p.U, p.Inward)
	if e != nil {
		return trimmed.Solid{}, e
	}
	axes := [3]geom.Vec3{f.XAxis(), f.YAxis(), f.ZAxis()}
	var vertices []geom.Vec3
	for _, v := range stock.Vertices() {
		var xyz [3]float64
		for i, a := range axes {
			x, o := new(big.Rat).SetFloat64(v.Dot(a)), new(big.Rat).SetFloat64(p.Origin.Dot(a))
			var exact bool
			xyz[i], exact = x.Sub(x, o).Float64()
			if !exact {
				return trimmed.Solid{}, geom.ErrUnrepresentable
			}
		}
		vertices = append(vertices, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
	}
	bounds, e := geom.Bounds(vertices)
	if e != nil || bounds.Min.Z != 0 || bounds.Max.Z <= 0 {
		return trimmed.Solid{}, fmt.Errorf("dual disk entry cap no longer belongs to stock: %w", geom.ErrDomain)
	}
	l, h := bounds.Min, bounds.Max
	corners := make(map[geom.Vec3]bool, 8)
	for _, v := range vertices {
		if corners[v] || v.X != l.X && v.X != h.X || v.Y != l.Y && v.Y != h.Y || v.Z != l.Z && v.Z != h.Z {
			return trimmed.Solid{}, geom.ErrDomain
		}
		corners[v] = true
	}
	edges := stock.Edges()
	var walls [6]bool
	for _, face := range stock.Faces() {
		if len(face.Boundary) != 4 || len(face.Holes) != 0 {
			return trimmed.Solid{}, geom.ErrDomain
		}
		var ring []geom.Vec3
		for _, use := range face.Boundary {
			edge := edges[use.Edge]
			a, b := vertices[edge.Start], vertices[edge.End]
			if (a.X != b.X && a.Y != b.Y) || (a.X != b.X && a.Z != b.Z) || (a.Y != b.Y && a.Z != b.Z) {
				return trimmed.Solid{}, geom.ErrDomain
			}
			ring = append(ring, a)
			if use.Reversed {
				ring[len(ring)-1] = b
			}
		}
		b, e := geom.Bounds(ring)
		if e != nil {
			return trimmed.Solid{}, e
		}
		found := -1
		for i, pair := range [3][2]float64{{b.Min.X, b.Max.X}, {b.Min.Y, b.Max.Y}, {b.Min.Z, b.Max.Z}} {
			if pair[0] == pair[1] {
				minima := [3]float64{l.X, l.Y, l.Z}
				found = 2 * i
				if pair[0] != minima[i] {
					found++
				}
			}
		}
		if found < 0 || walls[found] {
			return trimmed.Solid{}, geom.ErrDomain
		}
		walls[found] = true
	}
	depth, tolerance := p.Depth, p.Tolerance
	if p.Through {
		depth = h.Z
	} else if depth >= h.Z {
		return trimmed.Solid{}, fmt.Errorf("blind dual disk cut loses its floor: %w", geom.ErrDomain)
	}
	if tolerance == 0 {
		tolerance = 1e-9
	}
	d := step.SideOpenDualDiskDescriptor{Frame: f, Min: geom.Vec2{X: l.X, Y: l.Y}, Max: geom.Vec2{X: h.X, Y: h.Y},
		Discs: p.Discs, Height: h.Z, Depth: depth, Tolerance: tolerance}
	var solid trimmed.Solid
	if d.Discs[0].Radius == d.Discs[1].Radius {
		cut, e := trimmed.CutSideOpenDualDisks(ctx, f, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
		if e != nil {
			return trimmed.Solid{}, e
		}
		solid = cut.Solid
	} else {
		cut, e := trimmed.CutUnequalSideOpenDualDisks(ctx, f, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
		if e != nil {
			return trimmed.Solid{}, e
		}
		solid = cut.Solid
	}
	if out != nil {
		*out = d
	}
	return solid, nil
}
