package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// TiltedEllipticalPrism is a distinct exact rational-frame material domain.
// Its normalized extrusion axis is noncardinal; original analytic model and
// native rational endpoint ancestry are retained without float normalization.
type TiltedEllipticalPrism struct{ PythagoreanEllipticalPrism }

func exactPrismBasis(b mappedRationalBasis) error {
	for _, v := range [4][3]*big.Rat{b.Origin, b.X, b.Y, b.Z} {
		for _, q := range v {
			if q == nil || q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
				return geom.ErrInvalid
			}
		}
	}
	one := big.NewRat(1, 1)
	for _, v := range [3][3]*big.Rat{b.X, b.Y, b.Z} {
		if mappedRationalDot(v, v).Cmp(one) != 0 {
			return geom.ErrInvalid
		}
	}
	if mappedRationalDot(b.X, b.Z).Sign() != 0 {
		return geom.ErrInvalid
	}
	y := mappedRationalCross(b.Z, b.X)
	for i := range y {
		if y[i].Cmp(b.Y[i]) != 0 {
			return geom.ErrInvalid
		}
	}
	return nil
}
func tiltedPrismBasis(b mappedRationalBasis) error {
	if e := exactPrismBasis(b); e != nil {
		return e
	}
	nonzero := 0
	for _, q := range b.Z {
		if q.Sign() != 0 {
			nonzero++
		}
	}
	if nonzero < 2 {
		return geom.ErrInvalid
	}
	return nil
}

// ReadTiltedEllipticalPrism authenticates the complete model-only graph with
// exact rational orthonormal frames and a genuinely tilted extrusion axis.
// Bounded terminating source decimals and lossless binary64 positive dimensions
// are required. No pcurve repair or approximate frame fitting is performed.
func ReadTiltedEllipticalPrism(ctx context.Context, r io.Reader, l Limits, tolerance float64) (TiltedEllipticalPrism, error) {
	m, e := readRationalFrameEllipticalPrism(ctx, r, l, tolerance, true, true)
	if e != nil {
		return TiltedEllipticalPrism{}, e
	}
	return TiltedEllipticalPrism{m}, nil
}

// TiltedEllipticalPrismDefinition uses exact represented direction ratios.
// Their lengths must have rational square roots; supplied decimal approximations
// to a unit vector are not silently substituted with a nearby frame.
type TiltedEllipticalPrismDefinition ExactControlEllipticalPrismDefinition

// WriteTiltedEllipticalPrismSTEP emits the complete analytic model-only material
// and proves it with the tilted reader before any write. Dimensions are local.
func WriteTiltedEllipticalPrismSTEP(ctx context.Context, w io.Writer, d TiltedEllipticalPrismDefinition, tolerance float64) error {
	return writeRationalFrameEllipticalPrismSTEP(ctx, w, ExactControlEllipticalPrismDefinition(d), tolerance, true)
}
