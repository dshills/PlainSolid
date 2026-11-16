package trimmed

import (
	"context"
	"io"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

const (
	nativeClippedPolynomialTransverseCut = "trimmed-clipped-polynomial-transverse-cut-components"
	maxClippedPolynomialTransverseNodes  = 5
)

// ClippedPolynomialTransverseCut represents separate positive components
// of a source-backed Cut and their original transverse contact pairs.
type ClippedPolynomialTransverseCut struct {
	stocks    [3]*geom.BSplineSurface
	proof     *clippedPolynomialCutData
	axis      int
	tolerance float64
}

// CutClippedPolynomialTransverseComponents subtracts B from A on unequal
// exactly representable axis-aligned footprints. It discovers one to five
// transverse crossings in the complete overlap and preserves original
// charts, coedges and material components. A crossing within one connected
// component returns ErrClippedPolynomialCutSelfContact and no geometry.
func CutClippedPolynomialTransverseComponents(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, tolerance float64, o PolynomialNodeCutOptions) (*ClippedPolynomialTransverseCut, error) {
	stocks := [3]*geom.BSplineSurface{base, a, b}
	proof, err := buildClippedPolynomialCut(ctx, stocks, axis, tolerance, o, func(r geom.SplineGraphPolynomialArrangement, axis int) bool {
		n := len(r.ContactOrder)
		return n > 0 && n <= maxClippedPolynomialTransverseNodes && isNativeClippedTransverseArrangement(r, axis, n)
	})
	if err != nil {
		return nil, err
	}
	return &ClippedPolynomialTransverseCut{stocks: stocks, proof: proof, axis: axis, tolerance: tolerance}, nil
}

// Components returns the embedded positive shells in original chamber order.
func (r *ClippedPolynomialTransverseCut) Components() []Solid {
	if r == nil || r.proof == nil {
		return nil
	}
	return slices.Clone(r.proof.material.components)
}

// Contacts returns all original transverse crossings in physical free-axis
// order. Every entry links two distinct component-local vertex occurrences.
func (r *ClippedPolynomialTransverseCut) Contacts() []PolynomialMixedCutContact {
	if r == nil || r.proof == nil {
		return nil
	}
	return r.proof.typedContacts()
}

// ExactComponentVolumes returns caller-owned source integrals and their sum.
func (r *ClippedPolynomialTransverseCut) ExactComponentVolumes() ([]*big.Rat, *big.Rat) {
	if r == nil || r.proof == nil {
		return nil, nil
	}
	return r.proof.exactVolumes()
}

// Chambers lists the original overlap regions and positive exclusive stock
// tiles retained in a component. The returned slice is caller-owned.
func (r *ClippedPolynomialTransverseCut) Chambers(component int) []ClippedPolynomialCutRegion {
	if r == nil || r.proof == nil || component < 0 || component >= len(r.proof.shells) {
		return nil
	}
	return slices.Clone(r.proof.shells[component].Chambers)
}

// Walls describes original exposed walls and both source coedge clocks.
func (r *ClippedPolynomialTransverseCut) Walls(component int) []ClippedPolynomialWallBinding {
	if r == nil || r.proof == nil || component < 0 || component >= len(r.proof.shells) {
		return nil
	}
	return cloneClippedPolynomialWallBindings(r.proof.shells[component].Walls)
}

// Tessellate returns separate closed previews and proves that the only
// contacts between them are the distinct source-declared vertex pairs.
func (r *ClippedPolynomialTransverseCut) Tessellate(ctx context.Context, o parametric.TessellationOptions) ([]mesh.Mesh, error) {
	if r == nil || r.proof == nil {
		return nil, geom.ErrInvalid
	}
	d := r.proof
	return tessellateClippedCut(ctx, r.axis, d.material, d.shells, d.volumes, true, nil, o)
}

// TessellateWithVolumeTolerance also bounds each component's relative
// preview volume error against its exact source integral. Budget
// exhaustion returns no meshes.
func (r *ClippedPolynomialTransverseCut) TessellateWithVolumeTolerance(ctx context.Context, o parametric.TessellationOptions,
	relativeVolumeTolerance float64) ([]mesh.Mesh, error) {
	if r == nil || r.proof == nil || !finite(relativeVolumeTolerance) || relativeVolumeTolerance <= 0 {
		return nil, geom.ErrInvalid
	}
	d := r.proof
	return tessellateClippedCut(ctx, r.axis, d.material, d.shells, d.volumes, false, ratInset(relativeVolumeTolerance), o)
}

// WriteJSON records the original stocks and the complete independently
// checked component, contact, wall and volume ledger.
func (r *ClippedPolynomialTransverseCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || r == nil || r.proof == nil || w == nil {
		return geom.ErrInvalid
	}
	return writeClippedTransverseCutRecipe(ctx, w, nativeClippedPolynomialTransverseCut, r.stocks, r.axis, r.tolerance, r.proof)
}

// ReadClippedPolynomialTransverseCutJSON discovers the whole original
// overlap again. The number of recorded crossings is a checked assertion,
// never an input to constrain branch discovery.
func ReadClippedPolynomialTransverseCutJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialNodeCutOptions) (*ClippedPolynomialTransverseCut, error) {
	var recipe clippedPolynomialCutRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	n := len(recipe.Orders)
	if n < 1 || n > maxClippedPolynomialTransverseNodes || len(recipe.DerivativeGaps) != n || len(recipe.Shells) < 2 || recipe.Total == "" ||
		!recipe.Footprints[0].Valid() || !recipe.Footprints[1].Valid() {
		return nil, geom.ErrInvalid
	}
	stocks, err := recipe.decodeSupports(ctx, limits, nativeClippedPolynomialTransverseCut, n)
	if err != nil {
		return nil, err
	}
	out, err := CutClippedPolynomialTransverseComponents(ctx, stocks[0], stocks[1], stocks[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if err := recipe.verifyClippedTransverseCut(out.proof, limits); err != nil {
		return nil, err
	}
	return out, nil
}
