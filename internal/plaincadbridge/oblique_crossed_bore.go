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
	blindDepth           float64
	oppositeCap          bool
}

// Use the exact authored ratio for intersection and coaxiality predicates.
// The input direction is the normalized binary representation of that ratio.
func obliqueCrossingAxis(direction geom.Vec3) (geom.Vec2, float64, bool) {
	if direction.X == 0 || direction.Y != 0 || direction.Z == 0 {
		return geom.Vec2{}, 0, false
	}
	ratio, ok := trimmed.CrossedObliqueAxisRatio(math.Abs(direction.X), math.Abs(direction.Z))
	return ratio, math.Copysign(1, direction.X*direction.Z), ok
}

func obliqueLineZ(origin geom.Vec3, x float64, ratio geom.Vec2, slopeSign float64) *big.Rat {
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	dx := new(big.Rat).Sub(rat(x), rat(origin.X))
	return new(big.Rat).Add(rat(origin.Z), dx.Mul(dx, big.NewRat(int64(slopeSign*ratio.Y), int64(ratio.X))))
}

// Select a right-handed world-coordinate chart from the two cutter axes.
// The cardinal direction is normalized to a positive world coordinate so
// reversing an authored tool preserves the same STEP recipe.
func obliqueCrossingChart(cardinal, oblique geom.Vec3) (geom.Frame, geom.Vec2, float64, bool) {
	var x, z geom.Vec3
	basis := [3]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}}
	for _, v := range basis {
		if math.Abs(cardinal.Dot(v)) == 1 {
			z = v
		} else if oblique.Dot(v) != 0 {
			if x != (geom.Vec3{}) {
				return geom.Frame{}, geom.Vec2{}, 0, false
			}
			x = v
		}
	}
	f, err := geom.NewFrame(geom.Vec3{}, x, z)
	if err != nil {
		return geom.Frame{}, geom.Vec2{}, 0, false
	}
	ratio, sign, ok := obliqueCrossingAxis(f.ToLocal(oblique))
	return f, ratio, sign, ok
}

func newObliqueCrossedBore(base brep.Solid, tools []*cylinderBody) (*boreBody, bool, error) {
	cardinal, oblique := tools[0], tools[1]
	if !cardinalFrame(cardinal.frame) {
		cardinal, oblique = oblique, cardinal
	}
	if !cardinalFrame(cardinal.frame) || cardinalFrame(oblique.frame) || cardinal.radius != oblique.radius {
		return nil, false, nil
	}
	chart, ratio, pattern, ok := obliqueCrossingChart(cardinal.frame.ZAxis(), oblique.frame.ZAxis())
	if !ok {
		return nil, false, nil
	}
	a := chart.ToLocal(cardinal.frame.Origin())
	b := chart.ToLocal(oblique.frame.Origin())
	if a.Y != b.Y {
		return nil, false, nil
	}
	z, _ := obliqueLineZ(b, a.X, ratio, pattern).Float64()
	if !finite(z) {
		return nil, false, nil
	}
	center := geom.Vec3{X: a.X, Y: a.Y, Z: z}
	bounds, ok := orthogonalPrism(base, chart)
	if !ok {
		return nil, false, nil
	}
	origin, far := bounds.Min, bounds.Max
	if pattern < 0 {
		origin = geom.Vec3{X: bounds.Min.X, Y: bounds.Max.Y, Z: bounds.Max.Z}
		far = geom.Vec3{X: bounds.Max.X, Y: bounds.Min.Y, Z: bounds.Min.Z}
	}
	f, err := geom.NewFrame(chart.ToWorld(origin), chart.XAxis(), chart.ZAxis().Scale(pattern))
	if err != nil {
		return nil, true, err
	}
	dims := bounds.Max.Sub(bounds.Min)
	worldCenter := chart.ToWorld(center)
	local := f.ToLocal(worldCenter)
	if f.ToWorld(local) != worldCenter || f.ToWorld(dims) != chart.ToWorld(far) {
		return nil, false, nil
	}
	// Both finite tools must cover their entire intended stock openings.
	cuts := make([]trimmed.CylinderCut, 2)
	spans := make([]float64, 2)
	blind := make([]bool, 2)
	floors := make([]int, 2)
	cardinalSlot := 0
	g := obliqueCrossedGeometry{frame: f, dimensions: dims, crossing: local, radius: cardinal.radius, axisRatio: ratio}
	for i, tool := range tools {
		if tool == cardinal {
			cardinalSlot = i
			c, lo, hi, err := boreSpan(base, tool, true)
			if err != nil {
				c, lo, hi, err = boreSpan(base, tool, false)
				if err != nil {
					return nil, true, err
				}
				axis := tool.frame.ZAxis()
				start := f.ToLocal(tool.frame.Origin()).Z
				finish := f.ToLocal(tool.frame.Origin().Add(axis.Scale(tool.height))).Z
				g.oppositeCap = axis.Dot(f.ZAxis()) < 0
				g.blindDepth = finish
				if g.oppositeCap {
					start, g.blindDepth = dims.Z-start, dims.Z-finish
				}
				if start > 0 || g.blindDepth <= 0 || g.blindDepth >= dims.Z {
					return nil, false, nil
				}
				blind[i] = true
			}
			cuts[i], spans[i] = c, hi-lo
			if blind[i] {
				spans[i] = g.blindDepth
			}
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
				if math.Abs(support.Normal().Dot(chart.XAxis())) != 1 {
					return nil, false, nil
				}
				axis := chart.ToLocal(tool.frame.ZAxis())
				cap := chart.ToLocal(support.Origin())
				projected := (cap.X - b.X) / axis.X
				capPoint := b.Add(axis.Scale(projected))
				want := center.Z + pattern*(ratio.Y/ratio.X)*(cap.X-center.X)
				if math.Abs(capPoint.Z-want) > 1e-9/16 {
					return nil, true, geom.ErrUnrepresentable
				}
			}
			cuts[i], spans[i] = single.cuts[0], single.spans[0]
		}
	}
	var r trimmed.BoxCrossedBoresResult
	if g.blindDepth > 0 {
		r, floors[cardinalSlot], err = trimmed.BoxBlindCrossedObliqueBores(context.Background(), f, dims, local, ratio, g.radius, g.blindDepth, g.oppositeCap, 1e-9)
	} else {
		r, err = trimmed.BoxCrossedObliqueBores(context.Background(), f, dims, local, ratio, g.radius, 1e-9)
	}
	if errors.Is(err, geom.ErrDomain) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	return &boreBody{base: base, solid: r.Solid, cylinders: tools, cuts: cuts, spans: spans, blind: blind, floors: floors,
		crossed: &r, obliqueCrossing: &g}, true, nil
}

// obliqueBoreContainsTool compares exact authored ratios and transverse lines.
// The caller must prove that the covering cylinder is through the stock.
func obliqueBoreContainsTool(old, tool *cylinderBody, chart geom.Frame) bool {
	if tool.radius > old.radius {
		return false
	}
	ratio, sign, ok := obliqueCrossingAxis(chart.ToLocal(old.frame.ZAxis()))
	otherRatio, otherSign, otherOK := obliqueCrossingAxis(chart.ToLocal(tool.frame.ZAxis()))
	if !ok || !otherOK || ratio != otherRatio || sign != otherSign {
		return false
	}
	o, t := chart.ToLocal(old.frame.Origin()), chart.ToLocal(tool.frame.Origin())
	return t.Y == o.Y && obliqueLineZ(t, o.X, ratio, sign).Cmp(new(big.Rat).SetFloat64(o.Z)) == 0
}

// newGroupedObliqueCrossing retains the two cutters that cover every member
// of a batch. It qualifies each candidate as a regular finite crossing
// before using its oblique through span or cardinal stop for containment.
func newGroupedObliqueCrossing(base brep.Solid, tools []*cylinderBody) (*boreBody, bool, error) {
	var cardinal, oblique []*cylinderBody
	for _, tool := range tools {
		if cardinalFrame(tool.frame) {
			cardinal = append(cardinal, tool)
		} else {
			oblique = append(oblique, tool)
		}
	}
	if len(cardinal) == 0 || len(oblique) == 0 {
		return nil, false, nil
	}
	// A qualified crossing has just two axis lines and a common radius.
	firstCardinal, firstOblique := cardinal[0], oblique[0]
	chart, _, _, ok := obliqueCrossingChart(firstCardinal.frame.ZAxis(), firstOblique.frame.ZAxis())
	if !ok {
		return nil, false, nil
	}
	for _, tool := range cardinal[1:] {
		if !sameCardinalBoreLine(firstCardinal, tool) {
			return nil, false, nil
		}
		if tool.radius > firstCardinal.radius {
			firstCardinal = tool
		}
	}
	for _, tool := range oblique[1:] {
		if tool.radius > firstOblique.radius {
			firstOblique = tool
		}
	}
	if firstCardinal.radius != firstOblique.radius {
		return nil, false, nil
	}
	for _, tool := range oblique {
		if !obliqueBoreContainsTool(firstOblique, tool, chart) {
			return nil, false, nil
		}
	}

	// Pick an oblique tool that actually spans both stock openings. A
	// shorter duplicate can share the same line and radius.
	var through *cylinderBody
	for _, tool := range oblique {
		if tool.radius != firstOblique.radius {
			continue
		}
		if _, handled, err := newObliqueThroughBore(base, tool); handled && err == nil {
			through = tool
			break
		}
	}
	if through == nil {
		return nil, false, nil
	}
	// Check the cardinal span before constructing the crossing, so the knot
	// and tessellation work is done only once even with repeated cutters.
	for _, candidate := range cardinal {
		if candidate.radius != firstCardinal.radius {
			continue
		}
		_, _, _, err := boreSpan(base, candidate, true)
		blind := err != nil
		covered := true
		for _, tool := range cardinal {
			if !cardinalBoreContainsTool(candidate, blind, tool) {
				covered = false
				break
			}
		}
		if covered {
			body, handled, err := newObliqueCrossedBore(base, []*cylinderBody{candidate, through})
			if handled && err == nil {
				return body, true, nil
			}
		}
	}
	return nil, false, nil
}

func overlapObliqueCrossedBore(a stored, tool *cylinderBody) (float64, bool, error) {
	if g := a.bore.obliqueCrossing; g != nil {
		for i, old := range a.bore.cylinders {
			if cardinalBoreContainsTool(old, a.bore.blind[i], tool) || obliqueBoreContainsTool(old, tool, g.frame) {
				return 0, true, nil
			}
		}
		return overlapAfterObliqueCrossing(a, append(append([]*cylinderBody(nil), a.bore.cylinders...), tool))
	}
	if len(a.bore.cylinders) != 1 {
		return 0, false, nil
	}
	return overlapAfterObliqueCrossing(a, []*cylinderBody{a.bore.cylinders[0], tool})
}

func overlapAfterObliqueCrossing(a stored, tools []*cylinderBody) (float64, bool, error) {
	fail := func(err error) (float64, bool, error) { return 0, true, err }
	var result *boreBody
	var handled bool
	var err error
	if len(tools) == 2 {
		result, handled, err = newObliqueCrossedBore(a.bore.base, tools)
	} else {
		result, handled, err = newGroupedObliqueCrossing(a.bore.base, tools)
	}
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
	p.BlindDepth *= scale
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
	start, direction, height := c.Z-d.Z, f.ZAxis(), 3*d.Z
	if p.BlindDepth != 0 {
		start, height = 0, p.BlindDepth
		if p.OppositeCap {
			start, direction = d.Z, direction.Scale(-1)
		}
	}
	cf, err := geom.FrameFromNormal(f.ToWorld(geom.Vec3{X: c.X, Y: c.Y, Z: start}), direction)
	if err != nil {
		return nil, err
	}
	cardinal, err := newCylinder(cf, p.Radius, height)
	if err != nil {
		return nil, err
	}
	of, err := geom.FrameFromNormal(center.Sub(declared.Scale(e)), declared)
	if err != nil {
		return nil, err
	}
	oblique, err := newCylinder(of, p.Radius, 2*e*math.Hypot(ratio.X, ratio.Y))
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
