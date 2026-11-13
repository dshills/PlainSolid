package geom

import (
	"context"
	"math/big"
)

// ExactQuadraticArcHit contains a source-curve contact and the corresponding
// original quadratic arc parameter enclosure. ArcParameter is present only for
// exact rational recovery; algebraic contacts retain proved bounds on both.
type ExactQuadraticArcHit struct {
	Curve                            ExactCurveQuadricHit
	ArcLower, ArcUpper, ArcParameter *big.Rat
}

// ExactQuadraticArcOverlap proves a whole source span equals a bounded arc
// subinterval under ArcParameter=ArcOffset+ArcScale*CurveParameter.
// ArcStart/ArcEnd follow source traversal; a negative scale reverses the arc.
type ExactQuadraticArcOverlap struct {
	CurveSpan                             ParameterSpan
	ArcStart, ArcEnd, ArcOffset, ArcScale *big.Rat
}

// ExactQuadraticArcIntersection is exhaustive only when Complete. Pending
// preserves support-conic roots whose bounded arc membership is unproved.
// Overlaps carry exact affine parameter correspondences. Remaining Coincident
// spans lie on the support conic but their bounded overlap remains unresolved.
type ExactQuadraticArcIntersection struct {
	Hits                     []ExactQuadraticArcHit
	Overlaps                 []ExactQuadraticArcOverlap
	Pending                  []ExactCurveQuadricHit
	Coincident               []ParameterSpan
	Complete                 bool
	Operations, Subdivisions int
}

// IntersectExactCurveQuadraticArc requires arc to be a single rational quadratic
// Bézier span with three noncollinear poles and positive weights. Exact affine
// barycentrics L_i of its poles satisfy w0*w2*L1²=4*w1²*L0*L2. Positive
// barycentrics select the bounded arc; its inverse is
// s=w0*L1/(2*w1*L0+w0*L1). All tests use retained homogeneous coefficients.
// Algebraic roots whose interval control hull cannot prove strict membership or
// exclusion remain Pending/ErrConvergence. No sampled tolerance decides contact.
func IntersectExactCurveQuadraticArc(ctx context.Context, curve, arc *ExactSplineCurve, o ExactCurveSegmentOptions) (r ExactQuadraticArcIntersection, err error) {
	if ctx == nil || curve == nil || arc == nil || curve.patch == nil || arc.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	if arc.patch.degreeU != 2 || arc.patch.countU != 3 {
		return r, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations += b.operations }()
	var points [3][3]*big.Rat
	var weights [3]*big.Rat
	for i := 0; i < 3; i++ {
		h := arc.patch.net[i*arc.patch.countV]
		weights[i] = h[3]
		for _, x := range h {
			if e := b.check(x); e != nil {
				return r, e
			}
		}
		for axis := 0; axis < 3; axis++ {
			points[i][axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := b.check(points[i][axis]); e != nil {
				return r, e
			}
		}
	}
	e1, e2 := [3]*big.Rat{}, [3]*big.Rat{}
	for axis := 0; axis < 3; axis++ {
		e1[axis] = new(big.Rat).Sub(points[1][axis], points[0][axis])
		e2[axis] = new(big.Rat).Sub(points[2][axis], points[0][axis])
	}
	normal := [3]*big.Rat{}
	for axis := 0; axis < 3; axis++ {
		i, j := (axis+1)%3, (axis+2)%3
		normal[axis] = new(big.Rat).Sub(new(big.Rat).Mul(e1[i], e2[j]), new(big.Rat).Mul(e1[j], e2[i]))
		if e := b.check(normal[axis]); e != nil {
			return r, e
		}
	}
	pivot := -1
	for axis := 0; axis < 3; axis++ {
		if normal[axis].Sign() != 0 {
			pivot = axis
			break
		}
	}
	if pivot < 0 {
		return r, ErrDegenerate
	}
	i, j := (pivot+1)%3, (pivot+2)%3
	det := normal[pivot]
	var linear [3][4]*big.Rat
	for k := 0; k < 3; k++ {
		for axis := 0; axis < 4; axis++ {
			linear[k][axis] = new(big.Rat)
		}
	}
	linear[1][i].Quo(e2[j], det)
	linear[1][j].Quo(new(big.Rat).Neg(e2[i]), det)
	linear[2][i].Quo(new(big.Rat).Neg(e1[j]), det)
	linear[2][j].Quo(e1[i], det)
	for k := 1; k < 3; k++ {
		for axis := 0; axis < 3; axis++ {
			linear[k][3].Sub(linear[k][3], new(big.Rat).Mul(linear[k][axis], points[0][axis]))
		}
	}
	for axis := 0; axis < 4; axis++ {
		linear[0][axis].Neg(new(big.Rat).Add(linear[1][axis], linear[2][axis]))
	}
	linear[0][3].Add(linear[0][3], big.NewRat(1, 1))
	plane := [4]*big.Rat{normal[0], normal[1], normal[2], new(big.Rat)}
	for axis := 0; axis < 3; axis++ {
		plane[3].Sub(plane[3], new(big.Rat).Mul(normal[axis], points[0][axis]))
	}
	products := [10][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}, {0, 3}, {1, 3}, {2, 3}, {3, 3}}
	var quadric ExactQuadric
	first := new(big.Rat).Mul(weights[0], weights[2])
	second := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(weights[1], weights[1]))
	for term, pair := range products {
		u, v := pair[0], pair[1]
		x := new(big.Rat).Mul(first, new(big.Rat).Mul(linear[1][u], linear[1][v]))
		y := new(big.Rat).Mul(second, new(big.Rat).Mul(linear[0][u], linear[2][v]))
		if u != v {
			x.Mul(x, big.NewRat(2, 1))
			y.Add(y, new(big.Rat).Mul(second, new(big.Rat).Mul(linear[0][v], linear[2][u])))
		}
		quadric[term] = new(big.Rat).Sub(x, y)
		if e := b.check(quadric[term]); e != nil {
			return r, e
		}
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	local := o
	local.MaxOperations = remaining
	support, e := IntersectExactCurveQuadric(ctx, curve, quadric, ExactCurveQuadricOptions{ExactCurveSegmentOptions: local, Plane: &plane})
	r.Operations = support.Operations
	r.Subdivisions = support.Subdivisions
	r.Coincident = support.Coincident
	if e != nil {
		return r, e
	}
	b.maxOperations = o.MaxOperations - support.Operations
	arcLo, arcHi := arc.Domain()
	mapArc := func(s *big.Rat) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).SetFloat64(arcLo), new(big.Rat).Mul(new(big.Rat).Sub(new(big.Rat).SetFloat64(arcHi), new(big.Rat).SetFloat64(arcLo)), s))
	}
	evaluateLinear := func(h exactHomogeneous, k int) (*big.Rat, error) {
		v := new(big.Rat)
		for axis := 0; axis < 4; axis++ {
			v.Add(v, new(big.Rat).Mul(linear[k][axis], h[axis]))
			if e := b.check(v); e != nil {
				return nil, e
			}
		}
		return v, nil
	}
	// A supporting-conic span is an exact overlap only after bounded membership
	// and its whole parameter correspondence have been verified.
	r.Coincident = nil
	for _, span := range support.Coincident {
		overlap, ok, e := certifyQuadraticArcOverlap(ctx, curve, span, linear, weights, arcLo, arcHi, b)
		if e != nil {
			return r, e
		}
		if ok {
			r.Overlaps = append(r.Overlaps, overlap)
		} else {
			r.Coincident = append(r.Coincident, span)
		}
	}
	for _, hit := range support.Hits {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if hit.Parameter != nil {
			h := exactHomogeneous{hit.Point[0], hit.Point[1], hit.Point[2], big.NewRat(1, 1)}
			var lambda [3]*big.Rat
			outside := false
			for k := 0; k < 3; k++ {
				lambda[k], e = evaluateLinear(h, k)
				if e != nil {
					return r, e
				}
				outside = outside || lambda[k].Sign() < 0
			}
			if outside {
				continue
			}
			numerator := new(big.Rat).Mul(weights[0], lambda[1])
			denominator := new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), weights[1]), lambda[0]), numerator)
			var s *big.Rat
			if denominator.Sign() == 0 {
				if lambda[0].Sign() == 0 && lambda[1].Sign() == 0 && lambda[2].Sign() > 0 {
					s = big.NewRat(1, 1)
				} else {
					r.Pending = append(r.Pending, hit)
					continue
				}
			} else {
				s = new(big.Rat).Quo(numerator, denominator)
			}
			if s.Sign() < 0 || s.Cmp(big.NewRat(1, 1)) > 0 {
				continue
			}
			parameter := mapArc(s)
			if e := b.check(parameter); e != nil {
				return r, e
			}
			point, e := exactCurveRationalPoint(arc.patch, parameter, b)
			if e != nil {
				return r, e
			}
			same := true
			for axis := 0; axis < 3; axis++ {
				same = same && point[axis].Cmp(hit.Point[axis]) == 0
			}
			if !same {
				r.Pending = append(r.Pending, hit)
				continue
			}
			r.Hits = append(r.Hits, ExactQuadraticArcHit{Curve: hit, ArcParameter: parameter, ArcLower: new(big.Rat).Set(parameter), ArcUpper: new(big.Rat).Set(parameter)})
			continue
		}
		// Restrict homogeneous source coefficients to the algebraic root bracket,
		// using rational de Casteljau rather than rounded parameter knots.
		p, e := trimPatchBudget(curve.patch, ParameterRectangle{hit.Span.Start, hit.Span.End, 0, 1}, b)
		if e != nil {
			return r, e
		}
		net := make([]exactHomogeneous, p.countU)
		for k := range net {
			net[k] = p.net[k*p.countV]
		}
		origin, width := new(big.Rat).SetFloat64(hit.Span.Start), new(big.Rat).Sub(new(big.Rat).SetFloat64(hit.Span.End), new(big.Rat).SetFloat64(hit.Span.Start))
		lower, upper := new(big.Rat).Quo(new(big.Rat).Sub(hit.Lower, origin), width), new(big.Rat).Quo(new(big.Rat).Sub(hit.Upper, origin), width)
		net, e = restrictHomogeneousBezier(net, lower, upper, b)
		if e != nil {
			return r, e
		}
		values := [3][]*big.Rat{}
		inside, outside := true, false
		for k := 0; k < 3; k++ {
			values[k] = make([]*big.Rat, len(net))
			allNegative := true
			for index, h := range net {
				values[k][index], e = evaluateLinear(h, k)
				if e != nil {
					return r, e
				}
				inside = inside && values[k][index].Sign() > 0
				allNegative = allNegative && values[k][index].Sign() < 0
			}
			outside = outside || allNegative
		}
		if outside {
			continue
		}
		if !inside {
			r.Pending = append(r.Pending, hit)
			continue
		}
		var smin, smax *big.Rat
		for index := range net {
			n := new(big.Rat).Mul(weights[0], values[1][index])
			d := new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), weights[1]), values[0][index]), n)
			s := new(big.Rat).Quo(n, d)
			if e := b.check(s); e != nil {
				return r, e
			}
			if smin == nil || s.Cmp(smin) < 0 {
				smin = s
			}
			if smax == nil || s.Cmp(smax) > 0 {
				smax = s
			}
		}
		arcLower, arcUpper := mapArc(smin), mapArc(smax)
		if e := b.check(arcLower); e != nil {
			return r, e
		}
		if e := b.check(arcUpper); e != nil {
			return r, e
		}
		r.Hits = append(r.Hits, ExactQuadraticArcHit{Curve: hit, ArcLower: arcLower, ArcUpper: arcUpper})
	}
	r.Complete = len(r.Pending) == 0 && len(r.Coincident) == 0
	if !r.Complete {
		return r, ErrConvergence
	}
	return r, nil
}
