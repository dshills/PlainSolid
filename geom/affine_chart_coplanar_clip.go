package geom

import (
	"context"
	"math/big"
)

// CoplanarChartContactKind distinguishes the dimension of the closed footprint.
type CoplanarChartContactKind uint8

const (
	CoplanarChartsEmpty CoplanarChartContactKind = iota
	CoplanarChartsPoint
	CoplanarChartsSegment
	CoplanarChartsArea
)

// CoplanarAffineChartIntersection is an exact convex footprint intersection.
// Area vertices follow positive A-UV orientation; RelativeOrientation is the
// sign of B-UV orientation relative to A. Segment/point results claim no area.
// AreaA is original A-UV area; SquaredArea is squared physical model area.
// Every vertex retains both original UV coordinates and closed-side masks.
// Outputs are owned copies; no native shell or regularized volume is asserted.
type CoplanarAffineChartIntersection struct {
	Complete            bool
	Kind                CoplanarChartContactKind
	Vertices            []AffineChartIntersectionEndpoint
	RelativeOrientation int
	AreaA, SquaredArea  *big.Rat
	ExactOperations     int
}

// IntersectCoplanarAffineSurfaceCharts clips two complete affine chart rectangles
// without a preferred model projection. Original charts must prove the SAME
// supporting plane; noncoplanar input returns ErrDomain, never an empty claim.
// Rational clipping preserves nonbinary cuts. The polygon has at most eight
// extreme vertices. Limits are cumulative; zero selects100K operations/16384bits.
func IntersectCoplanarAffineSurfaceCharts(ctx context.Context, a, c *AffineSurfaceChart, o AffineChartIntersectionOptions) (r CoplanarAffineChartIntersection, err error) {
	if ctx == nil || a == nil || c == nil || a.determinant == nil || c.determinant == nil {
		return r, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 100000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = CoplanarAffineChartIntersection{}
		}
		r.ExactOperations = b.operations
	}()
	check := func(q *big.Rat) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		return b.check(q)
	}
	for _, chart := range []*AffineSurfaceChart{a, c} {
		for _, field := range [][3]*big.Rat{chart.origin, chart.u, chart.v} {
			for _, q := range field {
				if err = check(q); err != nil {
					return r, err
				}
			}
		}
	}
	type vertex struct {
		a, uv [2]*big.Rat
		p     [3]*big.Rat
	}
	polygon := make([]vertex, 4)
	for i, uv := range [][2]int64{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		polygon[i].a = [2]*big.Rat{big.NewRat(uv[0], 1), big.NewRat(uv[1], 1)}
		for axis := 0; axis < 3; axis++ {
			x := new(big.Rat).Mul(a.u[axis], polygon[i].a[0])
			if err = check(x); err != nil {
				return r, err
			}
			y := new(big.Rat).Mul(a.v[axis], polygon[i].a[1])
			if err = check(y); err != nil {
				return r, err
			}
			x.Add(x, y)
			if err = check(x); err != nil {
				return r, err
			}
			x.Add(x, a.origin[axis])
			if err = check(x); err != nil {
				return r, err
			}
			polygon[i].p[axis] = x
		}
		if b.operations >= o.MaxOperations {
			return r, ErrConvergence
		}
		inverse, e := c.InvertExactContext(ctx, polygon[i].p, AffineChartInverseOptions{MaxOperations: o.MaxOperations - b.operations, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += inverse.ExactOperations
		if e != nil {
			return r, e
		}
		if !inverse.OnPlane {
			return r, ErrDomain
		}
		polygon[i].uv = [2]*big.Rat{inverse.U, inverse.V}
	}
	area := func(p []vertex, target bool) (*big.Rat, error) {
		total := new(big.Rat)
		for i, v := range p {
			next := p[(i+1)%len(p)]
			x, y := v.a, next.a
			if target {
				x, y = v.uv, next.uv
			}
			left := new(big.Rat).Mul(x[0], y[1])
			if e := check(left); e != nil {
				return nil, e
			}
			right := new(big.Rat).Mul(x[1], y[0])
			if e := check(right); e != nil {
				return nil, e
			}
			left.Sub(left, right)
			if e := check(left); e != nil {
				return nil, e
			}
			total.Add(total, left)
			if e := check(total); e != nil {
				return nil, e
			}
		}
		return total, nil
	}
	orientation, e := area(polygon, true)
	if e != nil {
		return r, e
	}
	r.RelativeOrientation = orientation.Sign()
	if r.RelativeOrientation == 0 {
		return r, ErrDegenerate
	}
	bounds := [2][2]float64{{c.domain.UMin, c.domain.UMax}, {c.domain.VMin, c.domain.VMax}}
	equal := func(x, y vertex) bool { return x.a[0].Cmp(y.a[0]) == 0 && x.a[1].Cmp(y.a[1]) == 0 }
	for axis := 0; axis < 2; axis++ {
		for side := 0; side < 2; side++ {
			if len(polygon) == 0 {
				break
			}
			bound := new(big.Rat).SetFloat64(bounds[axis][side])
			out := []vertex{}
			appendVertex := func(v vertex) {
				if len(out) == 0 || !equal(out[len(out)-1], v) {
					out = append(out, v)
				}
			}
			inside := func(v vertex) bool {
				sign := v.uv[axis].Cmp(bound)
				return side == 0 && sign >= 0 || side == 1 && sign <= 0
			}
			previous := polygon[len(polygon)-1]
			previousInside := inside(previous)
			for _, current := range polygon {
				currentInside := inside(current)
				if previousInside != currentInside {
					numerator := new(big.Rat).Sub(bound, previous.uv[axis])
					if err = check(numerator); err != nil {
						return r, err
					}
					denominator := new(big.Rat).Sub(current.uv[axis], previous.uv[axis])
					if err = check(denominator); err != nil {
						return r, err
					}
					t := new(big.Rat).Quo(numerator, denominator)
					if err = check(t); err != nil {
						return r, err
					}
					interpolate := func(x, y *big.Rat) (*big.Rat, error) {
						q := new(big.Rat).Sub(y, x)
						if e := check(q); e != nil {
							return nil, e
						}
						q.Mul(q, t)
						if e := check(q); e != nil {
							return nil, e
						}
						q.Add(q, x)
						return q, check(q)
					}
					var v vertex
					for j := 0; j < 2; j++ {
						v.a[j], err = interpolate(previous.a[j], current.a[j])
						if err != nil {
							return r, err
						}
						v.uv[j], err = interpolate(previous.uv[j], current.uv[j])
						if err != nil {
							return r, err
						}
					}
					for j := 0; j < 3; j++ {
						v.p[j], err = interpolate(previous.p[j], current.p[j])
						if err != nil {
							return r, err
						}
					}
					appendVertex(v)
				}
				if currentInside {
					appendVertex(current)
				}
				previous, previousInside = current, currentInside
			}
			if len(out) > 1 && equal(out[0], out[len(out)-1]) {
				out = out[:len(out)-1]
			}
			if len(out) > 12 {
				return r, ErrConvergence
			}
			polygon = out
		}
	}
	twice, e := area(polygon, false)
	if e != nil {
		return r, e
	}
	if twice.Sign() < 0 {
		return r, ErrInvalid
	}
	r.Kind = CoplanarChartsArea
	if len(polygon) == 0 {
		r.Kind = CoplanarChartsEmpty
	} else if twice.Sign() == 0 {
		// A convex zero-area polygon is a segment or singleton. Keep its exact
		// lexicographic extremes, not duplicate traversal vertices.
		lo, hi := polygon[0], polygon[0]
		less := func(x, y vertex) bool {
			cmp := x.a[0].Cmp(y.a[0])
			return cmp < 0 || cmp == 0 && x.a[1].Cmp(y.a[1]) < 0
		}
		for _, v := range polygon {
			if less(v, lo) {
				lo = v
			}
			if less(hi, v) {
				hi = v
			}
		}
		polygon = []vertex{lo}
		r.Kind = CoplanarChartsPoint
		if !equal(lo, hi) {
			polygon = append(polygon, hi)
			r.Kind = CoplanarChartsSegment
		}
	}
	if len(polygon) > 8 {
		return r, ErrConvergence
	}
	normalizedArea := new(big.Rat).Quo(twice, big.NewRat(2, 1))
	if err = check(normalizedArea); err != nil {
		return r, err
	}
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(a.domain.UMax), new(big.Rat).SetFloat64(a.domain.UMin))
	height := new(big.Rat).Sub(new(big.Rat).SetFloat64(a.domain.VMax), new(big.Rat).SetFloat64(a.domain.VMin))
	r.AreaA = new(big.Rat).Mul(normalizedArea, width)
	if err = check(r.AreaA); err != nil {
		return r, err
	}
	r.AreaA.Mul(r.AreaA, height)
	if err = check(r.AreaA); err != nil {
		return r, err
	}
	gram := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		j, k := (axis+1)%3, (axis+2)%3
		x := new(big.Rat).Mul(a.u[j], a.v[k])
		if err = check(x); err != nil {
			return r, err
		}
		y := new(big.Rat).Mul(a.u[k], a.v[j])
		if err = check(y); err != nil {
			return r, err
		}
		x.Sub(x, y)
		if err = check(x); err != nil {
			return r, err
		}
		x.Mul(x, x)
		if err = check(x); err != nil {
			return r, err
		}
		gram.Add(gram, x)
		if err = check(gram); err != nil {
			return r, err
		}
	}
	r.SquaredArea = new(big.Rat).Mul(normalizedArea, normalizedArea)
	r.SquaredArea.Mul(r.SquaredArea, gram)
	if err = check(r.SquaredArea); err != nil {
		return r, err
	}
	mask := func(uv [2]*big.Rat, d ParameterRectangle) uint8 {
		var m uint8
		for axis, pair := range [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
			for side, v := range pair {
				if uv[axis].Cmp(new(big.Rat).SetFloat64(v)) == 0 {
					m |= 1 << uint(2*axis+side)
				}
			}
		}
		return m
	}
	for _, v := range polygon {
		ep := AffineChartIntersectionEndpoint{Point: v.p, B: v.uv}
		ep.A[0] = new(big.Rat).Add(new(big.Rat).SetFloat64(a.domain.UMin), new(big.Rat).Mul(v.a[0], width))
		ep.A[1] = new(big.Rat).Add(new(big.Rat).SetFloat64(a.domain.VMin), new(big.Rat).Mul(v.a[1], height))
		for _, q := range ep.A {
			if err = check(q); err != nil {
				return r, err
			}
		}
		ep.BoundaryA = mask(ep.A, a.domain)
		ep.BoundaryB = mask(ep.B, c.domain)
		r.Vertices = append(r.Vertices, ep)
	}
	r.Complete = true
	return r, nil
}
