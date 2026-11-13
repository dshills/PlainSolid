package geom

import (
	"fmt"
	"sort"
)

// NewBezier constructs a polynomial or rational Bezier curve on [0,1].
// Degree is len(poles)-1; nil weights specify polynomial geometry.
func NewBezier(poles []Vec3, weights []float64) (*BSpline, error) {
	if len(poles) < 2 || len(poles) > 65 {
		return nil, ErrInvalid
	}
	knots := make([]float64, 2*len(poles))
	for i := len(poles); i < len(knots); i++ {
		knots[i] = 1
	}
	return NewBSpline(len(poles)-1, poles, knots, weights)
}

// InsertKnot returns an independent curve with the same geometry and domain.
// t must be strictly interior. count is nonnegative, and resulting multiplicity
// cannot exceed degree+1. End clamping is deliberately a separate operation.
func (c *BSpline) InsertKnot(t float64, count int) (*BSpline, error) {
	if !c.validParameter(t) || count < 0 {
		return nil, ErrDomain
	}
	a, b := c.Domain()
	if t <= a || t >= b {
		return nil, ErrDomain
	}
	start := sort.SearchFloat64s(c.knots, t)
	end := sort.Search(len(c.knots), func(i int) bool { return c.knots[i] > t })
	if count > c.degree+1-(end-start) {
		return nil, fmt.Errorf("%w: knot multiplicity", ErrInvalid)
	}
	p := toHom(c.poles, c.weights)
	knots := append([]float64(nil), c.knots...)
	for r := 0; r < count; r++ {
		k := sort.Search(len(knots), func(i int) bool { return knots[i] > t }) - 1
		s := k - sort.SearchFloat64s(knots, t) + 1
		q := make([]homogeneous, len(p)+1)
		for i := 0; i <= k-c.degree; i++ {
			q[i] = p[i]
		}
		for i := k - s; i < len(p); i++ {
			q[i+1] = p[i]
		}
		for i := k - c.degree + 1; i <= k-s; i++ {
			alpha := (t - knots[i]) / (knots[i+c.degree] - knots[i])
			for j := 0; j < 4; j++ {
				q[i][j] = (1-alpha)*p[i-1][j] + alpha*p[i][j]
			}
		}
		next := make([]float64, len(knots)+1)
		copy(next, knots[:k+1])
		next[k+1] = t
		copy(next[k+2:], knots[k+1:])
		p, knots = q, next
	}
	poles := make([]Vec3, len(p))
	w := make([]float64, len(p))
	for i, h := range p {
		point, e := projectHom(h)
		if e != nil {
			return nil, e
		}
		poles[i], w[i] = point, h[3]
	}
	return NewBSpline(c.degree, poles, knots, w)
}

// Split returns two independent curves retaining the original parameters.
// At an existing discontinuity the left curve ends at the left limit and the
// right curve starts at the right limit. t must lie strictly inside the domain.
func (c *BSpline) Split(t float64) (left, right *BSpline, err error) {
	if !c.validParameter(t) {
		return nil, nil, ErrDomain
	}
	a, b := c.Domain()
	if t <= a || t >= b {
		return nil, nil, ErrDomain
	}
	first := sort.SearchFloat64s(c.knots, t)
	end := sort.Search(len(c.knots), func(i int) bool { return c.knots[i] > t })
	refined, e := c.InsertKnot(t, c.degree+1-(end-first))
	if e != nil {
		return nil, nil, e
	}
	first = sort.SearchFloat64s(refined.knots, t)
	left, e = NewBSpline(c.degree, refined.poles[:first], refined.knots[:first+c.degree+1], refined.weights[:first])
	if e != nil {
		return nil, nil, e
	}
	right, e = NewBSpline(c.degree, refined.poles[first:], refined.knots[first:], refined.weights[first:])
	if e != nil {
		return nil, nil, e
	}
	return left, right, nil
}
