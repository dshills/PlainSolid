package trimvalidate

import (
	"container/heap"
	"context"
	"math/big"
)

// SignedAreaInterval encloses the sum of the signed Euclidean areas of the
// certified rings. Holes with clockwise traversal contribute negatively.
// Bounds are caller-owned. Width is at most the requested exact tolerance.
type SignedAreaInterval struct {
	Lower, Upper       *big.Rat
	Work, Subdivisions int
}

// BoundSignedArea adds exact polynomial Green integrals or rational control
// hull enclosures. It refines the highest-uncertainty rational arc first until
// the exact total interval meets maxWidth. It is valid with arbitrary ring
// signs and does not interpret total winding as a union for same-sign nesting.
// Each query has its own proof budget. Failure does not expose partial bounds.
func (c *CertifiedLoops) BoundSignedArea(ctx context.Context, maxWidth *big.Rat) (SignedAreaInterval, error) {
	var zero SignedAreaInterval
	if ctx == nil || c == nil || len(c.rings) == 0 && !c.forest || maxWidth == nil || maxWidth.Sign() <= 0 {
		return zero, ErrOptions
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	b := &budget{ctx: ctx, o: c.options}
	b.check(point{maxWidth, maxWidth})
	if b.err != nil {
		return zero, b.err
	}
	lo, hi := new(big.Rat), new(big.Rat)
	var uncertain areaHeap
	index := 0
	include := func(c curve) bool {
		if !b.tick() {
			return false
		}
		a, z := exactOrBoundedCurveArea(c, b)
		if b.err != nil {
			return false
		}
		lo.Add(lo, a)
		hi.Add(hi, z)
		b.check(point{lo, hi})
		if b.err != nil {
			return false
		}
		if width := sub(z, a); width.Sign() > 0 {
			heap.Push(&uncertain, boundedAreaPiece{c, a, z, width, index})
			index++
		}
		return true
	}
	for _, a := range c.edges {
		if !include(a) {
			return zero, b.err
		}
	}
	width := new(big.Rat).Mul(maxWidth, big.NewRat(2, 1))
	for sub(hi, lo).Cmp(width) > 0 {
		if uncertain.Len() == 0 {
			return zero, ErrLimit
		}
		a := heap.Pop(&uncertain).(boundedAreaPiece)
		lo.Sub(lo, a.lo)
		hi.Sub(hi, a.hi)
		p, q := b.split(a.c)
		if b.err != nil {
			return zero, b.err
		}
		if !include(p) || !include(q) {
			return zero, b.err
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return SignedAreaInterval{Lower: div(lo, big.NewRat(2, 1)), Upper: div(hi, big.NewRat(2, 1)), Work: b.work, Subdivisions: b.subdivisions}, nil
}

func exactOrBoundedCurveArea(c curve, b *budget) (q, q) {
	if c.controls == nil {
		return areaBounds(c)
	}
	for _, w := range c.weights[1:] {
		if w.Cmp(c.weights[0]) != 0 {
			return areaBounds(c)
		}
	}
	// Polynomial higher-degree Bézier: multiply its power coefficients,
	// differentiate and integrate ∫cross(P,P') exactly.
	n := len(c.controls) - 1
	p := make([]point, n+1)
	for k := range p {
		p[k] = point{new(big.Rat), new(big.Rat)}
		for i := 0; i <= k; i++ {
			if !b.tick() {
				return nil, nil
			}
			v := new(big.Int).Mul(new(big.Int).Binomial(int64(n), int64(i)), new(big.Int).Binomial(int64(n-i), int64(k-i)))
			if (k-i)%2 != 0 {
				v.Neg(v)
			}
			p[k] = plus(p[k], scale(c.controls[i], new(big.Rat).SetInt(v)))
			b.check(p[k])
			if b.err != nil {
				return nil, nil
			}
		}
	}
	a := new(big.Rat)
	for i := 0; i <= n; i++ {
		for j := 1; j <= n; j++ {
			if !b.tick() {
				return nil, nil
			}
			a.Add(a, mul(cross(p[i], p[j]), big.NewRat(int64(j), int64(i+j))))
			b.check(point{a, a})
			if b.err != nil {
				return nil, nil
			}
		}
	}
	return a, a
}

type boundedAreaPiece struct {
	c             curve
	lo, hi, width q
	index         int
}
type areaHeap []boundedAreaPiece

func (q areaHeap) Len() int { return len(q) }
func (q areaHeap) Less(i, j int) bool {
	if order := q[i].width.Cmp(q[j].width); order != 0 {
		return order > 0
	}
	return q[i].index < q[j].index
}
func (q areaHeap) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *areaHeap) Push(x any)   { *q = append(*q, x.(boundedAreaPiece)) }
func (q *areaHeap) Pop() any {
	p := *q
	n := len(p) - 1
	a := p[n]
	p[n] = boundedAreaPiece{}
	*q = p[:n]
	return a
}
