package document

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/scene"
)

// SideOpenDualDiskRecipe defines two authored circles in a right-handed
// signed coordinate frame. Local z=0 is fixed to the tool entry cap.
// Through follows the current stock height; for a blind cut Depth must lie
// strictly between the current caps. Tolerance zero selects the native default.
// ExactWalls explicitly allows nonbinary homogeneous controls for an
// off-center unequal cut. This flag does not relax any endpoint or stock proof.
// OffsetCenters selects the separately certified common-center-below-side cut.
// UnequalCenterJoin selects different centers below the opening side and
// supplies their exact stock-local intersection as rational text. It is checked
// against the original circles on every evaluation. These policies are exclusive.
type SideOpenDualDiskRecipe struct {
	Origin, U, Inward geom.Vec3
	Discs             [2]trimmed.ExtrusionCircle
	Depth, Tolerance  float64
	Through           bool
	ExactWalls        bool       `json:",omitempty"`
	OffsetCenters     bool       `json:",omitempty"`
	UnequalCenterJoin *[2]string `json:",omitempty"`
}

func evaluateSideOpenDualDisk(ctx context.Context, stock brep.Solid, p *SideOpenDualDiskRecipe) (trimmed.Solid, error) {
	s, _, e := buildAuthoredSideOpenDualDisk(ctx, stock, p)
	return s, e
}

func buildAuthoredSideOpenDualDisk(ctx context.Context, stock brep.Solid, p *SideOpenDualDiskRecipe) (trimmed.Solid, scene.SideOpenDualDiskNormalBoundary, error) {
	d, e := authoredSideOpenDualDiskDescriptor(stock, p)
	if e != nil {
		return trimmed.Solid{}, nil, e
	}
	f, tolerance := d.Frame, d.Tolerance
	if p.UnequalCenterJoin != nil {
		join, e := p.exactJoin()
		if e != nil {
			return trimmed.Solid{}, nil, e
		}
		c, e := trimmed.CutUnequalCenterSideOpenDualDisks(ctx, f, d.Min, d.Max, d.Discs, join, d.Height, d.Depth, tolerance, trimvalidate.Options{})
		return c.Solid, c, e
	}
	if p.OffsetCenters {
		if d.Discs[0].Radius == d.Discs[1].Radius {
			return trimmed.Solid{}, nil, geom.ErrDomain
		}
		c, e := trimmed.CutOffsetSideOpenDualDisks(ctx, f, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
		return c.Solid, c, e
	}
	if d.Discs[0].Radius == d.Discs[1].Radius {
		if p.ExactWalls {
			return trimmed.Solid{}, nil, geom.ErrDomain
		}
		c, e := trimmed.CutSideOpenDualDisks(ctx, f, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
		return c.Solid, c, e
	}
	if externalSideDiskRadical(d) {
		c, e := trimmed.CutGeneralSideOpenDualDisks(ctx, f, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
		if p.ExactWalls && errors.Is(e, geom.ErrUnrepresentable) {
			c, e = trimmed.CutGeneralSideOpenDualDisksWithExactWalls(ctx, f, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
		}
		return c.Solid, c, e
	}
	if p.ExactWalls {
		return trimmed.Solid{}, nil, geom.ErrDomain
	}
	c, e := trimmed.CutUnequalSideOpenDualDisks(ctx, f, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
	return c.Solid, c, e
}

func authoredSideOpenDualDiskDescriptor(stock brep.Solid, p *SideOpenDualDiskRecipe) (step.SideOpenDualDiskDescriptor, error) {
	if p == nil || len(stock.Vertices()) != 8 || len(stock.Edges()) != 12 || len(stock.Faces()) != 6 || p.Through && p.Depth != 0 {
		return step.SideOpenDualDiskDescriptor{}, fmt.Errorf("exact dual disk cut needs rectangular stock and an authored cap recipe")
	}
	if p.ExactWalls && p.OffsetCenters || p.UnequalCenterJoin != nil && (p.ExactWalls || p.OffsetCenters) {
		return step.SideOpenDualDiskDescriptor{}, geom.ErrInvalid
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
		return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
	}
	f, e := geom.NewFrame(p.Origin, p.U, p.Inward)
	if e != nil {
		return step.SideOpenDualDiskDescriptor{}, e
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
				return step.SideOpenDualDiskDescriptor{}, geom.ErrUnrepresentable
			}
		}
		vertices = append(vertices, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
	}
	bounds, e := geom.Bounds(vertices)
	if e != nil || bounds.Min.Z != 0 || bounds.Max.Z <= 0 {
		return step.SideOpenDualDiskDescriptor{}, fmt.Errorf("dual disk entry cap no longer belongs to stock: %w", geom.ErrDomain)
	}
	l, h := bounds.Min, bounds.Max
	corners := make(map[geom.Vec3]bool, 8)
	for _, v := range vertices {
		if corners[v] || v.X != l.X && v.X != h.X || v.Y != l.Y && v.Y != h.Y || v.Z != l.Z && v.Z != h.Z {
			return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
		}
		corners[v] = true
	}
	edges := stock.Edges()
	var walls [6]bool
	for _, face := range stock.Faces() {
		if len(face.Boundary) != 4 || len(face.Holes) != 0 {
			return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
		}
		var ring []geom.Vec3
		for _, use := range face.Boundary {
			edge := edges[use.Edge]
			a, b := vertices[edge.Start], vertices[edge.End]
			if (a.X != b.X && a.Y != b.Y) || (a.X != b.X && a.Z != b.Z) || (a.Y != b.Y && a.Z != b.Z) {
				return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
			}
			ring = append(ring, a)
			if use.Reversed {
				ring[len(ring)-1] = b
			}
		}
		b, e := geom.Bounds(ring)
		if e != nil {
			return step.SideOpenDualDiskDescriptor{}, e
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
			return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
		}
		walls[found] = true
	}
	depth, tolerance := p.Depth, p.Tolerance
	if p.Through {
		depth = h.Z
	} else if depth >= h.Z {
		return step.SideOpenDualDiskDescriptor{}, fmt.Errorf("blind dual disk cut loses its floor: %w", geom.ErrDomain)
	}
	if tolerance == 0 {
		tolerance = 1e-9
	}
	d := step.SideOpenDualDiskDescriptor{Frame: f, Min: geom.Vec2{X: l.X, Y: l.Y}, Max: geom.Vec2{X: h.X, Y: h.Y},
		Discs: p.Discs, Height: h.Z, Depth: depth, Tolerance: tolerance}
	for _, c := range d.Discs {
		if !(geom.Vec3{X: c.Center.X, Y: c.Center.Y, Z: c.Radius}).IsFinite() || c.Radius <= 0 {
			return step.SideOpenDualDiskDescriptor{}, geom.ErrInvalid
		}
	}
	return d, nil
}

func (p *SideOpenDualDiskRecipe) exactJoin() (join [2]*big.Rat, err error) {
	if p == nil || p.UnequalCenterJoin == nil {
		return join, geom.ErrInvalid
	}
	for i, s := range p.UnequalCenterJoin {
		if len(s) == 0 || len(s) > 8192 {
			return join, geom.ErrInvalid
		}
		for _, c := range s {
			if (c < '0' || c > '9') && c != '-' && c != '/' {
				return join, geom.ErrInvalid
			}
		}
		q, ok := new(big.Rat).SetString(s)
		if !ok || q.RatString() != s {
			return join, geom.ErrInvalid
		}
		join[i] = q
	}
	return join, nil
}

func externalSideDiskRadical(d step.SideOpenDualDiskDescriptor) bool {
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	gap := new(big.Rat).Sub(q(d.Discs[1].Center.X), q(d.Discs[0].Center.X))
	delta := new(big.Rat).Sub(sq(q(d.Discs[0].Radius)), sq(q(d.Discs[1].Radius)))
	return delta.Abs(delta).Cmp(sq(gap)) > 0
}
