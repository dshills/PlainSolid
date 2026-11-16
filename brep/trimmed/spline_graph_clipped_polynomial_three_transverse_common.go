package trimmed

import (
	"context"
	"io"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

const nativeClippedPolynomialThreeTransverseCommon = "trimmed-clipped-polynomial-three-transverse-common"

// ClippedPolynomialCommonHistory binds original graph vertices, coedges and
// regions to native topology. Edges and Midpoints follow original branch and
// slab order. The second edge of an unsplit chain is -1. RoofOwners uses the
// original operand order.
type ClippedPolynomialCommonHistory struct {
	Base       int
	Vertices   []int
	Edges      [][2]int
	Midpoints  []int
	RoofFaces  []int
	RoofOwners []int
}

// ClippedPolynomialThreeTransverseCommon retains one embedded intersection
// shell, all three exact crossing records, and the original chart restrictions.
type ClippedPolynomialThreeTransverseCommon clippedPolynomialTransverseCommonProof

type clippedPolynomialTransverseCommonProof struct {
	solid     Solid
	sources   [3]*geom.BSplineSurface
	axis      int
	tolerance float64
	history   ClippedPolynomialCommonHistory
	contacts  []PolynomialMixedEnvelopeContact
	footprint ClippedPolynomialFootprint
}

// Boundary returns the original-source native intersection shell.
func (r *ClippedPolynomialThreeTransverseCommon) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns caller-owned copies of the complete native topology ledger.
func (r *ClippedPolynomialThreeTransverseCommon) History() ClippedPolynomialCommonHistory {
	if r == nil {
		return ClippedPolynomialCommonHistory{}
	}
	return copyClippedPolynomialCommonHistory(r.history)
}

func copyClippedPolynomialCommonHistory(out ClippedPolynomialCommonHistory) ClippedPolynomialCommonHistory {
	out.Vertices = slices.Clone(out.Vertices)
	out.Edges = slices.Clone(out.Edges)
	out.Midpoints = slices.Clone(out.Midpoints)
	out.RoofFaces = slices.Clone(out.RoofFaces)
	out.RoofOwners = slices.Clone(out.RoofOwners)
	return out
}

// Contacts returns all three original transverse crossings in increasing
// physical free-coordinate order. The rational fields are caller-owned.
func (r *ClippedPolynomialThreeTransverseCommon) Contacts() []PolynomialMixedEnvelopeContact {
	if r == nil {
		return nil
	}
	return copyClippedPolynomialCommonContacts(r.contacts)
}

func copyClippedPolynomialCommonContacts(contacts []PolynomialMixedEnvelopeContact) []PolynomialMixedEnvelopeContact {
	out := make([]PolynomialMixedEnvelopeContact, len(contacts))
	for i, c := range contacts {
		out[i] = c.copy()
	}
	return out
}

// ClippedFootprint returns the exact physical footprint and original stock
// domain restrictions.
func (r *ClippedPolynomialThreeTransverseCommon) ClippedFootprint() (ClippedPolynomialFootprint, bool) {
	if r == nil || r.sources[0] == nil {
		return ClippedPolynomialFootprint{}, false
	}
	return r.footprint, true
}

// CommonClippedPolynomialThreeTransverseEnvelope builds the intersection of
// original graph stocks with exactly three transverse crossings in the
// physical overlap. The base and both roof charts remain original.
func CommonClippedPolynomialThreeTransverseEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, tolerance float64, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*ClippedPolynomialThreeTransverseCommon, error) {
	proof, err := buildClippedPolynomialTransverseCommon(ctx, [3]*geom.BSplineSurface{base, a, b}, axis, tolerance, 3, o)
	return (*ClippedPolynomialThreeTransverseCommon)(proof), err
}

func buildClippedPolynomialTransverseCommon(ctx context.Context, sources [3]*geom.BSplineSurface, axis int,
	tolerance float64, requiredCount int, o PolynomialCubicCrossingTangencyEnvelopeOptions) (*clippedPolynomialTransverseCommonProof, error) {
	proof, e := prepareClippedPolynomialCommon(ctx, sources, axis, tolerance, o)
	if e != nil {
		return nil, e
	}
	p := proof.Projection
	n := len(p.Arrangement.ContactOrder)
	if n < 1 || n > maxClippedPolynomialTransverseNodes || requiredCount > 0 && n != requiredCount ||
		!isNativeClippedTransverseArrangement(p.Arrangement, axis, n) {
		return nil, geom.ErrConvergence
	}
	bc, e := polynomialEnvelopeBaseChart(proof.BaseAlignment)
	if e != nil {
		return nil, e
	}
	bound, e := assembleBoundPolynomialEnvelope(ctx, sources[0], [2]*geom.BSplineSurface{sources[1], sources[2]}, parametric.Intersection, tolerance, o,
		p.Arrangement, p.Owners, bc, projectedPolynomialCharts(p), nil)
	if e != nil {
		return nil, e
	}
	binding := bound.binding
	history := ClippedPolynomialCommonHistory{
		Base: bound.base, Vertices: binding.vertices, Edges: binding.chains, RoofFaces: binding.roofFaces, RoofOwners: binding.roofOwners,
	}
	for _, id := range binding.midpoints {
		if id >= 0 {
			history.Midpoints = append(history.Midpoints, id)
		}
	}
	return &clippedPolynomialTransverseCommonProof{
		solid: bound.solid, sources: sources, axis: axis, tolerance: tolerance,
		history: history, contacts: bound.contacts, footprint: clippedCommonFootprint(proof),
	}, nil
}

func (r *ClippedPolynomialThreeTransverseCommon) recipeLedger() polynomialEnvelopeLedger {
	return (*clippedPolynomialTransverseCommonProof)(r).recipeLedger()
}

func (r *clippedPolynomialTransverseCommonProof) recipeLedger() polynomialEnvelopeLedger {
	h := r.history
	return makePolynomialEnvelopeLedger(h.Edges, h.Vertices, h.Midpoints, h.RoofFaces, h.RoofOwners, h.Base, r.contacts)
}

// WriteJSON records the original stocks, exact overlap and native contact ledger.
func (r *ClippedPolynomialThreeTransverseCommon) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialEnvelopeRecipeWithFootprint(ctx, w, nativeClippedPolynomialThreeTransverseCommon,
		r.sources, r.axis, parametric.Intersection, r.tolerance, r.recipeLedger(), &r.footprint)
}

// ReadClippedPolynomialThreeTransverseCommonJSON re-proves the original
// footprint, roof traces, wall ownership and all three crossings.
func ReadClippedPolynomialThreeTransverseCommonJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialCubicCrossingTangencyEnvelopeOptions) (*ClippedPolynomialThreeTransverseCommon, error) {
	var recipe clippedPolynomialCommonRecipe
	if e := readPolynomialGraphRecipe(ctx, r, limits, &recipe); e != nil {
		return nil, e
	}
	if !recipe.validHeader(nativeClippedPolynomialThreeTransverseCommon, 3) || *recipe.Operation != parametric.Intersection || recipe.Footprint == nil {
		return nil, geom.ErrInvalid
	}
	sources, e := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if e != nil {
		return nil, e
	}
	out, e := CommonClippedPolynomialThreeTransverseEnvelope(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if *recipe.Footprint != out.footprint {
		return nil, geom.ErrConvergence
	}
	if e = recipe.verify(out.recipeLedger(), out.solid, limits); e != nil {
		return nil, e
	}
	return out, nil
}
