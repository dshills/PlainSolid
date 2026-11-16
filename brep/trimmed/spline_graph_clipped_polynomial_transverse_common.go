package trimmed

import (
	"context"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeClippedPolynomialTransverseCommon = "trimmed-clipped-polynomial-transverse-common"

// ClippedPolynomialTransverseCommon represents the closed intersection of
// two original stocks and their discovered transverse contact arrangement.
type ClippedPolynomialTransverseCommon struct {
	proof *clippedPolynomialTransverseCommonProof
}

// CommonClippedPolynomialTransverseEnvelope builds a source-proved
// intersection over unequal exactly representable axis-aligned footprints.
// It discovers one to five transverse crossings, preserving both original
// roof charts, the original base, and the exact clipped wall clocks.
func CommonClippedPolynomialTransverseEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*ClippedPolynomialTransverseCommon, error) {
	proof, err := buildClippedPolynomialTransverseCommon(ctx, [3]*geom.BSplineSurface{base, a, b}, axis, tolerance, 0, o)
	if err != nil {
		return nil, err
	}
	return &ClippedPolynomialTransverseCommon{proof: proof}, nil
}

// Boundary returns the closed native Common shell.
func (r *ClippedPolynomialTransverseCommon) Boundary() Solid {
	if r == nil || r.proof == nil {
		return Solid{}
	}
	return r.proof.solid
}

// Contacts returns detached exact contact records in physical free-axis order.
func (r *ClippedPolynomialTransverseCommon) Contacts() []PolynomialMixedEnvelopeContact {
	if r == nil || r.proof == nil {
		return nil
	}
	return copyClippedPolynomialCommonContacts(r.proof.contacts)
}

// History returns a detached copy of the original branch, sector and base
// bindings to the native shell.
func (r *ClippedPolynomialTransverseCommon) History() ClippedPolynomialCommonHistory {
	if r == nil || r.proof == nil {
		return ClippedPolynomialCommonHistory{}
	}
	return copyClippedPolynomialCommonHistory(r.proof.history)
}

// ClippedFootprint returns the physical overlap and the original owner domains.
func (r *ClippedPolynomialTransverseCommon) ClippedFootprint() (ClippedPolynomialFootprint, bool) {
	if r == nil || r.proof == nil {
		return ClippedPolynomialFootprint{}, false
	}
	return r.proof.footprint, true
}

// WriteJSON records both original stocks, the exact clipping domains and
// the complete native contact and cell ledger.
func (r *ClippedPolynomialTransverseCommon) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.proof == nil {
		return geom.ErrInvalid
	}
	d := r.proof
	return writePolynomialEnvelopeRecipeWithFootprint(ctx, w, nativeClippedPolynomialTransverseCommon,
		d.sources, d.axis, parametric.Intersection, d.tolerance, d.recipeLedger(), &d.footprint)
}

// ReadClippedPolynomialTransverseCommonJSON rediscovers the complete
// intersection of the original supports and validates every recorded entity.
// The recorded contact count is checked against discovery.
func ReadClippedPolynomialTransverseCommonJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*ClippedPolynomialTransverseCommon, error) {
	var recipe clippedPolynomialCommonRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	n := len(recipe.Ledger.Contacts)
	if n < 1 || n > maxClippedPolynomialTransverseNodes || !recipe.validHeader(nativeClippedPolynomialTransverseCommon, n) ||
		*recipe.Operation != parametric.Intersection || recipe.Footprint == nil {
		return nil, geom.ErrInvalid
	}
	sources, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CommonClippedPolynomialTransverseEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if *recipe.Footprint != out.proof.footprint {
		return nil, geom.ErrConvergence
	}
	if err := recipe.verify(out.proof.recipeLedger(), out.proof.solid, limits); err != nil {
		return nil, err
	}
	return out, nil
}
