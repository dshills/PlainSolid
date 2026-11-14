package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type sideDualDiskRecipe struct {
	frame         geom.Frame
	min, max      geom.Vec2
	discs         [2]trimmed.ExtrusionCircle
	height, depth float64
}

func (p sideDualDiskRecipe) offsetCenters() bool {
	return p.discs[0].Center.Y == p.discs[1].Center.Y && p.discs[0].Center.Y < p.min.Y
}

func (p sideDualDiskRecipe) differentCenterY() bool {
	return p.discs[0].Center.Y != p.discs[1].Center.Y && p.discs[0].Center.Y < p.min.Y && p.discs[1].Center.Y < p.min.Y
}

// The squared half-chord is rational. Only a lossless rational square root
// proposes a join, and the native section then verifies both circle equations,
// side intercepts, the excluded crossing, and original wall ownership.
func (p sideDualDiskRecipe) unequalCenterJoin() ([2]*big.Rat, error) {
	var out [2]*big.Rat
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	a, b := p.discs[0], p.discs[1]
	dx, dy := new(big.Rat).Sub(q(b.Center.X), q(a.Center.X)), new(big.Rat).Sub(q(b.Center.Y), q(a.Center.Y))
	norm := new(big.Rat).Add(square(dx), square(dy))
	if norm.Sign() == 0 {
		return out, geom.ErrDomain
	}
	a2, b2 := square(q(a.Radius)), square(q(b.Radius))
	projection := new(big.Rat).Quo(new(big.Rat).Add(norm, new(big.Rat).Sub(a2, b2)), new(big.Rat).Mul(big.NewRat(2, 1), norm))
	perpendicular2 := new(big.Rat).Sub(new(big.Rat).Quo(a2, norm), square(projection))
	if perpendicular2.Sign() <= 0 {
		return out, geom.ErrDomain
	}
	n, d := new(big.Int).Sqrt(perpendicular2.Num()), new(big.Int).Sqrt(perpendicular2.Denom())
	if new(big.Int).Mul(n, n).Cmp(perpendicular2.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(perpendicular2.Denom()) != 0 {
		return out, geom.ErrUnrepresentable
	}
	perpendicular := new(big.Rat).SetFrac(n, d)
	if dx.Sign() < 0 {
		perpendicular.Neg(perpendicular)
	}
	out[0] = new(big.Rat).Add(q(a.Center.X), new(big.Rat).Sub(new(big.Rat).Mul(projection, dx), new(big.Rat).Mul(perpendicular, dy)))
	out[1] = new(big.Rat).Add(q(a.Center.Y), new(big.Rat).Add(new(big.Rat).Mul(projection, dy), new(big.Rat).Mul(perpendicular, dx)))
	return out, nil
}

func (p sideDualDiskRecipe) externalRadical() bool {
	x0, x1 := p.discs[0].Center.X, p.discs[1].Center.X
	left, right := p.discs[0].Radius, p.discs[1].Radius
	if x0 > x1 {
		x0, x1, left, right = x1, x0, right, left
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	delta := new(big.Rat).Sub(rat(x1), rat(x0))
	d2 := new(big.Rat).Mul(delta, delta)
	a, b := rat(left), rat(right)
	radiusDelta := new(big.Rat).Sub(a.Mul(a, a), b.Mul(b, b))
	return radiusDelta.Abs(radiusDelta).Cmp(d2) > 0
}

type sideDualDiskBody struct {
	sideDualDiskRecipe
	boundary   trimmed.Solid
	walls      [2][]int
	measures   trimmed.SideOpenDualDiskMeasures
	exactWalls bool
}

// planarFace resolves a finite face from the actual rational trim. Several
// stock walls share a support at the blind floor split; the query point must
// belong to exactly one of them. A point in an opening cannot select the wall.
func (b *sideDualDiskBody) planarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	n, e := normal.Unit()
	if e != nil || !origin.IsFinite() {
		return PlanarFace{}, geom.ErrInvalid
	}
	ctx := context.Background()
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	point := [3]*big.Rat{rat(origin.X), rat(origin.Y), rat(origin.Z)}
	var chosen *trimmed.AffineSplineFaceQuery
	var out PlanarFace
	for id, f := range b.boundary.Faces() {
		if id == b.walls[0][0] || id == b.walls[1][0] {
			continue
		}
		q, e := trimmed.NewAffineSplineFaceQuery(ctx, f, trimmed.AffineSplineFaceQueryOptions{MaxDomainSubdivisions: 128})
		if e != nil {
			return PlanarFace{}, e
		}
		plane := q.Plane()
		// Constructor-owned stock/floor planes are signed coordinate planes.
		// Normalize the rational normal before conversion to avoid overflow.
		var xyz [3]float64
		terms := 0
		for axis, c := range plane[:3] {
			if c.Sign() != 0 {
				terms++
				xyz[axis] = float64(c.Sign())
			}
		}
		faceNormal := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		if terms != 1 || math.Abs(faceNormal.Dot(n)) < 1-1e-9 {
			continue
		}
		location, e := q.ClassifyRationalPoint(ctx, point, geom.AffineChartInverseOptions{})
		if e != nil {
			return PlanarFace{}, e
		}
		if location.Location != trimvalidate.Inside && location.Location != trimvalidate.Boundary {
			continue
		}
		if chosen != nil {
			return PlanarFace{}, unsupportedEdge("planar face reference is ambiguous at a retained stock seam")
		}
		out = PlanarFace{Face: id, Origin: coordinates(origin), Normal: coordinates(faceNormal)}
		chosen = q
	}
	if chosen == nil {
		return PlanarFace{}, unsupportedEdge("planar face reference no longer exists in the trimmed material")
	}
	a, e := chosen.AreaBounds(ctx, big.NewRat(1, 10000), 0)
	if e != nil {
		return PlanarFace{}, e
	}
	out.Area, _ = new(big.Rat).Quo(new(big.Rat).Add(a.ModelArea.Lower, a.ModelArea.Upper), big.NewRat(2, 1)).Float64()
	return out, nil
}

func (p *sideDualDiskBody) analytic() *analyticBody {
	return &analyticBody{solid: p.boundary, sideDualDisk: p, volume: p.measures.Volume, area: p.measures.Area}
}

func (p sideDualDiskRecipe) solid() (*sideDualDiskBody, error) {
	ctx := context.Background()
	if p.differentCenterY() {
		join, e := p.unequalCenterJoin()
		if e != nil {
			return nil, e
		}
		cut, e := trimmed.CutUnequalCenterSideOpenDualDisks(ctx, p.frame, p.min, p.max, p.discs, join, p.height, p.depth, 1e-9, trimvalidate.Options{})
		if e != nil {
			return nil, e
		}
		m, e := cut.Measures()
		return &sideDualDiskBody{sideDualDiskRecipe: p, boundary: cut.Solid, walls: cut.WallFaces, measures: m}, e
	}
	if p.offsetCenters() {
		cut, e := trimmed.CutOffsetSideOpenDualDisks(ctx, p.frame, p.min, p.max, p.discs, p.height, p.depth, 1e-9, trimvalidate.Options{})
		if e != nil {
			return nil, e
		}
		m, e := cut.Measures()
		return &sideDualDiskBody{sideDualDiskRecipe: p, boundary: cut.Solid, walls: cut.WallFaces, measures: m}, e
	}
	if p.discs[0].Radius == p.discs[1].Radius {
		cut, e := trimmed.CutSideOpenDualDisks(ctx, p.frame, p.min, p.max, p.discs, p.height, p.depth, 1e-9, trimvalidate.Options{})
		if e != nil {
			return nil, e
		}
		m, e := cut.Measures()
		return &sideDualDiskBody{sideDualDiskRecipe: p, boundary: cut.Solid, walls: cut.WallFaces, measures: m}, e
	}
	if p.externalRadical() {
		cut, e := trimmed.CutGeneralSideOpenDualDisks(ctx, p.frame, p.min, p.max, p.discs, p.height, p.depth, 1e-9, trimvalidate.Options{})
		exactWalls := errors.Is(e, geom.ErrUnrepresentable)
		if exactWalls {
			// Both constructors enforce the same exact endpoints and stock proof.
			// This second attempt allows rational wall controls, without rounding.
			cut, e = trimmed.CutGeneralSideOpenDualDisksWithExactWalls(ctx, p.frame, p.min, p.max, p.discs, p.height, p.depth, 1e-9, trimvalidate.Options{})
		}
		if e != nil {
			return nil, e
		}
		m, e := cut.Measures()
		return &sideDualDiskBody{sideDualDiskRecipe: p, boundary: cut.Solid, walls: cut.WallFaces, measures: m, exactWalls: exactWalls}, e
	}
	cut, e := trimmed.CutUnequalSideOpenDualDisks(ctx, p.frame, p.min, p.max, p.discs, p.height, p.depth, 1e-9, trimvalidate.Options{})
	if e != nil {
		return nil, e
	}
	m, e := cut.Measures()
	return &sideDualDiskBody{sideDualDiskRecipe: p, boundary: cut.Solid, walls: cut.WallFaces, measures: m}, e
}

func putSideDualDiskSTEPProduct(b *Bridge, d step.SideOpenDualDiskDescriptor, native trimmed.Solid, walls [2][]int, measures trimmed.SideOpenDualDiskMeasures, exactWalls bool) (Shape, error) {
	p := sideDualDiskRecipe{d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth}
	cut := &sideDualDiskBody{sideDualDiskRecipe: p, boundary: native, walls: walls, measures: measures, exactWalls: exactWalls}
	if d.LengthUnit != .001 {
		// This reader proves a supported SI prefix. Identify that prefix
		// before multiplying, since the floats for 10^n are not generally exact.
		power := -18
		for ; power <= 18; power++ {
			if d.LengthUnit == math.Pow10(power) {
				break
			}
		}
		if power > 18 {
			return Shape{}, geom.ErrUnrepresentable
		}
		factor := big.NewRat(1, 1)
		for n := power + 3; n > 0; n-- {
			factor.Mul(factor, big.NewRat(10, 1))
		}
		for n := power + 3; n < 0; n++ {
			factor.Quo(factor, big.NewRat(10, 1))
		}
		o := p.frame.Origin()
		for _, v := range []*float64{&o.X, &o.Y, &o.Z, &p.min.X, &p.min.Y, &p.max.X, &p.max.Y,
			&p.discs[0].Center.X, &p.discs[0].Center.Y, &p.discs[1].Center.X, &p.discs[1].Center.Y,
			&p.discs[0].Radius, &p.discs[1].Radius, &p.height, &p.depth} {
			scaled, exact := new(big.Rat).Mul(new(big.Rat).SetFloat64(*v), factor).Float64()
			if !exact || !finite(scaled) {
				return Shape{}, geom.ErrUnrepresentable
			}
			*v = scaled
		}
		var e error
		p.frame, e = geom.NewFrame(o, p.frame.XAxis(), p.frame.ZAxis())
		if e != nil {
			return Shape{}, e
		}
		cut, e = p.solid()
		if e != nil {
			return Shape{}, e
		}
	}
	return b.putAnalyticBody(cut.analytic())
}

func dualDiskFromNotch(p step.SideNotchBody, c *cylinderBody) (sideDualDiskRecipe, bool) {
	var out sideDualDiskRecipe
	if c == nil || p.Left <= 0 || p.Inset > 0 || len(p.Through) != 0 || len(p.Blind) != 0 || !cardinalFrame(p.Frame) {
		return out, false
	}
	second, low, high, ok := sideNotchBore(p, c)
	first := trimmed.ExtrusionCircle{Radius: p.Radius}
	if !ok || p.Inset == 0 && second.Center.Y != 0 || p.Inset < 0 && (second.Center.Y >= -p.Inset || second.Radius == first.Radius) || second.Center.X == 0 || low != 0 || high != p.Depth ||
		compareCircularFootprints(first, second, false) >= 0 || compareCircularFootprints(first, second, true) <= 0 ||
		compareCircularFootprints(second, first, true) <= 0 {
		return out, false
	}
	return sideDualDiskRecipe{p.Frame, geom.Vec2{X: -p.Left, Y: -p.Inset}, geom.Vec2{X: p.Right, Y: p.Back}, [2]trimmed.ExtrusionCircle{first, second}, p.Length, p.Depth}, true
}

// dualDiskCutRecipe recognizes the original planar stock and both cutters,
// or a previously published single side opening and one additional cutter.
func dualDiskCutRecipe(a stored, tools []stored) (sideDualDiskRecipe, bool) {
	if a.analytic != nil && a.analytic.sideNotch != nil && len(tools) == 1 {
		return dualDiskFromNotch(*a.analytic.sideNotch, tools[0].cylinder)
	}
	if !a.planar() || len(tools) != 2 || tools[0].cylinder == nil || tools[1].cylinder == nil {
		return sideDualDiskRecipe{}, false
	}
	// Either tool may establish the stock's coordinate frame.
	if p, ok := dualDiskFromStock(a, tools[0].cylinder, tools[1].cylinder); ok {
		return p, true
	}
	return dualDiskFromStock(a, tools[1].cylinder, tools[0].cylinder)
}

func dualDiskFromStock(a stored, first, second *cylinderBody) (sideDualDiskRecipe, bool) {
	if f, box, ok := sideCylinderBlindRecipe(a.solid, first); ok && box.Min.Y >= 0 {
		return dualDiskFromNotch(sideNotchRecipe(f, box, first.radius, first.height), second)
	}
	if f, box, ok := sideCylinderThroughRecipe(a.solid, first); ok && box.Min.Y >= 0 {
		return dualDiskFromNotch(sideNotchRecipe(f, box, first.radius, box.Max.Z), second)
	}
	return sideDualDiskRecipe{}, false
}

func (b *Bridge) cutSideDualDisk(a stored, tools []stored) (Shape, bool, error) {
	p, ok := dualDiskCutRecipe(a, tools)
	if !ok {
		return Shape{}, false, nil
	}
	cut, e := p.solid()
	if e != nil {
		return Shape{}, true, e
	}
	if !removesMaterial(a.result.GeometryAssertions.Volume, cut.measures.Volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	out, e := b.putNotchedAnalytic(cut.analytic(), a.result.Bounds)
	return out, true, e
}

func overlapSideDualDisk(a, c stored) (float64, bool, error) {
	if c.cylinder == nil {
		return 0, false, nil
	}
	if a.analytic != nil && a.analytic.sideDualDisk != nil {
		p := *a.analytic.sideDualDisk
		d, lo, hi, ok := circularStockSpan(step.DiskHoleExtrusionBody{Frame: p.frame, Height: p.height}, c.cylinder)
		if ok && (d == p.discs[0] || d == p.discs[1]) {
			// A repeated cutter misses the removed prefix. Above its blind
			// floor the original stock still fills its exposed section.
			area, _ := sideCylinderSection(d.Center.Y-p.min.Y, d.Radius)
			return area * max(0, hi-max(lo, p.depth)), true, nil
		}
	}
	if a.analytic == nil || a.analytic.sideNotch == nil {
		return 0, false, nil
	}
	p, ok := dualDiskFromNotch(*a.analytic.sideNotch, c.cylinder)
	if !ok {
		return 0, false, nil
	}
	if p.offsetCenters() || p.differentCenterY() {
		cut, e := p.solid()
		if e != nil {
			return 0, true, e
		}
		v := a.result.GeometryAssertions.Volume - cut.measures.Volume
		if !finite(v) || v <= 0 {
			return 0, true, geom.ErrUnrepresentable
		}
		return v, true, nil
	}
	// Verify cutter clearance from the pristine side walls in exact input
	// arithmetic; capture is a measure-only query and need not build trims.
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x, radius := rat(p.discs[1].Center.X), rat(p.discs[1].Radius)
	if new(big.Rat).Sub(x, radius).Cmp(rat(p.min.X)) <= 0 || new(big.Rat).Add(x, radius).Cmp(rat(p.max.X)) >= 0 || radius.Cmp(rat(p.max.Y)) >= 0 {
		return 0, true, unsupportedEdge("additional side opening reaches another stock wall")
	}
	r0 := rat(p.discs[0].Radius)
	d := new(big.Rat).Abs(x)
	r02, r12 := new(big.Rat).Mul(r0, r0), new(big.Rat).Mul(radius, radius)
	firstOffset := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(d, d), r02), r12), new(big.Rat).Mul(d, big.NewRat(2, 1)))
	secondOffset := new(big.Rat).Sub(d, firstOffset)
	h2 := new(big.Rat).Sub(r02, new(big.Rat).Mul(firstOffset, firstOffset))
	af, _ := firstOffset.Float64()
	bf, _ := secondOffset.Float64()
	hf, _ := h2.Float64()
	df, _ := d.Float64()
	h := math.Sqrt(hf)
	// Circle/sweep integration gives the exposed part of the second half
	// disk after the first has already been removed. Signed offsets permit
	// the radical crossing to lie on either side of a cutter center.
	l, r := p.discs[0].Radius, p.discs[1].Radius
	area := (r*r*math.Atan2(h, -bf) - l*l*math.Atan2(h, af) + df*h) / 2
	volume := area * p.depth
	if !finite(volume) || volume <= 0 {
		return 0, true, geom.ErrUnrepresentable
	}
	return volume, true, nil
}

func (p sideDualDiskRecipe) bounds() (geom.Box, error) {
	var corners []geom.Vec3
	for _, x := range []float64{p.min.X, p.max.X} {
		for _, y := range []float64{p.min.Y, p.max.Y} {
			for _, z := range []float64{0, p.height} {
				corners = append(corners, p.frame.ToWorld(geom.Vec3{X: x, Y: y, Z: z}))
			}
		}
	}
	return geom.Bounds(corners)
}

func (p sideDualDiskRecipe) transformed(m geom.Affine) (sideDualDiskRecipe, bool) {
	x, y, z := m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.YAxis()), m.ApplyVector(p.frame.ZAxis())
	if m.Determinant() < 0 {
		x = x.Scale(-1)
		p.min.X, p.max.X = -p.max.X, -p.min.X
		for i := range p.discs {
			p.discs[i].Center.X = -p.discs[i].Center.X
		}
	}
	f, e := geom.NewFrame(m.Apply(p.frame.Origin()), x, z)
	if e != nil || !cardinalFrame(f) || f.XAxis() != x || f.YAxis() != y || f.ZAxis() != z {
		return sideDualDiskRecipe{}, false
	}
	p.frame = f
	return p, true
}
