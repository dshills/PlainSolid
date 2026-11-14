package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// compareNativeSourceQuadraticUse independently compares the retained model
// descriptor on its actual range to the complete composed source p-curve.
func compareNativeSourceQuadraticUse(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	q, ok := use.Curve.(ExactQuadraticUV)
	if !ok {
		return false, nil
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	source, reverse, e := exactEdgeSplineWithOptions(model, limits)
	if e != nil || source == nil {
		return false, e
	}
	patch, e := exactFaceSupportPatch(Face{Surface: surface}, limits)
	if e != nil {
		return false, e
	}
	target, e := composeSplineTrimControls(ctx, patch, q.HomogeneousControls(), limits)
	if e != nil {
		return false, e
	}
	return compareSplineDescriptors(ctx, source, target, reverse != use.Reversed, o)
}

// exactAffineNativeSourceUse proves identical normalized parameterization,
// not merely collinearity or equal endpoints. Bounded quadratic controls only.
func exactAffineNativeSourceUse(u Use) bool {
	if u.Curve == nil {
		return true
	}
	q, ok := u.Curve.(ExactQuadraticUV)
	if !ok {
		return false
	}
	h := q.HomogeneousControls()
	if h[0][2] == nil || h[0][2].Sign() <= 0 {
		return false
	}
	for i, p := range h {
		if p[2].Cmp(h[0][2]) != 0 {
			return false
		}
		t := big.NewRat(int64(i), 2)
		for j, ends := range [2][2]float64{{u.UVStart.X, u.UVEnd.X}, {u.UVStart.Y, u.UVEnd.Y}} {
			want := new(big.Rat).Mul(p[2], nativeAlgebraicLerp(ends[0], ends[1], t))
			if p[j].Cmp(want) != 0 {
				return false
			}
		}
	}
	return true
}
