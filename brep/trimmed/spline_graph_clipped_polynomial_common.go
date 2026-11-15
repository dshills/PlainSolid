package trimmed

import (
	"context"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeClippedPolynomialCommon = "trimmed-clipped-polynomial-common"

// ClippedPolynomialFootprint records the physical Common domain and its
// original chart restrictions. Original and Restricted index base, A, B.
// BoundaryRoofs goes counterclockwise from YMin, with bits 0 and 1 denoting
// that the side belongs to the original footprint of A or B.
type ClippedPolynomialFootprint struct {
	Physical      geom.ParameterRectangle    `json:"physical"`
	Original      [3]geom.ParameterRectangle `json:"originalDomains"`
	Restricted    [3]geom.ParameterRectangle `json:"restrictedDomains"`
	BoundaryRoofs [4]uint8                   `json:"boundaryRoofs"`
}

type clippedPolynomialCommonRecipe struct {
	polynomialEnvelopeRecipe
	Footprint *ClippedPolynomialFootprint `json:"footprint"`
}

// ClippedFootprint is the original domain ledger of a clipped Common result.
func (r *AlignedPolynomialEnvelope) ClippedFootprint() (ClippedPolynomialFootprint, bool) {
	if r == nil || r.clippedFootprint == nil {
		return ClippedPolynomialFootprint{}, false
	}
	return *r.clippedFootprint, true
}

// CommonClippedPolynomialEnvelope builds the material intersection over the
// exact shared portion of two axis-aligned physical stock footprints. Each
// original roof must be strictly above the common base over its full stock.
// Two transverse crossings, or a cubic crossing and a quadratic tangency,
// must remain in the overlap.
// The shell retains unmodified original cap sources and owner wall trims.
func CommonClippedPolynomialEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	var patches [3]*geom.SurfacePatch
	for i, source := range sources {
		p, e := geom.NewSurfacePatch(source, o.Section.Patch)
		if e != nil {
			return nil, e
		}
		patches[i] = p
	}
	proof, e := geom.DiscoverClippedSplineGraphPolynomialArrangement(ctx, patches[0], patches[1], patches[2], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	bc, e := polynomialEnvelopeBaseChart(proof.BaseAlignment)
	if e != nil {
		return nil, e
	}
	p := proof.Projection
	out, e := buildNativePolynomialEnvelope(ctx, sources, 0, axis, parametric.Intersection, tolerance, o,
		nativeClippedPolynomialCommon, p.Arrangement, p.Owners, bc, projectedPolynomialCharts(p))
	if e != nil {
		return nil, e
	}
	out.clippedFootprint = &ClippedPolynomialFootprint{Physical: proof.Reference.Domain(), Original: proof.OriginalDomains,
		Restricted: proof.RestrictedDomains, BoundaryRoofs: proof.BoundaryRoofs}
	return out, nil
}

// ReadClippedPolynomialCommonJSON re-proves the original stock clearances and
// exact common footprint before binding the native trim and contact ledger.
func ReadClippedPolynomialCommonJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*AlignedPolynomialEnvelope, error) {
	var recipe clippedPolynomialCommonRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if !recipe.validHeader(nativeClippedPolynomialCommon, 2) || *recipe.Operation != parametric.Intersection || recipe.Footprint == nil {
		return nil, geom.ErrInvalid
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if e != nil {
		return nil, e
	}
	out, e := CommonClippedPolynomialEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if *recipe.Footprint != *out.clippedFootprint {
		return nil, geom.ErrConvergence
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
