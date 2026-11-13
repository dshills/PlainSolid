package trimvalidate

import (
	"math/big"

	"plainsolid/geom"
)

// A positive rational Bernstein curve with nondecreasing control projections
// and distinct endpoint projections is a strict graph. Grouping pairs in
// the quotient-rule numerator gives only nonnegative terms in the interior
// and at least one positive term. Endpoint speeds may vanish.
func monotoneProjection(c curve, axis point, strictControls bool) bool {
	p := c.points()
	x := dot(p[0], axis)
	for _, v := range p[1:] {
		y := dot(v, axis)
		if s := y.Cmp(x); s < 0 || s == 0 && strictControls {
			return false
		}
		x = y
	}
	return x.Cmp(dot(p[0], axis)) > 0
}

// Homogeneous refinement can prove monotonicity even when the original
// control polygon folds in projection. Every child must strictly increase
// on the original chord axis; a decreasing child invalidates this sufficient
// proof, without asserting that the geometric curve self-intersects.
func certifyBezierMonotone(c curve, axis point, b *budget) bool {
	stack := []curve{c}
	for len(stack) != 0 {
		if !b.tick() {
			return false
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if dot(minus(p.p[2], p.p[0]), axis).Sign() <= 0 {
			return false
		}
		if monotoneProjection(p, axis, false) {
			continue
		}
		left, right := b.split(p)
		if b.err != nil {
			return false
		}
		stack = append(stack, right, left)
	}
	return true
}

func (b *budget) splitBezier(c curve) (curve, curve) {
	n := len(c.controls)
	p := make([]point, n)
	w := append([]q(nil), c.weights...)
	for i, v := range c.controls {
		p[i] = scale(v, w[i])
	}
	left, right := c, c
	left.depth++
	right.depth++
	left.controls, right.controls = make([]point, n), make([]point, n)
	left.weights, right.weights = make([]q, n), make([]q, n)
	left.controls[0], right.controls[n-1] = c.p[0], c.p[2]
	left.weights[0], right.weights[n-1] = w[0], w[n-1]
	for j := 1; j < n; j++ {
		for i := 0; i < n-j; i++ {
			if !b.tick() {
				return curve{}, curve{}
			}
			p[i] = midpoint(p[i], p[i+1])
			w[i] = div(add(w[i], w[i+1]), big.NewRat(2, 1))
			b.check(p[i])
			b.check(point{w[i], w[i]})
		}
		k := n - 1 - j
		left.controls[j], right.controls[k] = scale(p[0], div(big.NewRat(1, 1), w[0])), scale(p[k], div(big.NewRat(1, 1), w[k]))
		left.weights[j], right.weights[k] = w[0], w[k]
		b.check(left.controls[j])
		b.check(right.controls[k])
		if b.err != nil {
			return curve{}, curve{}
		}
	}
	left.p[2], right.p[0] = left.controls[n-1], right.controls[0]
	return left, right
}

// The parent curve has a strict monotone projection, which remains monotone
// on every subdivided child. In coordinates x=(axis·(p-start))/(axis·d)
// and y=cross(d,p-start) with d the child's chord, the Jacobian determinant
// is one, x runs monotonically from 0 to 1, and y vanishes at the ends.
// The difference in twice-oriented area is -2∫y dx, bounded by the hull.
func bezierChordAreaBounds(c curve) (q, q) {
	start, end := c.p[0], c.p[2]
	d := minus(end, start)
	lo, hi := new(big.Rat), new(big.Rat)
	for _, p := range c.controls {
		y := cross(d, minus(p, start))
		if y.Cmp(lo) < 0 {
			lo = y
		}
		if y.Cmp(hi) > 0 {
			hi = y
		}
	}
	chord := cross(start, end)
	return sub(chord, mul(big.NewRat(2, 1), hi)), sub(chord, mul(big.NewRat(2, 1), lo))
}

func (b *budget) locateBezierPoint(query point, c curve) (hit, complete bool) {
	if b.o.MaxBits < 64 || b.o.MaxWork-b.work < 1 || b.o.MaxSubdivisions-b.subdivisions < 1 {
		return false, false
	}
	h := make([][4]*big.Rat, len(c.controls))
	for i, p := range c.controls {
		w := c.weights[i]
		h[i] = [4]*big.Rat{mul(p[0], w), mul(p[1], w), new(big.Rat), w}
	}
	o := geom.SurfacePatchOptions{MaxOperations: b.o.MaxWork - b.work, MaxCoefficientBits: b.o.MaxBits}
	exact, e := geom.NewExactHomogeneousBezier(b.ctx, h, o)
	if e != nil {
		if b.ctx.Err() != nil {
			b.err = b.ctx.Err()
		}
		return false, false
	}
	b.work += exact.ExactOperations()
	if b.o.MaxWork-b.work < 1 {
		return false, false
	}
	p, e := geom.FindExactCurvePointParameters(b.ctx, exact, [3]*big.Rat{query[0], query[1], new(big.Rat)},
		geom.ExactCurveSegmentOptions{MaxOperations: b.o.MaxWork - b.work, MaxCoefficientBits: b.o.MaxBits,
			MaxSubdivisions: b.o.MaxSubdivisions - b.subdivisions})
	b.work += min(p.Operations, b.o.MaxWork-b.work)
	b.subdivisions += min(p.Subdivisions, b.o.MaxSubdivisions-b.subdivisions)
	if b.ctx.Err() != nil {
		b.err = b.ctx.Err()
	}
	if e != nil || !p.Complete {
		return false, false
	}
	return len(p.Roots) != 0 || len(p.ConstantSpans) != 0, true
}
