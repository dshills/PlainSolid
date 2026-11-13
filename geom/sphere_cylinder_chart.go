package geom

import (
	"context"
	"math"
	"math/big"
)

// SphereCylinderWallChart is an immutable monotone projection chart on one
// proved wall graph arc. Parameter t runs from 0 to1 in increasing projection.
// Interior Eval/Derivative are numerical analytic representatives; endpoint
// proof is supplied separately by EndpointBounds, never inferred from Eval.
type SphereCylinderWallChart struct {
	center, origin, axis Vec3
	radius               float64
	ax, half, sign       int
	H, A                 *big.Rat
	projection           [3]*big.Rat
	ends                 [2]SphereCylinderWallEvent
}

// NewSphereCylinderWallCharts constructs one chart per certified graph arc;
// isolated graph points do not create zero-length curves. Endpoint extrema or
// height folds may make the projection chart derivative unbounded.
func NewSphereCylinderWallCharts(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, window [2]*big.Rat, o SurfacePatchEvaluationOptions) ([]SphereCylinderWallChart, error) {
	g, e := BuildFiniteSphereCylinderWallGraph(ctx, center, sphereRadius, origin, axis, radius, window, o)
	if e != nil {
		return nil, e
	}
	ax := 0
	if axis.Y != 0 {
		ax = 1
	} else if axis.Z != 0 {
		ax = 2
	}
	var out []SphereCylinderWallChart
	for _, arc := range g.Arcs {
		c := SphereCylinderWallChart{center: center, origin: origin, axis: axis, radius: radius, ax: ax, half: arc.AngularHalf, sign: g.Domain.Branches[arc.Branch].HeightSign, H: new(big.Rat).Set(g.Domain.Angular.ProjectionSquared), A: new(big.Rat).Set(g.Domain.Angular.A)}
		for i := 0; i < 3; i++ {
			c.projection[i] = new(big.Rat).Set(g.Domain.Angular.Projection[i])
		}
		for i := 0; i < 2; i++ {
			c.ends[i] = copySphereCylinderEvent(g.Events[arc.Events[i]])
		}
		out = append(out, c)
	}
	return out, nil
}
func copySphereCylinderEvent(v SphereCylinderWallEvent) SphereCylinderWallEvent {
	if v.ProjectionValue != nil {
		v.ProjectionValue = new(big.Rat).Set(v.ProjectionValue)
	}
	v.IncidentArcs = append([]int(nil), v.IncidentArcs...)
	return v
}

// EndpointEvent returns owned exact graph provenance, including cap identity.
func (c SphereCylinderWallChart) EndpointEvent(i int) (SphereCylinderWallEvent, error) {
	if c.H == nil || i < 0 || i > 1 {
		return SphereCylinderWallEvent{}, ErrInvalid
	}
	return copySphereCylinderEvent(c.ends[i]), nil
}
func (c SphereCylinderWallChart) projectionValue(v SphereCylinderWallEvent) float64 {
	if v.ProjectionExtreme != 0 {
		h, _ := c.H.Float64()
		return float64(v.ProjectionExtreme) * math.Sqrt(h)
	}
	q, _ := v.ProjectionValue.Float64()
	return q
}
func (c SphereCylinderWallChart) numeric(t float64, derivative bool) (Vec3, error) {
	if c.H == nil || !finite(t) || t < 0 || t > 1 || derivative && (t == 0 || t == 1) {
		return Vec3{}, ErrDomain
	}
	h, _ := c.H.Float64()
	a, _ := c.A.Float64()
	lo, hi := c.projectionValue(c.ends[0]), c.projectionValue(c.ends[1])
	q := lo + (hi-lo)*t
	w := h - q*q
	d := a + 2*c.radius*q
	if w < 0 || d < 0 || !finite(h) || h <= 0 {
		return Vec3{}, ErrConvergence
	}
	orientation := 1.0
	if coordinate(c.axis, c.ax) < 0 {
		orientation = -1
	}
	side := -1.0
	if c.half == 1 {
		side = 1
	}
	j, k := (c.ax+1)%3, (c.ax+2)%3
	v := [3]float64{}
	for i := 0; i < 3; i++ {
		v[i], _ = c.projection[i].Float64()
	}
	cross := [3]float64{}
	cross[j] = -orientation * v[k]
	cross[k] = orientation * v[j]
	xyz := [3]float64{}
	if derivative {
		if w <= 0 || d <= 0 {
			return Vec3{}, ErrConvergence
		}
		for i := 0; i < 3; i++ {
			xyz[i] = c.radius * (v[i] - side*q/math.Sqrt(w)*cross[i]) / h * (hi - lo)
		}
		xyz[c.ax] = float64(c.sign) * c.radius * (hi - lo) / math.Sqrt(d)
	} else {
		for i := 0; i < 3; i++ {
			xyz[i] = coordinate(c.origin, i) + c.radius*(q*v[i]+side*math.Sqrt(w)*cross[i])/h
		}
		xyz[c.ax] = coordinate(c.center, c.ax) + float64(c.sign)*math.Sqrt(d)
	}
	result := Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	if !result.IsFinite() {
		return Vec3{}, ErrConvergence
	}
	return result, nil
}
func (c SphereCylinderWallChart) Eval(t float64) (Vec3, error)       { return c.numeric(t, false) }
func (c SphereCylinderWallChart) Derivative(t float64) (Vec3, error) { return c.numeric(t, true) }

// EndpointBounds encloses the exact model endpoint using rational nested
// square-root intervals. The graph proves each radicand nonnegative; negative
// lower interval tails are safely clamped to zero. Bounds are caller-owned.
func (c SphereCylinderWallChart) EndpointBounds(ctx context.Context, index int, o SurfacePatchEvaluationOptions) (out [3][2]*big.Rat, err error) {
	if ctx == nil || c.H == nil || index < 0 || index > 1 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	ev := c.ends[index]
	var qlo, qhi *big.Rat
	if ev.ProjectionExtreme != 0 {
		l, h, e := rationalSqrtBounds(c.H, 96, b)
		if e != nil {
			return out, e
		}
		qlo, qhi = l, h
		if ev.ProjectionExtreme < 0 {
			qlo, qhi = new(big.Rat).Neg(h), new(big.Rat).Neg(l)
		}
	} else {
		qlo, qhi = new(big.Rat).Set(ev.ProjectionValue), new(big.Rat).Set(ev.ProjectionValue)
	}
	orientation := int64(1)
	if coordinate(c.axis, c.ax) < 0 {
		orientation = -1
	}
	side := int64(-1)
	if c.half == 1 {
		side = 1
	}
	j, k := (c.ax+1)%3, (c.ax+2)%3
	cross := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	cross[j].Mul(big.NewRat(-orientation, 1), c.projection[k])
	cross[k].Mul(big.NewRat(orientation, 1), c.projection[j])
	wl, wh := new(big.Rat), new(big.Rat)
	if ev.ProjectionExtreme == 0 {
		rad := new(big.Rat).Sub(c.H, new(big.Rat).Mul(qlo, qlo))
		var e error
		wl, wh, e = rationalSqrtBounds(rad, 96, b)
		if e != nil {
			return out, e
		}
	}
	for i := 0; i < 3; i++ {
		terms := [][3]*big.Rat{{c.projection[i], qlo, qhi}, {new(big.Rat).Mul(big.NewRat(side, 1), cross[i]), wl, wh}}
		l, h := new(big.Rat), new(big.Rat)
		for _, term := range terms {
			a, z := term[1], term[2]
			if term[0].Sign() < 0 {
				a, z = z, a
			}
			l.Add(l, new(big.Rat).Mul(term[0], a))
			h.Add(h, new(big.Rat).Mul(term[0], z))
		}
		scale := new(big.Rat).Quo(r(c.radius), c.H)
		out[i] = [2]*big.Rat{new(big.Rat).Add(r(coordinate(c.origin, i)), new(big.Rat).Mul(scale, l)), new(big.Rat).Add(r(coordinate(c.origin, i)), new(big.Rat).Mul(scale, h))}
	}
	dl := new(big.Rat).Add(c.A, new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), r(c.radius)), qlo))
	dh := new(big.Rat).Add(c.A, new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), r(c.radius)), qhi))
	if ev.HeightSign == 0 {
		dl.SetInt64(0)
		dh.SetInt64(0)
	}
	if dl.Sign() < 0 {
		dl.SetInt64(0)
	}
	if dh.Sign() < 0 {
		return out, ErrConvergence
	}
	zl, _, e := rationalSqrtBounds(dl, 96, b)
	if e != nil {
		return out, e
	}
	_, zh, e := rationalSqrtBounds(dh, 96, b)
	if e != nil {
		return out, e
	}
	if c.sign < 0 {
		zl, zh = new(big.Rat).Neg(zh), new(big.Rat).Neg(zl)
	}
	out[c.ax] = [2]*big.Rat{new(big.Rat).Add(r(coordinate(c.center, c.ax)), zl), new(big.Rat).Add(r(coordinate(c.center, c.ax)), zh)}
	for _, v := range out {
		for _, q := range v {
			if e = b.check(q); e != nil {
				return [3][2]*big.Rat{}, e
			}
		}
	}
	return out, nil
}
