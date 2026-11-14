package step

import (
	"context"
	"io"
	"math/big"
)

// OrthogonalEllipticalAnnularPrism retains concentric ellipse frames differing
// by exactly a positive quarter turn about their common extrusion axis.
type OrthogonalEllipticalAnnularPrism struct{ EllipticalAnnularPrism }
type OrthogonalEllipticalAnnularPrismDefinition EllipticalAnnularPrismDefinition

func orthogonalAnnularBasis(b mappedRationalBasis) mappedRationalBasis {
	out := b
	out.X = b.Y
	for i, q := range b.X {
		out.Y[i] = new(big.Rat).Neg(q)
	}
	return out
}
func orientedAnnularDimensions(a, b, c, d, h float64, nonhomothetic, orthogonal bool) bool {
	if !orthogonal {
		return selectedAnnularDimensions(a, b, c, d, h, nonhomothetic)
	}
	for _, v := range []float64{a, b, c, d, h} {
		if !finiteNumber(v) || v <= 0 {
			return false
		}
	}
	return a > b && c > d && c < b && d < a
}
func fillOrthogonalAnnularPrism(f *File, b mappedRationalBasis, ref Value, a, h float64) error {
	inner := orthogonalAnnularBasis(b)
	z := new(big.Rat)
	ar, hr := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(h)
	for _, v := range []struct {
		id int64
		p  [3]*big.Rat
	}{{59, exactPrismPoint(inner, ar, z, z)}, {61, exactPrismPoint(inner, ar, z, hr)}, {63, exactPrismPoint(inner, ar, z, z)}} {
		en := f.Entities[v.id]
		items := []Value{}
		for _, q := range v.p {
			s, e := mappedTerminatingDecimal(q)
			if e != nil {
				return e
			}
			items = append(items, Value{Kind: Number, Text: s})
		}
		en.Records[0].Args[1] = Value{Kind: Aggregate, Items: items}
		f.Entities[v.id] = en
	}
	for _, id := range []int64{72, 80, 86} {
		en := f.Entities[id]
		en.Records[0].Args[1] = ref
		f.Entities[id] = en
	}
	return nil
}

// ReadOrthogonalEllipticalAnnularPrism proves complete original supports and
// directed source ownership. Strict inner-major<outer-minor containment is exact.
// Aligned or negatively quarter-turned source frames remain unsupported here.
func ReadOrthogonalEllipticalAnnularPrism(ctx context.Context, r io.Reader, l Limits, tol float64) (OrthogonalEllipticalAnnularPrism, error) {
	m, e := readOrientedEllipticalAnnularPrism(ctx, r, l, tol, false, true)
	if e != nil {
		return OrthogonalEllipticalAnnularPrism{}, e
	}
	return OrthogonalEllipticalAnnularPrism{m}, nil
}

// WriteOrthogonalEllipticalAnnularPrismSTEP emits a distinct positive quarter-turn
// inner frame and proves native exact cap/wall joins before any output.
func WriteOrthogonalEllipticalAnnularPrismSTEP(ctx context.Context, w io.Writer, d OrthogonalEllipticalAnnularPrismDefinition, tol float64) error {
	return writeOrientedEllipticalAnnularPrismSTEP(ctx, w, EllipticalAnnularPrismDefinition(d), tol, false, true)
}

// InnerRationalFrame returns caller-owned coefficients for the proved original
// inner ellipse frame, distinct from the promoted outer RationalFrame method.
func (m OrthogonalEllipticalAnnularPrism) InnerRationalFrame() [4][3]*big.Rat {
	out := m.RationalFrame()
	x := out[1]
	out[1] = out[2]
	for i, q := range x {
		if q != nil {
			out[2][i] = new(big.Rat).Neg(q)
		}
	}
	return out
}
