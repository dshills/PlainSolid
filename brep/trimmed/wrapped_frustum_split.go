//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SplitWrappedFrustumAtZ partitions a canonical drafted wrapped solid at
// an exactly represented height. The shared cap is constructed with the
// exact rational section radius and original quadrant basis. If its
// vertices are not exactly representable, no partial output is returned.
func SplitWrappedFrustumAtZ(ctx context.Context, w *geom.WrappedSurfacePatch, z, tolerance float64, o geom.SurfacePatchOptions) (Solid, Solid, WrappedCylinderSplitHistory, error) {
	var h WrappedCylinderSplitHistory
	a, b, e := geom.SplitWrappedFrustumAtZ(ctx, w, z, o)
	if e != nil {
		return Solid{}, Solid{}, h, e
	}
	lower, _, e := NewWrappedFrustum(ctx, a, tolerance, o)
	if e != nil {
		return Solid{}, Solid{}, h, e
	}
	upper, _, e := NewWrappedFrustum(ctx, b, tolerance, o)
	if e != nil {
		return Solid{}, Solid{}, h, e
	}
	return lower, upper, wrappedCylinderCutHistory(), nil
}

// WrappedFrustumDraftAtNeutralZ builds a drafted support from an original
// canonical cylinder or frustum. The given signed radius change per model
// height leaves the neutral section fixed. Endpoint changes are exact
// rationals; only the resulting trimmed topology requires represented
// model vertices.
func WrappedFrustumDraftAtNeutralZ(ctx context.Context, w *geom.WrappedSurfacePatch, neutralZ, slope, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, geom.WrappedFrustumEmbedding, error) {
	var zero geom.WrappedFrustumEmbedding
	if !finite(neutralZ) || !finite(slope) {
		return Solid{}, zero, geom.ErrInvalid
	}
	p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return Solid{}, p, e
	}
	offset := func(z *big.Rat) *big.Rat {
		d := new(big.Rat).Sub(z, ratInset(neutralZ))
		return d.Mul(d, ratInset(slope))
	}
	out, p, e := geom.DraftWrappedFrustumExact(ctx, w, offset(p.LowerZ), offset(p.UpperZ), o)
	if e != nil {
		return Solid{}, p, e
	}
	return NewWrappedFrustum(ctx, out, tolerance, o)
}
