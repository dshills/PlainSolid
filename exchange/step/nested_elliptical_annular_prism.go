package step

import (
	"context"
	"io"
)

func nestedAnnularDimensions(a, b, c, d, h float64) bool {
	for _, v := range []float64{a, b, c, d, h} {
		if !finiteNumber(v) || v <= 0 {
			return false
		}
	}
	return a > b && c > d && c < a && d < b
}
func selectedAnnularDimensions(a, b, c, d, h float64, nonhomothetic bool) bool {
	if !nestedAnnularDimensions(a, b, c, d, h) {
		return false
	}
	return annularDimensions(a, b, c, d, h) != nonhomothetic
}

// NestedEllipticalAnnularPrism is a concentric non-homothetic exact annular
// extrusion. Inner semiaxes are individually smaller; both loops share one
// rational orthonormal frame. Native cap scaling is independent in U and V.
type NestedEllipticalAnnularPrism struct{ EllipticalAnnularPrism }

// ReadNestedEllipticalAnnularPrism proves the complete original model-only
// topology and strictly nested, non-homothetic ellipse supports. The existing
// homothetic reader remains a separate domain. No sampled containment is used.
func ReadNestedEllipticalAnnularPrism(ctx context.Context, r io.Reader, l Limits, tol float64) (NestedEllipticalAnnularPrism, error) {
	m, e := readEllipticalAnnularPrism(ctx, r, l, tol, true)
	if e != nil {
		return NestedEllipticalAnnularPrism{}, e
	}
	return NestedEllipticalAnnularPrism{m}, nil
}

// NestedEllipticalAnnularPrismDefinition has the same exact source fields as the
// homothetic definition, but requires distinct inner/outer semiaxis ratios.
type NestedEllipticalAnnularPrismDefinition EllipticalAnnularPrismDefinition

// WriteNestedEllipticalAnnularPrismSTEP proves exact cap and wall identities and
// re-reads the complete analytic STEP material before any output is written.
func WriteNestedEllipticalAnnularPrismSTEP(ctx context.Context, w io.Writer, d NestedEllipticalAnnularPrismDefinition, tol float64) error {
	return writeEllipticalAnnularPrismSTEP(ctx, w, EllipticalAnnularPrismDefinition(d), tol, true)
}
