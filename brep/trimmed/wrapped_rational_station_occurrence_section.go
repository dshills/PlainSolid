//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRationalStationOccurrenceSection is the exact world curve and plane
// frame of an original horizontal section. Original retains native cap/crease
// owners and the original curve parameterization. Origin is the mapped profile
// center, U/V are mapped original X/Y axes, and Normal=U×V points toward
// increasing original height. At the lower cap it is opposite cap outwardness.
// This is a weighted noncircular profile, not a replacement circle or polygon.
type WrappedRationalStationOccurrenceSection struct {
	Original             WrappedStationSection
	Curve                *geom.ExactSplineCurve
	Origin, U, V, Normal [3]*big.Rat
	TransformOperations  int
}

// SectionRational accepts a height in the original source frame, hence a
// possibly oblique world plane normal to the occurrence's original loft axis.
// It does not claim arbitrary oblique cuts relative to the original loft.
// Source extraction and curve transformation have separate options-bounded
// work stages; pose/frame arithmetic uses the occurrence's bit limit, which
// additionally caps transformed curve coefficients. No rounded curve, frame
// or support is used as proof. All failures are atomic.
func (q WrappedRationalStationOccurrence) SectionRational(ctx context.Context, height *big.Rat, o geom.SurfacePatchOptions) (WrappedRationalStationOccurrenceSection, error) {
	empty := WrappedRationalStationOccurrenceSection{}
	if q.bits == 0 {
		return empty, geom.ErrInvalid
	}
	original, e := q.source.SectionWrappedRationalStationLoftRational(ctx, height, o)
	if e != nil {
		return empty, e
	}
	if original.Inner != nil || original.Outer.Section.Curve == nil {
		return empty, geom.ErrConvergence
	}
	matrix, e := q.pose.Matrix()
	if e != nil {
		return empty, e
	}
	limits := o
	if limits.MaxCoefficientBits == 0 || limits.MaxCoefficientBits > q.bits {
		limits.MaxCoefficientBits = q.bits
	}
	curve, e := original.Outer.Section.Curve.TransformRational(ctx, matrix, limits)
	if e != nil {
		return empty, e
	}
	center := [3]*big.Rat{original.Outer.Section.CenterX, original.Outer.Section.CenterY, original.Outer.Section.Height}
	origin, e := q.pose.TransformPoint(ctx, center, false, q.bits)
	if e != nil {
		return empty, e
	}
	result := WrappedRationalStationOccurrenceSection{Original: original, Curve: curve, Origin: origin, TransformOperations: curve.ExactOperations()}
	for i := 0; i < 3; i++ {
		result.U[i] = new(big.Rat).Set(matrix[i][0])
		result.V[i] = new(big.Rat).Set(matrix[i][1])
		result.Normal[i] = new(big.Rat).Set(matrix[i][2])
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
