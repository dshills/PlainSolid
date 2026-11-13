package step

import (
	"io"
	"math/big"
	"plainsolid/geom"
)

// PolynomialCylinderResidual bounds the cylinder equation on complete source
// knot spans. RadialDistanceUpper is one-sided distance to the infinite support,
// not to any bounded face or to the intersection of two supports.
type PolynomialCylinderResidual struct {
	Spans                                    []PolynomialSphereResidualSpan
	MaxAbsoluteResidual, RadialDistanceUpper *big.Rat
}

// BoundPolynomialSplineCylinderResidual bounds |d|²-(d·axis)²/|axis|²-radius²,
// d=C(t)-origin, using exact rational arithmetic. Axis is any finite nonzero
// represented vector; no normalization or square-root rounding is introduced.
// It shares the polynomial source and resource restrictions of the sphere
// residual API. Small residuals do not establish trim ownership or matching.
func BoundPolynomialSplineCylinderResidual(r io.Reader, l Limits, curveID int64, origin, axis geom.Vec3, radius float64) (PolynomialCylinderResidual, error) {
	out, e := boundPolynomialSplineSupportResidual(r, l, curveID, origin, radius, &axis)
	if e != nil {
		return PolynomialCylinderResidual{}, e
	}
	return PolynomialCylinderResidual{Spans: out.Spans, MaxAbsoluteResidual: out.MaxAbsoluteResidual, RadialDistanceUpper: out.RadialDistanceUpper}, nil
}
