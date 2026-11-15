package plaincadbridge

import (
	"context"
	"math"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type rationalPoint2 [2]*big.Rat

func rationalXY(p geom.Vec2) rationalPoint2 {
	return rationalPoint2{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y)}
}

func (a rationalPoint2) sub(b rationalPoint2) rationalPoint2 {
	return rationalPoint2{new(big.Rat).Sub(a[0], b[0]), new(big.Rat).Sub(a[1], b[1])}
}

func (a rationalPoint2) add(b rationalPoint2) rationalPoint2 {
	return rationalPoint2{new(big.Rat).Add(a[0], b[0]), new(big.Rat).Add(a[1], b[1])}
}

func (a rationalPoint2) dot(b rationalPoint2) *big.Rat {
	return new(big.Rat).Add(new(big.Rat).Mul(a[0], b[0]), new(big.Rat).Mul(a[1], b[1]))
}

func (a rationalPoint2) cross(b rationalPoint2) *big.Rat {
	return new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0]))
}

// The source already proves a simple loop of exact straight edges and CCW
// minor arcs. Its convex chord polygon and nonnegative exact tangent turns
// give a convex curved loop: each arc uses the turn removed from its two
// adjacent chord corners.
func convexStepLoop(segments []trimmed.ExactCircularLoopSegment) bool {
	tangent := func(s trimmed.ExactCircularLoopSegment, at geom.Vec2) rationalPoint2 {
		if s.Radius == 0 {
			return rationalXY(s.End).sub(rationalXY(s.Start))
		}
		r := rationalXY(at).sub(rationalXY(s.Center))
		return rationalPoint2{new(big.Rat).Neg(r[1]), r[0]}
	}
	for i, s := range segments {
		for _, other := range segments {
			sign, err := geom.Orient2D(s.Start, s.End, other.Start)
			if err != nil || sign < 0 {
				return false
			}
		}
		next := segments[(i+1)%len(segments)]
		a, b := tangent(s, s.End), tangent(next, next.Start)
		turn := a.cross(b).Sign()
		if turn < 0 || turn == 0 && a.dot(b).Sign() <= 0 {
			return false
		}
	}
	return true
}

type stepLoopMeshPoint struct {
	exact, display rationalPoint2
	xy             geom.Vec2
	rounding       *big.Rat
}

func newStepLoopMeshPoint(exact rationalPoint2) stepLoopMeshPoint {
	x, _ := exact[0].Float64()
	y, _ := exact[1].Float64()
	point := stepLoopMeshPoint{exact: exact, xy: geom.Vec2{X: x, Y: y}}
	if !finite(x) || !finite(y) {
		return point
	}
	point.display = rationalXY(point.xy)
	delta := point.display.sub(exact)
	point.rounding = new(big.Rat).Add(new(big.Rat).Abs(delta[0]), new(big.Rat).Abs(delta[1]))
	return point
}

// For an angle below pi/2, its tangent is an upper bound in radians.
func stepLoopWithinAngle(a, b rationalPoint2, angleSquared *big.Rat) bool {
	dot := a.dot(b)
	if dot.Sign() <= 0 {
		return false
	}
	cross := a.cross(b)
	crossSquared := new(big.Rat).Mul(cross, cross)
	bound := new(big.Rat).Mul(dot, dot)
	return crossSquared.Cmp(bound.Mul(bound, angleSquared)) <= 0
}

type stepLoopMesh struct {
	ring                      []geom.Vec2
	deflection, angleSquared  *big.Rat
	maxPoints, maxArcSegments int
}

func (m *stepLoopMesh) add(p geom.Vec2) error {
	if len(m.ring) == m.maxPoints {
		return parametric.ErrBudget
	}
	m.ring = append(m.ring, p)
	return nil
}

func (m *stepLoopMesh) line(a, b stepLoopMeshPoint) error {
	if a.rounding == nil || b.rounding == nil || a.rounding.Cmp(m.deflection) > 0 || b.rounding.Cmp(m.deflection) > 0 {
		return parametric.ErrBudget
	}
	if m.angleSquared.Sign() > 0 && !stepLoopWithinAngle(b.exact.sub(a.exact), b.display.sub(a.display), m.angleSquared) {
		return parametric.ErrBudget
	}
	return m.add(a.xy)
}

func (m *stepLoopMesh) arc(center, start, end rationalPoint2, radius *big.Rat) error {
	one, two := big.NewRat(1, 1), big.NewRat(2, 1)
	radiusSquared := new(big.Rat).Mul(radius, radius)
	parameter := func(q rationalPoint2) *big.Rat {
		return new(big.Rat).Quo(start.cross(q), new(big.Rat).Add(radiusSquared, start.dot(q)))
	}
	// Exact axis extrema prevent placement from shrinking the mesh bounds.
	knots := []*big.Rat{new(big.Rat), parameter(end)}
	negative := new(big.Rat).Neg(radius)
	zero := new(big.Rat)
	for _, axis := range []rationalPoint2{{radius, zero}, {zero, radius}, {negative, zero}, {zero, negative}} {
		if start.cross(axis).Sign() > 0 && axis.cross(end).Sign() > 0 {
			knots = append(knots, parameter(axis))
		}
	}
	slices.SortFunc(knots, (*big.Rat).Cmp)
	pieces := len(knots) - 1
	if pieces > m.maxArcSegments {
		return parametric.ErrBudget
	}
	point := func(t *big.Rat) stepLoopMeshPoint {
		// The positive denominator is the exact tangent-half-angle map.
		sq := new(big.Rat).Mul(t, t)
		denom := new(big.Rat).Add(one, sq)
		cos := new(big.Rat).Quo(new(big.Rat).Sub(one, sq), denom)
		sin := new(big.Rat).Quo(new(big.Rat).Mul(two, t), denom)
		r := rationalPoint2{
			new(big.Rat).Sub(new(big.Rat).Mul(cos, start[0]), new(big.Rat).Mul(sin, start[1])),
			new(big.Rat).Add(new(big.Rat).Mul(sin, start[0]), new(big.Rat).Mul(cos, start[1])),
		}
		return newStepLoopMeshPoint(center.add(r))
	}
	fourRadius := new(big.Rat).Mul(radius, big.NewRat(4, 1))
	var emit func(*big.Rat, *big.Rat, stepLoopMeshPoint, stepLoopMeshPoint) error
	emit = func(lo, hi *big.Rat, a, b stepLoopMeshPoint) error {
		if a.rounding == nil || b.rounding == nil || a.rounding.Cmp(m.deflection) > 0 || b.rounding.Cmp(m.deflection) > 0 {
			return parametric.ErrBudget
		}
		chord := b.exact.sub(a.exact)
		// Sagitta = chord² / (4 * (radius + distance(center, chord))).
		errorBound := new(big.Rat).Quo(chord.dot(chord), fourRadius)
		rounding := a.rounding
		if b.rounding.Cmp(rounding) > 0 {
			rounding = b.rounding
		}
		errorBound.Add(errorBound, rounding)
		chord = b.display.sub(a.display)
		normal := rationalPoint2{chord[1], new(big.Rat).Neg(chord[0])}
		normalOK := m.angleSquared.Sign() == 0 ||
			stepLoopWithinAngle(normal, a.exact.sub(center), m.angleSquared) &&
				stepLoopWithinAngle(normal, b.exact.sub(center), m.angleSquared)
		if errorBound.Cmp(m.deflection) <= 0 && normalOK && (chord[0].Sign() != 0 || chord[1].Sign() != 0) {
			return m.add(a.xy)
		}
		if pieces == m.maxArcSegments {
			return parametric.ErrBudget
		}
		pieces++
		mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), two)
		p := point(mid)
		if err := emit(lo, mid, a, p); err != nil {
			return err
		}
		return emit(mid, hi, p, b)
	}
	a := point(knots[0])
	for i := 1; i < len(knots); i++ {
		b := point(knots[i])
		if err := emit(knots[i-1], knots[i], a, b); err != nil {
			return err
		}
		a = b
	}
	return nil
}

// The two planar caps are convex. Their exact chord polygon lies inside the
// source, and maximum distance to it is bounded on the curved boundary by the
// largest sagitta. Rounding perturbs convex combinations by at most the
// largest vertex error. The same bound covers axial strips pointwise at a
// common height. Each source normal on a minor arc lies in the cone between
// the endpoint radials, so checking both endpoints bounds the whole strip.
func (p stepArcLoopSupport) tessellate(o parametric.TessellationOptions) (mesh.Mesh, bool, error) {
	return p.tessellateWithHole(context.Background(), o, nil)
}

func (p stepArcLoopSupport) tessellateWithHole(ctx context.Context, o parametric.TessellationOptions, hole *trimmed.ExtrusionCircle) (mesh.Mesh, bool, error) {
	d := p.descriptor
	if !p.exactPlacement || !convexStepLoop(d.Segments) {
		return mesh.Mesh{}, false, nil
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, true, geom.ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 256
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	origin, x, y, z := d.Frame.Origin(), d.Frame.XAxis(), d.Frame.YAxis(), d.Frame.ZAxis()
	translation := rationalXY(geom.Vec2{X: origin.Dot(x), Y: origin.Dot(y)})
	station := new(big.Rat).SetFloat64(origin.Dot(z))
	var levels [2]float64
	axialError := new(big.Rat)
	for i, h := range []float64{min(0, d.Height), max(0, d.Height)} {
		exact := new(big.Rat).Add(station, new(big.Rat).SetFloat64(h))
		levels[i], _ = exact.Float64()
		if !finite(levels[i]) {
			return mesh.Mesh{}, true, geom.ErrUnrepresentable
		}
		delta := new(big.Rat).Abs(new(big.Rat).Sub(exact, new(big.Rat).SetFloat64(levels[i])))
		if delta.Cmp(axialError) > 0 {
			axialError = delta
		}
	}
	angle := new(big.Rat).SetFloat64(o.Angle)
	m := stepLoopMesh{
		deflection:     new(big.Rat).Sub(new(big.Rat).SetFloat64(o.Deflection), axialError),
		angleSquared:   new(big.Rat).Mul(angle, angle),
		maxPoints:      o.MaxTriangles/4 + 1,
		maxArcSegments: o.MaxSubdivisions,
	}
	if levels[0] >= levels[1] || m.deflection.Sign() < 0 {
		return mesh.Mesh{}, true, parametric.ErrBudget
	}
	for _, s := range d.Segments {
		start, end := rationalXY(s.Start), rationalXY(s.End)
		var err error
		if s.Radius == 0 {
			err = m.line(newStepLoopMeshPoint(start.add(translation)), newStepLoopMeshPoint(end.add(translation)))
		} else {
			center := rationalXY(s.Center)
			err = m.arc(center.add(translation), start.sub(center), end.sub(center), new(big.Rat).SetFloat64(s.Radius))
		}
		if err != nil {
			return mesh.Mesh{}, true, err
		}
	}
	// Prove the rounded ring is a strictly convex, singly traversed polygon
	// before reusing the ordinary cap and axial-strip assembler.
	n := len(m.ring)
	if n > 10000 {
		return mesh.Mesh{}, false, nil
	}
	ring := make([]geom.Vec3, n)
	seen := make(map[geom.Vec2]bool, n)
	for i, point := range m.ring {
		if seen[point] {
			return mesh.Mesh{}, false, nil
		}
		seen[point] = true
		prev, next := m.ring[(i+n-1)%n], m.ring[(i+1)%n]
		sign, err := geom.Orient2D(prev, point, next)
		if err != nil || sign <= 0 {
			return mesh.Mesh{}, false, nil
		}
		for j, candidate := range m.ring {
			if j == i || j == (i+1)%n {
				continue
			}
			sign, err := geom.Orient2D(point, next, candidate)
			if err != nil || sign < 0 {
				return mesh.Mesh{}, false, nil
			}
		}
		ring[i] = geom.Vec3{X: point.X, Y: point.Y}
	}
	if hole != nil {
		if frameMap(d.Frame) != geom.Identity() {
			return mesh.Mesh{}, false, nil
		}
		display, err := tessellateStepLoopThroughHole(ctx, ring, *hole, m, levels, o.MaxTriangles)
		return display, true, err
	}
	display, err := mesh.Extrude(ring, geom.Vec3{Z: 1})
	if err != nil {
		return mesh.Mesh{}, false, nil
	}
	basis := frameMap(d.Frame)
	for i, point := range display.Vertices {
		point.Z = levels[i/n]
		display.Vertices[i] = basis.ApplyVector(point)
	}
	if err := display.ValidateSolid(); err != nil {
		return mesh.Mesh{}, true, err
	}
	return display, true, nil
}
