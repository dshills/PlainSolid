package trimmed

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"slices"

	"plainsolid/geom"
)

const nativeAlignedPolynomialCut = "trimmed-aligned-polynomial-cut-components"

// AlignedPolynomialCut contains separately embedded positive shells for an
// order-three crossing and an order-two tangency in independent roof charts.
// Contacts record shared model points with a separate vertex ID per shell.
type AlignedPolynomialCut struct {
	sources     [3]*geom.BSplineSurface
	material    polynomialNodeCutMaterial
	orders      [2]int
	derivatives [2]*big.Rat
	volumes     []*big.Rat
	total       *big.Rat
	axis        int
	tolerance   float64
	kind        string
}

// Components returns shells in the retained arrangement's region order.
func (r *AlignedPolynomialCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.material.components)
}

// Regions returns the retained region indices in the physical base chart.
func (r *AlignedPolynomialCut) Regions() []int {
	if r == nil {
		return nil
	}
	return slices.Clone(r.material.regions)
}

// Contacts returns the exact junctions in increasing physical free order.
func (r *AlignedPolynomialCut) Contacts() [2]PolynomialMixedCutContact {
	var out [2]PolynomialMixedCutContact
	if r == nil {
		return out
	}
	for i, c := range r.material.contacts {
		out[i] = PolynomialMixedCutContact{
			TwoPolynomialNodeComponentContact: c.copy(),
			Order:                             r.orders[i], DerivativeGap: new(big.Rat).Set(r.derivatives[i]),
		}
	}
	return out
}

// ExactComponentVolumes returns caller-owned copies of the proven physical
// volumes in component order and their total. The integrals are recomputed
// from the original roofs during construction and native reload.
func (r *AlignedPolynomialCut) ExactComponentVolumes() (volumes []*big.Rat, total *big.Rat) {
	if r == nil || r.total == nil {
		return nil, nil
	}
	for _, v := range r.volumes {
		volumes = append(volumes, new(big.Rat).Set(v))
	}
	return volumes, new(big.Rat).Set(r.total)
}

// CutAlignedPolynomialComponents subtracts B from A. It requires the bounded
// aligned envelope family with a constant physical quadratic height scale.
// Each selected positive sector becomes a component on both original roofs.
// Closures meet only at the exactly indexed junctions.
func CutAlignedPolynomialComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o PolynomialNodeCutOptions) (*AlignedPolynomialCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	stock, e := prepareAlignedPolynomialStock(ctx, base, a, b, PolynomialCubicCrossingTangencyEnvelopeOptions(o))
	if e != nil {
		return nil, e
	}
	measure, e := geom.IntegrateAlignedSplineGraphPolynomialVolumes(ctx, stock.patches[0], stock.patches[1], axis, stock.first == 1, o.Proof)
	if e != nil {
		return nil, e
	}
	aligned, proof := measure.Alignment, measure.Alignment.Arrangement
	owners, charts := aligned.Owners, alignedPolynomialCharts(aligned)
	if measure.Reverse {
		proof.Regions = slices.Clone(proof.Regions)
		for i := range proof.Regions {
			proof.Regions[i].HeightSide = -proof.Regions[i].HeightSide
		}
		owners[0], owners[1] = owners[1], owners[0]
		charts[0], charts[1] = charts[1], charts[0]
	}
	return buildNativePolynomialCut(ctx, [3]*geom.BSplineSurface{base, a, b}, axis, tolerance, o,
		nativeAlignedPolynomialCut, proof, owners, charts, polynomialCutVolumes{measure.Regions, measure.Volumes, measure.Total})
}

type polynomialCutVolumes struct {
	regions []int
	volumes []*big.Rat
	total   *big.Rat
}

func buildNativePolynomialCut(ctx context.Context, sources [3]*geom.BSplineSurface, axis int, tolerance float64, o PolynomialNodeCutOptions,
	kind string, proof geom.SplineGraphPolynomialArrangement, owners [2]geom.SplineGraphPolynomialOwnerLedger,
	charts [2]graphEnvelopeChart, measure polynomialCutVolumes) (*AlignedPolynomialCut, error) {
	if !isNativeAlignedPolynomialArrangement(proof, axis) {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := proof.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	pieces, e := polynomialNodePieces(proof.Edges, uv)
	if e != nil {
		return nil, e
	}
	loops, e := polynomialNodeRegionLoops(proof.Regions, uv, pieces)
	if e != nil {
		return nil, e
	}
	mapped, e := polynomialOwnerPieces(proof, owners)
	if e != nil {
		return nil, e
	}
	// Source ledger keys are physical directions. Each value is the
	// exact restriction independently constructed in the owner's chart.
	expect := [2]map[[2]geom.Vec2]openSectionPiece{}
	for layer := range expect {
		expect[layer] = make(map[[2]geom.Vec2]openSectionPiece)
		for i, chain := range pieces {
			for j, p := range chain {
				owner := mapped[layer][i][j]
				if _, duplicate := expect[layer][[2]geom.Vec2{p.a, p.b}]; duplicate {
					return nil, geom.ErrConvergence
				}
				expect[layer][[2]geom.Vec2{p.a, p.b}] = owner
				expect[layer][[2]geom.Vec2{p.b, p.a}] = openSectionPiece{a: owner.b, b: owner.a, c: reversePCurve(owner.c), side: owner.side}
			}
		}
	}
	roofs := [2]*geom.BSplineSurface{sources[1], sources[2]}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	assemble := func(loop []openSectionPiece) (Solid, OpenSectionHistory, error) {
		return assembleOpenSplineGraphPairOnCharts(ctx, roofs[0], roofs[1], 0, true, tolerance, loop, charts, evaluate)
	}
	bind := func(part Solid, h OpenSectionHistory, loop []openSectionPiece) error {
		return bindAlignedPolynomialCutLayer(ctx, part, h, roofs, charts, loop, expect, o.Proof)
	}
	material, e := buildPolynomialNodeCutComponents(ctx, o.Proof, proof.Regions, proof.Nodes, proof.Vertices, xyz, loops, assemble, bind)
	if e != nil {
		return nil, e
	}
	if !slices.Equal(material.regions, measure.regions) || len(material.components) != len(measure.volumes) {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return &AlignedPolynomialCut{
		sources: sources, material: material, orders: [2]int(proof.ContactOrder),
		derivatives: [2]*big.Rat(proof.DerivativeGap), volumes: measure.volumes, total: measure.total, axis: axis, tolerance: tolerance, kind: kind,
	}, nil
}

func bindAlignedPolynomialCutLayer(ctx context.Context, part Solid, h OpenSectionHistory, roofs [2]*geom.BSplineSurface, charts [2]graphEnvelopeChart,
	loop []openSectionPiece, expected [2]map[[2]geom.Vec2]openSectionPiece, proof geom.GraphPointContactOptions) error {
	type wallOwner struct {
		roof int
		trim openSectionPiece
	}
	walls := make(map[int]wallOwner, 2*len(h.WallFaces))
	orientation := [2]bool{charts[0].reversed, !charts[1].reversed}
	for i, index := range [2]int{h.SplineFace, h.PlaneFace} {
		face := part.faces[index]
		if len(face.Outer) != len(loop) {
			return geom.ErrConvergence
		}
		own := make([]openSectionPiece, len(loop))
		for j, p := range loop {
			if p.side < 0 {
				q, ok := expected[i][[2]geom.Vec2{p.a, p.b}]
				if !ok {
					return geom.ErrConvergence
				}
				own[j] = q
			} else {
				q, e := charts[i].piece(p)
				if e != nil {
					return e
				}
				own[j] = q
			}
		}
		contour := 0
		for j, p := range own {
			k := j
			if charts[i].reversed {
				k = len(loop) - 1 - j
			}
			use := face.Outer[k]
			if use.Reversed != charts[i].reversed {
				return geom.ErrConvergence
			}
			if p.side < 0 {
				if contour >= len(h.ContourEdges) || use.Edge != h.ContourEdges[contour] {
					return geom.ErrConvergence
				}
				contour++
				// The upper source owns the shared model curve.
				if i == 1 {
					continue
				}
			} else {
				if _, duplicate := walls[use.Edge]; duplicate {
					return geom.ErrConvergence
				}
				walls[use.Edge] = wallOwner{i, p}
			}
			model, ok := part.edges[use.Edge].Curve.(surfaceTrimCurve)
			if !ok || model.surface != roofs[i] || model.a != p.a || model.b != p.b || !matchesTangencyTrim(model.curve, p.c) {
				return geom.ErrConvergence
			}
		}
		if contour == 0 || contour != len(h.ContourEdges) {
			return geom.ErrConvergence
		}
		if charts[i].reversed {
			slices.Reverse(own)
			for j := range own {
				p := &own[j]
				p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
			}
		}
		if e := bindPolynomialTangencyFace(face, roofs[i], orientation[i], own); e != nil {
			return fmt.Errorf("trimmed: aligned Cut cap %d: %w", i, e)
		}
	}

	for _, index := range h.WallFaces {
		var seen [2]bool
		for _, use := range part.faces[index].Outer {
			owner, ok := walls[use.Edge]
			if !ok {
				continue // upright
			}
			i := owner.roof
			trim, ok := graphWallSourceTrim(use.Curve, i == 0, charts[i].transfer != nil)
			if !ok || seen[i] || use.Reversed != (i == 0) || trim.surface != roofs[i] || trim.a != owner.trim.a || trim.b != owner.trim.b {
				return geom.ErrConvergence
			}
			seen[i] = true
			delete(walls, use.Edge)
		}
		if !seen[0] || !seen[1] {
			return geom.ErrConvergence
		}
	}
	if len(walls) != 0 {
		return geom.ErrConvergence
	}
	return validatePolynomialCutBoundaryOnCharts(ctx, part, h, orientation, proof)
}

func graphWallSourceTrim(curve PCurve, reverse, aligned bool) (wallTrim, bool) {
	if reverse {
		c, ok := curve.(reverseTrim)
		if !ok {
			return wallTrim{}, false
		}
		curve = c.differentiableTrim
	}
	if aligned {
		c, ok := curve.(affineGraphWallTrim)
		return c.wallTrim, ok
	}
	c, ok := curve.(wallTrim)
	return c, ok
}

// WriteJSON records the unmodified input supports and the indexed component
// contacts. Loading re-proves both native owner boundaries and exact volumes.
func (r *AlignedPolynomialCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writePolynomialMixedCutRecipe(ctx, w, r.kind, r.sources, r.axis, r.tolerance, r.material, r.orders[:], r.derivatives[:])
}

// ReadAlignedPolynomialCutJSON reconstructs the original-source components
// and verifies the contact and region ledger before publishing them.
func ReadAlignedPolynomialCutJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o PolynomialNodeCutOptions) (*AlignedPolynomialCut, error) {
	recipe, sources, e := readPolynomialMixedCutRecipe(ctx, r, limits, nativeAlignedPolynomialCut, 2)
	if e != nil {
		return nil, e
	}
	out, e := CutAlignedPolynomialComponents(ctx, sources[0], sources[1], sources[2], *recipe.Axis, recipe.Tolerance, o)
	if e != nil {
		return nil, e
	}
	if e = recipe.verifyTypedContacts(out.material, out.orders[:], out.derivatives[:], limits); e != nil {
		return nil, e
	}
	return out, nil
}
