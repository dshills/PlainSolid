package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumWorldNestedToroidWitness retains two exact positive radicals:
// rho=sqrt(AzimuthSquared), d=sqrt((rho-MajorRadius)^2+HeightOffset^2).
// With R=MajorRadius+MinorRadius*(rho-MajorRadius)/d the exact point is
// Base+RadialVector*(R/rho)+Axis*(MinorRadius*HeightOffset/d).
// Base, RadialVector and unit Axis are exact rational world data. Axis is the
// transformed torus axis, not an outward material normal. Face identifies the
// unchanged original native face. Root intervals enclose, never replace, rho/d.
type FilletedFrustumWorldNestedToroidWitness struct {
	Base, RadialVector, Axis                               [3]*big.Rat
	MajorRadius, MinorRadius, HeightOffset, AzimuthSquared *big.Rat
	AzimuthRoot, MeridianRoot                              [2]*big.Rat
	Face                                                   int
}

// FilletedFrustumOccurrenceNestedDistance selects WorldNested when the source
// nested projection wins. Otherwise Fallback contains the already-computed
// world torus/rim/rational result. Squared bounds and nearest status are exact
// rigid invariants; Original coordinates explicitly remain in the source frame.
type FilletedFrustumOccurrenceNestedDistance struct {
	Original                 FilletedFrustumNestedDistance
	Lower, Upper             *big.Rat
	WorldNested              *FilletedFrustumWorldNestedToroidWitness
	Fallback                 FilletedFrustumOccurrenceToroidDistance
	NearestProven, Converged bool
}

// BoundBoundaryNestedDistanceRational performs one original-frame query and
// symbolically places its chosen witness. Exact proper orthonormality preserves
// both nested radical meaning, source ownership and Euclidean squared distance.
// Pose arithmetic keeps 4096-bit intermediate limits; failures are atomic.
func (q FilletedFrustumOccurrence) BoundBoundaryNestedDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (out FilletedFrustumOccurrenceNestedDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceNestedDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundBoundaryNestedDistanceRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	return q.transformFilletedNestedDistance(ctx, original)
}

// transformFilletedNestedDistance reuses one completed source proof.
func (q FilletedFrustumOccurrence) transformFilletedNestedDistance(ctx context.Context, original FilletedFrustumNestedDistance) (out FilletedFrustumOccurrenceNestedDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceNestedDistance{}
		}
	}()
	var e error
	out.Original = original
	out.Lower = new(big.Rat).Set(original.Lower)
	out.Upper = new(big.Rat).Set(original.Upper)
	out.NearestProven = original.NearestProven
	out.Converged = original.Converged
	if original.Nested == nil {
		out.Fallback, e = q.transformFilletedToroidDistance(ctx, original.Baseline)
		if e != nil {
			return out, e
		}
		return out, nil
	}
	w := original.Nested
	base, e := q.pose.TransformPoint(ctx, w.Center, false, 4096)
	if e != nil {
		return out, e
	}
	radial, e := q.pose.TransformVector(ctx, [3]*big.Rat{w.Azimuth[0], w.Azimuth[1], new(big.Rat)}, false, 4096)
	if e != nil {
		return out, e
	}
	axis, e := q.pose.TransformVector(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldNested = &FilletedFrustumWorldNestedToroidWitness{Base: base, RadialVector: radial, Axis: axis, MajorRadius: new(big.Rat).Set(w.MajorRadius), MinorRadius: new(big.Rat).Set(w.MinorRadius), HeightOffset: new(big.Rat).Set(w.HeightOffset), AzimuthSquared: new(big.Rat).Set(w.AzimuthSquared), AzimuthRoot: [2]*big.Rat{new(big.Rat).Set(w.AzimuthRoot[0]), new(big.Rat).Set(w.AzimuthRoot[1])}, MeridianRoot: [2]*big.Rat{new(big.Rat).Set(w.MeridianRoot[0]), new(big.Rat).Set(w.MeridianRoot[1])}, Face: w.Face}
	return out, nil
}
