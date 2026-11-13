package geom

import (
	"context"
	"math"
	"math/big"
)

// SphereCylinderLensFan is an immutable numerical chart of one exact circular
// lens sector. U traverses its positively oriented arc, V runs from the exact
// chord midpoint to that arc. V=0 is collapsed; the two U sides are the shared
// chord halves. Exact boundary ownership is independent of rounded angles.
type SphereCylinderLensFan struct {
	owner               SphereCylinderCapCircleOwner
	center, hub         Vec3
	endpoints           [2]Vec3
	exactHub            [3]*big.Rat
	events              [2]SphereCylinderCapPoint
	radius, from, sweep float64
	ax, orientation     int
	radiusUpper         *big.Rat
}
type SphereCylinderLensFanBounds struct{ UU, UV, VV *big.Rat }

// NewSphereCylinderLensFans accepts only a strict transverse cap lens. Returns
// cylinder-owned fan first and sphere-owned fan second. Their straight sides
// match crosswise: fan0.U=0 is fan1.U=1 and vice versa, with identical V.
// Interior trigonometry is numerical; endpoint anchors are shared represented
// choices from certified enclosures. This is not lossless algebraic export.
func NewSphereCylinderLensFans(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, capParameter *big.Rat, o SurfacePatchEvaluationOptions) (out [2]SphereCylinderLensFan, err error) {
	q, e := ClassifySphereCylinderCapMaterial(ctx, center, sphereRadius, origin, axis, radius, capParameter, o)
	if e != nil {
		return out, e
	}
	if q.Kind != SphereCylinderCapLens || len(q.Events.Points) != 2 {
		return out, ErrDomain
	}
	ax := 0
	if axis.Y != 0 {
		ax = 1
	} else if axis.Z != 0 {
		ax = 2
	}
	orientation := 1
	if coordinate(axis, ax) < 0 {
		orientation = -1
	}
	j, k := (ax+1)%3, (ax+2)%3
	toVec := func(v [3]*big.Rat) (Vec3, error) {
		var a [3]float64
		for i := 0; i < 3; i++ {
			a[i], _ = v[i].Float64()
			if !finite(a[i]) {
				return Vec3{}, ErrUnrepresentable
			}
		}
		return Vec3{X: a[0], Y: a[1], Z: a[2]}, nil
	}
	var points [2]Vec3
	for n, p := range q.Events.Points {
		var v [3]*big.Rat
		for i := 0; i < 3; i++ {
			v[i] = new(big.Rat).Mul(new(big.Rat).Add(p.Bounds[i][0], p.Bounds[i][1]), big.NewRat(1, 2))
		}
		points[n], e = toVec(v)
		if e != nil {
			return out, e
		}
	}
	hub, e := toVec(q.Events.Points[0].Base)
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: q.ExactOperations}
	for n, arc := range q.Boundary {
		p := SphereCylinderLensFan{owner: arc.Owner, hub: hub, ax: ax, orientation: orientation}
		c, rs := q.CylinderCenter, q.CylinderRadiusSquared
		if arc.Owner == CapSphereCircle {
			c, rs = q.SphereSectionCenter, q.SphereRadiusSquared
		}
		p.center, e = toVec(c)
		if e != nil {
			return out, e
		}
		_, upper, e := rationalSqrtBounds(rs, 128, b)
		if e != nil {
			return out, e
		}
		p.radiusUpper = upper
		rf, _ := rs.Float64()
		p.radius = math.Sqrt(rf)
		if !finite(p.radius) || p.radius <= 0 {
			return out, ErrUnrepresentable
		}
		ids := [2]int{arc.StartEvent, arc.EndEvent}
		for i, id := range ids {
			p.endpoints[i] = points[id]
			p.events[i] = copyCapPoint(q.Events.Points[id])
		}
		for i := 0; i < 3; i++ {
			p.exactHub[i] = new(big.Rat).Set(q.Events.Points[0].Base[i])
		}
		start, end := p.endpoints[0].Sub(p.center), p.endpoints[1].Sub(p.center)
		p.from = math.Atan2(float64(orientation)*coordinate(start, k), coordinate(start, j))
		to := math.Atan2(float64(orientation)*coordinate(end, k), coordinate(end, j))
		p.sweep = to - p.from
		if p.sweep <= 0 {
			p.sweep += 2 * math.Pi
		}
		if p.sweep <= 0 || p.sweep >= 2*math.Pi {
			return out, ErrConvergence
		}
		out[n] = p
	}
	return out, nil
}
func copyCapPoint(p SphereCylinderCapPoint) SphereCylinderCapPoint {
	p.CapParameter = new(big.Rat).Set(p.CapParameter)
	p.Radicand = new(big.Rat).Set(p.Radicand)
	for i := 0; i < 3; i++ {
		p.Base[i] = new(big.Rat).Set(p.Base[i])
		p.Vector[i] = new(big.Rat).Set(p.Vector[i])
		for j := 0; j < 2; j++ {
			p.Bounds[i][j] = new(big.Rat).Set(p.Bounds[i][j])
		}
	}
	return p
}
func (p SphereCylinderLensFan) BoundaryProvenance() (SphereCylinderCapCircleOwner, [3]*big.Rat, [2]SphereCylinderCapPoint, error) {
	if p.radiusUpper == nil {
		return 0, [3]*big.Rat{}, [2]SphereCylinderCapPoint{}, ErrInvalid
	}
	h := [3]*big.Rat{}
	for i := 0; i < 3; i++ {
		h[i] = new(big.Rat).Set(p.exactHub[i])
	}
	return p.owner, h, [2]SphereCylinderCapPoint{copyCapPoint(p.events[0]), copyCapPoint(p.events[1])}, nil
}

// HessianBounds bounds the ideal exact angular chart, using sweep<8 radians.
// Floating evaluation/anchor rounding must be accounted for independently.
func (p SphereCylinderLensFan) HessianBounds() (SphereCylinderLensFanBounds, error) {
	if p.radiusUpper == nil {
		return SphereCylinderLensFanBounds{}, ErrInvalid
	}
	return SphereCylinderLensFanBounds{new(big.Rat).Mul(p.radiusUpper, big.NewRat(64, 1)), new(big.Rat).Mul(p.radiusUpper, big.NewRat(8, 1)), new(big.Rat)}, nil
}
func (p SphereCylinderLensFan) jet(u, v float64) (point, du, dv, duu, duv, dvv Vec3, err error) {
	if p.radiusUpper == nil || !finite(u) || !finite(v) || u < 0 || u > 1 || v < 0 || v > 1 {
		return point, du, dv, duu, duv, dvv, ErrDomain
	}
	a := p.from + p.sweep*u
	s, c := math.Sincos(a)
	j, k := (p.ax+1)%3, (p.ax+2)%3
	position, first, second := [3]float64{}, [3]float64{}, [3]float64{}
	for i := 0; i < 3; i++ {
		position[i] = coordinate(p.center, i)
	}
	position[j] += p.radius * c
	position[k] += float64(p.orientation) * p.radius * s
	first[j] = -p.radius * p.sweep * s
	first[k] = float64(p.orientation) * p.radius * p.sweep * c
	second[j] = -p.radius * p.sweep * p.sweep * c
	second[k] = -float64(p.orientation) * p.radius * p.sweep * p.sweep * s
	boundary := Vec3{X: position[0], Y: position[1], Z: position[2]}
	if u == 0 {
		boundary = p.endpoints[0]
	} else if u == 1 {
		boundary = p.endpoints[1]
	}
	dv = boundary.Sub(p.hub)
	point = p.hub.Add(dv.Scale(v))
	duv = Vec3{X: first[0], Y: first[1], Z: first[2]}
	du = duv.Scale(v)
	duu = Vec3{X: v * second[0], Y: v * second[1], Z: v * second[2]}
	for _, q := range []Vec3{point, du, dv, duu, duv} {
		if !q.IsFinite() {
			return point, du, dv, duu, duv, dvv, ErrConvergence
		}
	}
	return
}
func (p SphereCylinderLensFan) Eval(u, v float64) (Vec3, error) {
	x, _, _, _, _, _, e := p.jet(u, v)
	return x, e
}
func (p SphereCylinderLensFan) Partials(u, v float64) (Vec3, Vec3, error) {
	_, a, b, _, _, _, e := p.jet(u, v)
	return a, b, e
}
func (p SphereCylinderLensFan) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	_, _, _, a, b, c, e := p.jet(u, v)
	return a, b, c, e
}

// Domain is the unit fan rectangle; V=0 is collapsed to the chord midpoint.
func (p SphereCylinderLensFan) Domain() ParameterRectangle {
	return ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
}
