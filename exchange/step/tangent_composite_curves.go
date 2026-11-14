package step

import (
	"io"
	"math/big"
	"plainsolid/geom"
)

// TangentCompositeCurveModel retains independently checked stronger transitions.
// TangentTransitions[i][j] describes the join after segment j of curve i.
// True requires exact equality of nonzero directed endpoint derivatives in the
// original spline parameters, a conservative subset of geometric tangency.
// False retains positional CONTINUOUS (or terminal open DISCONTINUOUS).
type TangentCompositeCurveModel struct {
	CompositeCurveModel
	TangentTransitions [][]bool
}

// ReadTangentCompositeCurves adds only proved CONT_SAME_GRADIENT declarations.
// Original positional-only ReadCompositeCurves continues to reject them.
func ReadTangentCompositeCurves(r io.Reader, l Limits) (TangentCompositeCurveModel, error) {
	var flags [][]bool
	m, e := readCompositeCurves(r, l, &flags)
	if e != nil {
		return TangentCompositeCurveModel{}, e
	}
	return TangentCompositeCurveModel{m, flags}, nil
}

// WriteTangentCompositeCurves rechecks each declared derivative identity before
// output. Unequal parameter speeds, zero tangents and curvature claims are not
// supported by this deliberately bounded C1 gate.
func WriteTangentCompositeCurves(w io.Writer, m TangentCompositeCurveModel) error {
	if m.TangentTransitions == nil {
		return compositeFailure()
	}
	return writeCompositeCurves(w, m.CompositeCurveModel, m.TangentTransitions)
}
func compositeEndpointDerivative(s CompositeSplineSegment, end bool) ([3]*big.Rat, error) {
	var out [3]*big.Rat
	if s.Curve.Geometry == nil {
		return out, compositeFailure()
	}
	g := s.Curve.Geometry
	p, k, w := g.Poles(), g.Knots(), g.Weights()
	n := len(p) - 1
	degree := g.Degree()
	if n < 1 || len(w) != len(p) || len(k) != len(p)+degree+1 {
		return out, compositeFailure()
	}
	a, b := g.Domain()
	i, j := 0, 1
	span := new(big.Rat).Sub(sideRat(k[degree+1]), sideRat(a))
	ratio := new(big.Rat).Quo(sideRat(w[1]), sideRat(w[0]))
	if end == s.SameSense {
		i, j = n-1, n
		span.Sub(sideRat(b), sideRat(k[n]))
		ratio.Quo(sideRat(w[n-1]), sideRat(w[n]))
	}
	if span.Sign() <= 0 {
		return out, compositeFailure()
	}
	factor := new(big.Rat).Mul(big.NewRat(int64(degree), 1), new(big.Rat).Quo(ratio, span))
	if !s.SameSense {
		factor.Neg(factor)
	}
	zero := true
	for axis, pair := range [][2]float64{{p[i].X, p[j].X}, {p[i].Y, p[j].Y}, {p[i].Z, p[j].Z}} {
		out[axis] = new(big.Rat).Mul(new(big.Rat).Sub(sideRat(pair[1]), sideRat(pair[0])), factor)
		if out[axis].Sign() != 0 {
			zero = false
		}
		if out[axis].Num().BitLen() > 4096 || out[axis].Denom().BitLen() > 4096 {
			return [3]*big.Rat{}, geom.ErrConvergence
		}
	}
	if zero {
		return [3]*big.Rat{}, compositeFailure()
	}
	return out, nil
}
func validateCompositeTangents(c CompositeSplineCurve, flags []bool, geometric ...bool) error {
	if e := validateCompositeCurve(c); e != nil {
		return e
	}
	if len(flags) != len(c.Segments) {
		return compositeFailure()
	}
	for i, strong := range flags {
		if !strong {
			continue
		}
		next := i + 1
		if next == len(c.Segments) {
			if !c.Closed {
				return compositeFailure()
			}
			next = 0
		}
		a, e := compositeEndpointDerivative(c.Segments[i], true)
		if e != nil {
			return e
		}
		b, e := compositeEndpointDerivative(c.Segments[next], false)
		if e != nil {
			return e
		}
		if len(geometric) != 0 && geometric[0] {
			if !positiveProportionalCompositeTangents(a, b) {
				return compositeFailure()
			}
			continue
		}
		for j := range a {
			if a[j].Cmp(b[j]) != 0 {
				return compositeFailure()
			}
		}
	}
	return nil
}
