package plaincadbridge

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type obliqueCrossedGeometry struct {
	frame                geom.Frame
	dimensions, crossing geom.Vec3
	radius               float64
}

// The only supported oblique direction has exact authored slope 3/4 in
// the XZ chart. Compare its represented direction and use the rational
// slope to qualify the common point.
func obliqueCrossingAxis(direction geom.Vec3) (float64, bool) {
	if direction.X == 0 || direction.Y != 0 || direction.Z == 0 {
		return 0, false
	}
	ax, az := math.Abs(direction.X), math.Abs(direction.Z)
	canonical, err := (geom.Vec3{X: 4, Z: 3}).Unit()
	if err != nil || math.Abs(ax-canonical.X) > 8*0x1p-52 || math.Abs(az-canonical.Z) > 8*0x1p-52 {
		return 0, false
	}
	return math.Copysign(1, direction.X*direction.Z), true
}

func obliqueCrossingPoint(cardinal, oblique *cylinderBody, sign float64) (geom.Vec3, bool) {
	a, b := cardinal.frame.Origin(), oblique.frame.Origin()
	if a.Y != b.Y {
		return geom.Vec3{}, false
	}
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	x := new(big.Rat).Sub(rat(a.X), rat(b.X))
	z := new(big.Rat).Add(rat(b.Z), x.Mul(x, big.NewRat(3*int64(sign), 4)))
	h, _ := z.Float64()
	return geom.Vec3{X: a.X, Y: a.Y, Z: h}, finite(h)
}

func newObliqueCrossedBore(base brep.Solid, tools []*cylinderBody) (*boreBody, bool, error) {
	cardinal, oblique := tools[0], tools[1]
	if !cardinalFrame(cardinal.frame) {
		cardinal, oblique = oblique, cardinal
	}
	if !cardinalFrame(cardinal.frame) || math.Abs(cardinal.frame.ZAxis().Z) != 1 || cardinal.radius != oblique.radius {
		return nil, false, nil
	}
	pattern, ok := obliqueCrossingAxis(oblique.frame.ZAxis())
	if !ok {
		return nil, false, nil
	}
	center, ok := obliqueCrossingPoint(cardinal, oblique, pattern)
	if !ok {
		return nil, false, nil
	}
	identity, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		return nil, true, err
	}
	bounds, ok := orthogonalPrism(base, identity)
	if !ok || !rectangularFaces(base, identity, bounds.Min, bounds.Max) {
		return nil, false, nil
	}
	origin, far := bounds.Min, bounds.Max
	if pattern < 0 {
		origin = geom.Vec3{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Max.Z}
		far = geom.Vec3{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Min.Z}
	}
	f, err := geom.NewFrame(origin, geom.Vec3{X: 1}, geom.Vec3{Z: pattern})
	if err != nil {
		return nil, true, err
	}
	dims := bounds.Max.Sub(bounds.Min)
	local := f.ToLocal(center)
	if f.ToWorld(local) != center || f.ToWorld(dims) != far {
		return nil, false, nil
	}
	// Both finite tools must cover their entire intended stock openings.
	cuts := make([]trimmed.CylinderCut, 2)
	spans := make([]float64, 2)
	for i, tool := range tools {
		if tool == cardinal {
			c, lo, hi, err := boreSpan(base, tool, true)
			if err != nil {
				return nil, true, err
			}
			cuts[i], spans[i] = c, hi-lo
		} else {
			single, handled, err := newObliqueThroughBore(base, tool)
			if err != nil {
				return nil, true, err
			}
			if !handled {
				return nil, false, nil
			}
			for _, face := range []int{single.cuts[0].Entry, single.cuts[0].Exit} {
				support, err := base.FacePlane(face)
				if err != nil || math.Abs(support.Normal().X) != 1 {
					return nil, true, geom.ErrDomain
				}
				axis := tool.frame.ZAxis()
				projected := (support.Origin().X - tool.frame.Origin().X) / axis.X
				world := tool.frame.Origin().Add(axis.Scale(projected))
				want := center.Z + pattern*.75*(support.Origin().X-center.X)
				if math.Abs(world.Z-want) > 1e-9/16 {
					return nil, true, geom.ErrUnrepresentable
				}
			}
			cuts[i], spans[i] = single.cuts[0], single.spans[0]
		}
	}
	g := obliqueCrossedGeometry{f, dims, local, cardinal.radius}
	r, err := trimmed.BoxCrossedBores345(context.Background(), f, dims, local, g.radius, 1e-9)
	if err != nil {
		return nil, true, err
	}
	return &boreBody{base: base, solid: r.Solid, cylinders: tools, cuts: cuts, spans: spans, blind: []bool{false, false},
		crossed: &r, obliqueCrossing: &g}, true, nil
}

func overlapObliqueCrossedBore(a stored, tool *cylinderBody) (float64, bool, error) {
	fail := func(err error) (float64, bool, error) { return 0, true, err }
	if a.bore.obliqueCrossing != nil {
		for _, old := range a.bore.cylinders {
			if sameCardinalBoreLine(old, tool) && old.radius >= tool.radius {
				return 0, true, nil
			}
		}
		// The retained oblique through wall covers all in-stock generators of
		// a coaxial tool. Compare the qualified opening centers at both caps.
		slope, ok := obliqueCrossingAxis(tool.frame.ZAxis())
		if ok && tool.radius <= a.bore.obliqueCrossing.radius {
			g := a.bore.obliqueCrossing
			center := g.frame.ToWorld(g.crossing)
			if slope == g.frame.ZAxis().Z && tool.frame.Origin().Y == center.Y {
				rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
				dx := new(big.Rat).Sub(rat(center.X), rat(tool.frame.Origin().X))
				z := new(big.Rat).Add(rat(tool.frame.Origin().Z), dx.Mul(dx, big.NewRat(3*int64(slope), 4)))
				if z.Cmp(rat(center.Z)) == 0 {
					return 0, true, nil
				}
			}
		}
		return 0, false, nil
	}
	if len(a.bore.cylinders) != 1 {
		return 0, false, nil
	}
	result, handled, err := newObliqueCrossedBore(a.bore.base, []*cylinderBody{a.bore.cylinders[0], tool})
	if !handled {
		return 0, false, nil
	}
	if err != nil {
		return fail(err)
	}
	after, _, err := result.measures()
	if err != nil {
		return fail(err)
	}
	delta := a.result.GeometryAssertions.Volume - after
	if !finite(delta) || delta < 0 {
		return fail(geom.ErrUnrepresentable)
	}
	return delta, true, nil
}

func crossedObliqueFromSTEP(p step.CrossedObliqueBoreBody, unit float64) (*boreBody, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 || !cardinalFrame(p.Frame) {
		return nil, geom.ErrInvalid
	}
	p.Frame, _ = geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
	p.Dimensions = p.Dimensions.Scale(scale)
	p.Crossing = p.Crossing.Scale(scale)
	p.Radius *= scale
	f := p.Frame
	bounds, err := geom.Bounds([]geom.Vec3{f.Origin(), f.ToWorld(p.Dimensions)})
	if err != nil {
		return nil, err
	}
	base, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return nil, err
	}
	d := p.Dimensions
	c := p.Crossing
	center := f.ToWorld(c)
	declared := f.XAxis().Scale(4).Add(f.ZAxis().Scale(3))
	// Exact binary offsets along the authored 4:3 direction preserve its
	// intersection point. This finite replacement clears every generator.
	extent := 4 * math.Max(d.X, p.Radius)
	e := math.Exp2(math.Ceil(math.Log2(extent)))
	if !finite(e) {
		return nil, geom.ErrInvalid
	}
	cf, err := geom.FrameFromNormal(center.Add(f.ZAxis().Scale(-d.Z)), f.ZAxis())
	if err != nil {
		return nil, err
	}
	cardinal, err := newCylinder(cf, p.Radius, 3*d.Z)
	if err != nil {
		return nil, err
	}
	of, err := geom.FrameFromNormal(center.Sub(declared.Scale(e)), declared)
	if err != nil {
		return nil, err
	}
	oblique, err := newCylinder(of, p.Radius, 10*e)
	if err != nil {
		return nil, err
	}
	g, handled, err := newObliqueCrossedBore(base, []*cylinderBody{cardinal, oblique})
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, geom.ErrUnrepresentable
	}
	return g, nil
}
