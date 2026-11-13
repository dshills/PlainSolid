package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

type extrusionQuarterArc struct {
	circle   ExtrusionCircle
	quadrant int
}

func (arc extrusionQuarterArc) wall(frame geom.Frame, lo, hi float64) (geom.Surface, [4]geom.Vec2, error) {
	c := arc.circle.Center
	f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{X: c.X, Y: c.Y}), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return nil, [4]geom.Vec2{}, e
	}
	s, e := geom.NewCylinderSurface(f, arc.circle.Radius)
	a, b := float64(arc.quadrant)*math.Pi/2, float64(arc.quadrant+1)*math.Pi/2
	return s, [4]geom.Vec2{{X: a, Y: lo}, {X: b, Y: lo}, {X: b, Y: hi}, {X: a, Y: hi}}, e
}

// RoundedRectangle specifies a planar rectangle with independent circular
// corner radii in counterclockwise order starting at Min. A zero radius keeps
// that corner sharp. Adjacent corner arcs must leave a positive straight side.
type RoundedRectangle struct {
	Min, Max geom.Vec2
	Radii    [4]float64
}

// ExtrudeRoundedRectangle sweeps a mixed line and quarter-circle profile with
// optional through bores and blind pockets. The retained boundary consists of
// analytic planar and cylindrical faces, with common model edges and circular
// cap trims. Tangent vertices must be exactly representable in the sketch.
// Holes must lie strictly inside the polygon of straight sides and corner
// chords, a conservative subset of the rounded profile. Height and blind
// pocket entry semantics match ExtrudeDiskWithHoles.
func ExtrudeRoundedRectangle(ctx context.Context, frame geom.Frame, profile RoundedRectangle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) (Solid, error) {
	return ExtrudeMixedRectangle(ctx, frame, profile, nil, through, blind, height)
}

// ExtrudeMixedRectangle sweeps an optionally rounded rectangle with strictly
// disjoint rectangular through pockets, circular bores, and circular blind
// pockets. Rectangular holes retain planar walls and the circular trims stay
// analytic. Every opening is wholly inside the corner-chord polygon.
// Rectangular walls are appended before the circular cavity walls.
func ExtrudeMixedRectangle(ctx context.Context, frame geom.Frame, profile RoundedRectangle, rectangles []ExtrusionRectangle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) (Solid, error) {
	return extrudeMixedRectangle(ctx, frame, profile, rectangles, nil, through, blind, height)
}

func extrudeMixedRectangle(ctx context.Context, frame geom.Frame, profile RoundedRectangle, rectangles []ExtrusionRectangle, boss *ExtrusionBoss, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) (Solid, error) {
	holes, e := validateExtrusionHoles(ctx, frame, through, blind, height)
	if e != nil {
		return Solid{}, e
	}
	outer, arcs, e := roundedExtrusionProfile(profile)
	if e != nil {
		return Solid{}, e
	}
	for _, h := range holes {
		if e := proveDiskInConvexExtrusion(outer, h); e != nil {
			return Solid{}, e
		}
	}
	footprints := rectangles
	if boss != nil {
		if !finite(boss.Height) || boss.Height <= 0 {
			return Solid{}, geom.ErrInvalid
		}
		footprints = append(append([]ExtrusionRectangle(nil), rectangles...), boss.Footprint)
		plane := max(0, height)
		offset := boss.Height
		if !boss.Upper {
			plane, offset = min(0, height), -offset
		}
		if _, e := geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(plane), ratInset(offset))); e != nil {
			return Solid{}, e
		}
	}
	rings, e := proveRectangularExtrusionHoles(outer, footprints, holes)
	if e != nil {
		return Solid{}, e
	}
	return extrudeCircularTopology(ctx, frame, outer, nil, arcs, rings[:len(rectangles)], boss, through, blind, min(0, height), max(0, height))
}

func roundedExtrusionProfile(p RoundedRectangle) ([]geom.Vec2, []extrusionQuarterArc, error) {
	if !uvFinite(p.Min) || !uvFinite(p.Max) || p.Min.X >= p.Max.X || p.Min.Y >= p.Max.Y {
		return nil, nil, geom.ErrInvalid
	}
	dims := [2]*big.Rat{new(big.Rat).Sub(ratInset(p.Max.X), ratInset(p.Min.X)), new(big.Rat).Sub(ratInset(p.Max.Y), ratInset(p.Min.Y))}
	for i, r := range p.Radii {
		if !finite(r) || r < 0 {
			return nil, nil, geom.ErrInvalid
		}
		s := p.Radii[(i+1)%4]
		if !finite(s) || s < 0 {
			return nil, nil, geom.ErrInvalid
		}
		if new(big.Rat).Add(ratInset(r), ratInset(s)).Cmp(dims[i%2]) >= 0 {
			return nil, nil, geom.ErrDomain
		}
	}
	corners := [4]geom.Vec2{p.Min, {X: p.Max.X, Y: p.Min.Y}, p.Max, {X: p.Min.X, Y: p.Max.Y}}
	in := [4]geom.Vec2{{Y: 1}, {X: -1}, {Y: -1}, {X: 1}}
	out := [4]geom.Vec2{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	var points []geom.Vec2
	var arcs []extrusionQuarterArc
	offset := func(c geom.Vec2, r float64, d geom.Vec2) (geom.Vec2, error) {
		x, e := geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(c.X), ratInset(r*d.X)))
		if e != nil {
			return geom.Vec2{}, e
		}
		y, e := geom.RepresentedExactScalar(new(big.Rat).Add(ratInset(c.Y), ratInset(r*d.Y)))
		return geom.Vec2{X: x, Y: y}, e
	}
	for i, c := range corners {
		r := p.Radii[i]
		if r == 0 {
			points, arcs = append(points, c), append(arcs, extrusionQuarterArc{})
			continue
		}
		a, e := offset(c, r, in[i])
		if e != nil {
			return nil, nil, e
		}
		b, e := offset(c, r, out[i])
		if e != nil {
			return nil, nil, e
		}
		center, e := offset(a, r, out[i])
		if e != nil {
			return nil, nil, e
		}
		points = append(points, a, b)
		arcs = append(arcs, extrusionQuarterArc{ExtrusionCircle{Center: center, Radius: r}, (i + 2) % 4}, extrusionQuarterArc{})
	}
	return points, arcs, nil
}

func proveDiskInConvexExtrusion(outer []geom.Vec2, h ExtrusionCircle) error {
	r2 := new(big.Rat).Mul(ratInset(h.Radius), ratInset(h.Radius))
	for j, a := range outer {
		b := outer[(j+1)%len(outer)]
		x, y := new(big.Rat).Sub(ratInset(b.X), ratInset(a.X)), new(big.Rat).Sub(ratInset(b.Y), ratInset(a.Y))
		u, v := new(big.Rat).Sub(ratInset(h.Center.X), ratInset(a.X)), new(big.Rat).Sub(ratInset(h.Center.Y), ratInset(a.Y))
		c := new(big.Rat).Sub(new(big.Rat).Mul(x, v), new(big.Rat).Mul(y, u))
		len2 := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
		if c.Sign() <= 0 || new(big.Rat).Mul(c, c).Cmp(new(big.Rat).Mul(r2, len2)) <= 0 {
			return geom.ErrDomain
		}
	}
	return nil
}
