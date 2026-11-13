package geom

import (
	"context"
	"math"
	"math/big"
)

// SurfaceArcUVOptions controls the parameter-only exact interval solver.
// Zero fields select four contractions, 1M operations and 16384 coefficient bits.
type SurfaceArcUVOptions struct{ MaxIterations, MaxOperations, MaxCoefficientBits int }

// SurfaceArcUVEncloser binds a scalar Bernstein plane residual to one regular
// arc. An enclosure covers all its points in a given free-parameter slab.
// It is immutable and safe for concurrent queries.
type SurfaceArcUVEncloser struct {
	net         []*big.Rat
	domain      ParameterRectangle
	nu, nv, dep int
	options     SurfaceArcUVOptions
}

// NewSurfaceArcUVEncloser extracts the homogeneous plane residual over a
// certified single knot cell and independently checks its dependent derivative.
func (a SurfacePlaneArc) NewSurfaceArcUVEncloser(ctx context.Context, o SurfaceArcUVOptions) (*SurfaceArcUVEncloser, error) {
	p, dep := a.source, a.Certificate.FixedParameterAxis
	if ctx == nil || p == nil || dep < 0 || dep > 1 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return nil, ErrInvalid
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 4
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxIterations < 1 || o.MaxIterations > 256 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, ErrInvalid
	}
	q := &SurfaceArcUVEncloser{net: make([]*big.Rat, len(p.net)), domain: p.Domain(), nu: p.countU, nv: p.countV, dep: dep, options: o}
	b := q.budget()
	equation := exactPlaneCoefficients(a.plane)
	for i, h := range p.net {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		q.net[i] = exactPlaneHomogeneousValue(equation, h)
		if e := b.check(q.net[i]); e != nil {
			return nil, e
		}
	}
	d, e := scalarPartial(q.net, q.nu, q.nv, dep == 0, q.domain, b)
	if e != nil {
		return nil, e
	}
	if coefficientRange(d).sign() == 0 {
		return nil, ErrConvergence
	}
	return q, nil
}

func (q *SurfaceArcUVEncloser) budget() *polynomialBudget {
	return &polynomialBudget{maxOperations: q.options.MaxOperations, maxBits: q.options.MaxCoefficientBits}
}

func scalarBezierChildren(points []*big.Rat, t *big.Rat, b *polynomialBudget) ([]*big.Rat, []*big.Rat, error) {
	n := len(points)
	work := append([]*big.Rat(nil), points...)
	left, right := make([]*big.Rat, n), make([]*big.Rat, n)
	left[0], right[n-1] = work[0], work[n-1]
	u := new(big.Rat).Sub(big.NewRat(1, 1), t)
	for level := 1; level < n; level++ {
		for i := 0; i < n-level; i++ {
			v := new(big.Rat).Add(new(big.Rat).Mul(u, work[i]), new(big.Rat).Mul(t, work[i+1]))
			if e := b.check(v); e != nil {
				return nil, nil, e
			}
			work[i] = v
		}
		left[level], right[n-level-1] = work[0], work[n-level-1]
	}
	return left, right, nil
}

func (q *SurfaceArcUVEncloser) restrict(net []*big.Rat, axis int, lo, hi, spanLo, spanHi float64, b *polynomialBudget) ([]*big.Rat, error) {
	if lo == spanLo && hi == spanHi {
		return net, nil
	}
	exact := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	a, z := exact(spanLo), exact(spanHi)
	x, y := exact(lo), exact(hi)
	// Split at the upper endpoint then at the relative lower endpoint.
	upper := new(big.Rat).Quo(new(big.Rat).Sub(y, a), new(big.Rat).Sub(z, a))
	lower := new(big.Rat).Quo(new(big.Rat).Sub(x, a), new(big.Rat).Sub(y, a))
	count, rows, stride := q.nu, q.nv, q.nv
	if axis == 1 {
		count, rows, stride = q.nv, q.nu, 1
	}
	result, work := make([]*big.Rat, len(net)), make([]*big.Rat, count)
	for j := 0; j < rows; j++ {
		base := j
		if axis == 1 {
			base *= q.nv
		}
		for i := range work {
			work[i] = net[base+i*stride]
		}
		values := work
		if hi != spanHi {
			var e error
			values, _, e = scalarBezierChildren(values, upper, b)
			if e != nil {
				return nil, e
			}
		}
		if lo != spanLo {
			var e error
			_, values, e = scalarBezierChildren(values, lower, b)
			if e != nil {
				return nil, e
			}
		}
		for i, value := range values {
			result[base+i*stride] = value
		}
	}
	return result, nil
}

// Enclose bounds the exact free slab in UV. A false result with nil error
// proves the slab misses the source cell or excludes every residual root.
// If contraction is exhausted after making progress, the retained box is
// still conservative. This API does not compute an enclosure in model space.
func (q *SurfaceArcUVEncloser) Enclose(ctx context.Context, from, to float64) (UVBox, bool, error) {
	var zero UVBox
	if ctx == nil || q == nil || len(q.net) == 0 || !finite(from) || !finite(to) || from >= to {
		return zero, false, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, false, e
	}
	d := q.domain
	slabs := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	free := 1 - q.dep
	lo, hi := max(from, slabs[free][0]), min(to, slabs[free][1])
	if lo > hi {
		return zero, false, nil
	}
	var b = q.budget()
	box := func() UVBox { return UVBox{Vec2{slabs[0][0], slabs[1][0]}, Vec2{slabs[0][1], slabs[1][1]}} }
	if lo == hi {
		slabs[free] = [2]float64{lo, hi}
		return box(), true, nil
	}
	values, e := q.restrict(q.net, free, lo, hi, slabs[free][0], slabs[free][1], b)
	slabs[free] = [2]float64{lo, hi}
	if e != nil {
		return box(), true, e
	}
	lo, hi = slabs[q.dep][0], slabs[q.dep][1]
	for range q.options.MaxIterations {
		if e = ctx.Err(); e != nil {
			return box(), true, e
		}
		if coefficientRange(values).sign() != 0 {
			return zero, false, nil
		}
		d = ParameterRectangle{UMin: slabs[0][0], UMax: slabs[0][1], VMin: slabs[1][0], VMax: slabs[1][1]}
		derivative, e := scalarPartial(values, q.nu, q.nv, q.dep == 0, d, b)
		if e != nil {
			return box(), true, e
		}
		dr := coefficientRange(derivative)
		if dr.sign() == 0 {
			return box(), true, ErrConvergence
		}
		half := big.NewRat(1, 2)
		at, e := restrictedScalarRange(values, q.nu, q.nv, q.dep, half, b)
		if e != nil {
			return box(), true, e
		}
		step, e := intervalDivide(at, dr, b)
		if e != nil {
			return box(), true, e
		}
		center := new(big.Rat).Mul(half, new(big.Rat).Add(new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)))
		image, e := intervalSubtract(exactInterval{center, center}, step, b)
		if e != nil {
			return box(), true, e
		}
		x, y := max(lo, rationalFloatBound(image.lo, true)), min(hi, rationalFloatBound(image.hi, false))
		if x > y {
			return zero, false, nil
		}
		if x == y {
			x, y = max(lo, math.Nextafter(x, math.Inf(-1))), min(hi, math.Nextafter(y, math.Inf(1)))
		}
		if x == y || x == lo && y == hi {
			break
		}
		values, e = q.restrict(values, q.dep, x, y, lo, hi, b)
		slabs[q.dep], lo, hi = [2]float64{x, y}, x, y
		if e != nil {
			return box(), true, e
		}
	}
	return box(), true, nil
}
