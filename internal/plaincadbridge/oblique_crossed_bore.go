package plaincadbridge

import (
	"context"
	"errors"
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
	axisRatio            geom.Vec2
}

// Use the exact authored ratio for intersection and coaxiality predicates.
// The input direction is the normalized binary representation of that ratio.
func obliqueCrossingAxis(direction geom.Vec3) (geom.Vec2, float64, bool) {
	if direction.X == 0 || direction.Y != 0 || direction.Z == 0 {
		return geom.Vec2{}, 0, false
	}
	ax, az := math.Abs(direction.X), math.Abs(direction.Z)
	for _, ratio := range [2]geom.Vec2{{X: 4, Y: 3}, {X: 3, Y: 4}} {
		if math.Abs(ax-ratio.X/5) <= 8*0x1p-52 && math.Abs(az-ratio.Y/5) <= 8*0x1p-52 {
			return ratio, math.Copysign(1, direction.X*direction.Z), true
		}
	}
	return geom.Vec2{}, 0, false
}

func obliqueLineZ(origin geom.Vec3, x float64, ratio geom.Vec2, slopeSign float64) *big.Rat {
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	dx := new(big.Rat).Sub(rat(x), rat(origin.X))
	return new(big.Rat).Add(rat(origin.Z), dx.Mul(dx, big.NewRat(int64(slopeSign*ratio.Y), int64(ratio.X))))
}

func newObliqueCrossedBore(base brep.Solid, tools []*cylinderBody) (*boreBody, bool, error) {
	cardinal, oblique := tools[0], tools[1]
	if !cardinalFrame(cardinal.frame) {
		cardinal, oblique = oblique, cardinal
	}
	if !cardinalFrame(cardinal.frame) || math.Abs(cardinal.frame.ZAxis().Z) != 1 || cardinal.radius != oblique.radius {
		return nil, false, nil
	}
	ratio, pattern, ok := obliqueCrossingAxis(oblique.frame.ZAxis())
	if !ok {
		return nil, false, nil
	}
	a, b := cardinal.frame.Origin(), oblique.frame.Origin()
	if a.Y != b.Y {
		return nil, false, nil
	}
	z, _ := obliqueLineZ(b, a.X, ratio, pattern).Float64()
	if !finite(z) {
		return nil, false, nil
	}
	center := geom.Vec3{X: a.X, Y: a.Y, Z: z}
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
				if err != nil {
					return nil, true, err
				}
				if math.Abs(support.Normal().X) != 1 {
					return nil, false, nil
				}
				axis := tool.frame.ZAxis()
				projected := (support.Origin().X - tool.frame.Origin().X) / axis.X
				world := tool.frame.Origin().Add(axis.Scale(projected))
				want := center.Z + pattern*(ratio.Y/ratio.X)*(support.Origin().X-center.X)
				if math.Abs(world.Z-want) > 1e-9/16 {
					return nil, true, geom.ErrUnrepresentable
				}
			}
			cuts[i], spans[i] = single.cuts[0], single.spans[0]
		}
	}
	g := obliqueCrossedGeometry{f, dims, local, cardinal.radius, ratio}
	r, err := trimmed.BoxCrossedObliqueBores(context.Background(), f, dims, local, ratio, g.radius, 1e-9)
	if errors.Is(err, geom.ErrDomain) {
		return nil, false, nil
	}
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
		// Compare the unrounded authored lines to recognize covered tools.
		ratio, sign, ok := obliqueCrossingAxis(tool.frame.ZAxis())
		g := a.bore.obliqueCrossing
		if ok && ratio == g.axisRatio && sign == g.frame.ZAxis().Z && tool.radius <= g.radius {
			o := tool.frame.Origin()
			for _, old := range a.bore.cylinders {
				if cardinalFrame(old.frame) {
					continue
				}
				b := old.frame.Origin()
				if o.Y == b.Y && obliqueLineZ(o, b.X, ratio, sign).Cmp(new(big.Rat).SetFloat64(b.Z)) == 0 {
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
	ratio := p.AxisRatio
	if ratio == (geom.Vec2{}) {
		ratio = geom.Vec2{X: 4, Y: 3}
	}
	declared := f.XAxis().Scale(ratio.X).Add(f.ZAxis().Scale(ratio.Y))
	// Exact binary offsets along the authored direction preserve its
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
