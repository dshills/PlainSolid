//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumWorldConeWitness denotes
// Base+(RadialVector+Axis*AxialCoefficient)*sqrt(Radicand).
// Axis is the exact transformed source unit axis, not a material normal.
// RadialVector is orthogonal to Axis. Root encloses the positive radical;
// the original retained conical face is identified by Face.
type FilletedFrustumWorldConeWitness struct {
	Base, RadialVector, Axis   [3]*big.Rat
	AxialCoefficient, Radicand *big.Rat
	Root                       [2]*big.Rat
	Face                       int
}

// FilletedFrustumOccurrenceConeDistance preserves original source coordinates
// separately. WorldCone is the selected retained candidate, otherwise Fallback
// holds the already-computed nested/torus/rim/rational result in world space.
type FilletedFrustumOccurrenceConeDistance struct {
	Original                 FilletedFrustumConeDistance
	Lower, Upper             *big.Rat
	WorldCone                *FilletedFrustumWorldConeWitness
	Fallback                 FilletedFrustumOccurrenceNestedDistance
	NearestProven, Converged bool
}

// BoundBoundaryConeDistanceRational performs exactly one source query after
// exact inverse placement. Proper rational orthonormality preserves the full
// nearest proof, trim ownership and squared bounds. Pose intermediates are
// limited to 4096 bits and any error returns an empty result.
func (q FilletedFrustumOccurrence) BoundBoundaryConeDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (out FilletedFrustumOccurrenceConeDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceConeDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundBoundaryConeDistanceRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	return q.transformFilletedConeDistance(ctx, original)
}

func (q FilletedFrustumOccurrence) transformFilletedConeDistance(ctx context.Context, original FilletedFrustumConeDistance) (out FilletedFrustumOccurrenceConeDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceConeDistance{}
		}
	}()
	var e error
	out.Original = original
	out.Lower = new(big.Rat).Set(original.Lower)
	out.Upper = new(big.Rat).Set(original.Upper)
	out.NearestProven = original.NearestProven
	out.Converged = original.Converged
	if original.Cone == nil {
		out.Fallback, e = q.transformFilletedNestedDistance(ctx, original.Baseline)
		if e != nil {
			return out, e
		}
		return out, nil
	}
	w := original.Cone
	base, e := q.pose.TransformPoint(ctx, w.Base, false, 4096)
	if e != nil {
		return out, e
	}
	radial, e := q.pose.TransformVector(ctx, [3]*big.Rat{w.RadicalVector[0], w.RadicalVector[1], new(big.Rat)}, false, 4096)
	if e != nil {
		return out, e
	}
	axis, e := q.pose.TransformVector(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldCone = &FilletedFrustumWorldConeWitness{Base: base, RadialVector: radial, Axis: axis, AxialCoefficient: new(big.Rat).Set(w.RadicalVector[2]), Radicand: new(big.Rat).Set(w.Radicand), Root: [2]*big.Rat{new(big.Rat).Set(w.Root[0]), new(big.Rat).Set(w.Root[1])}, Face: w.Face}
	return out, nil
}
