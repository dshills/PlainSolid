package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphCapAttachment is the original stock coedge of an existing
// clipped cap use. UV and Model have the same exact increasing parameter
// and domain as the cap use. StockReversed is the opposite face traversal;
// it does not claim that the rest of that original face has been clipped.
type AlgebraicGraphCapAttachment struct {
	Source                  RationalGraphCapEdgeSource
	UV, Model               *geom.ExactComposedCurveCell
	StockReversed           bool
	ExactOperations, Splits int
}

// OriginalStockAttachment transports one retained cap boundary interval
// back to its original stock UV trim and proves equality along the entire
// model continuation. Generated cut seams have no stock coedge; use the
// cap-pair ownership proof for those. The private stock recipe and exact
// original branch data are used independently of edits to diagnostic wires.
func (c AlgebraicGraphCapClip) OriginalStockAttachment(ctx context.Context, wire, use int, o geom.SurfaceCurveAgreementOptions, roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapAttachment, error) {
	var zero AlgebraicGraphCapAttachment
	if o.MaxOperations < 0 || o.MaxCoefficientBits < 0 || o.MaxSubdivisions < 0 {
		return zero, geom.ErrInvalid
	}
	if ctx == nil || c.recipe == nil || wire < 0 || wire >= len(c.wires) || use < 0 || use >= len(c.wires[wire]) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if roots.MaxOperations == 0 {
		roots.MaxOperations = 2000000
	}
	if roots.MaxCoefficientBits == 0 {
		roots.MaxCoefficientBits = 16384
	}
	if roots.MaxSubdivisions == 0 {
		roots.MaxSubdivisions = 8192
	}
	if roots.MaxOperations < 1 || roots.MaxCoefficientBits < 64 || roots.MaxSubdivisions < 1 {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, roots.MaxOperations, roots.MaxSubdivisions, roots.MaxCoefficientBits}
	if o.MaxOperations == 0 || o.MaxOperations > b.work {
		o.MaxOperations = b.work
	}
	if o.MaxCoefficientBits == 0 || o.MaxCoefficientBits > b.bits {
		o.MaxCoefficientBits = b.bits
	}
	if o.MaxSubdivisions == 0 || o.MaxSubdivisions > b.subdiv {
		o.MaxSubdivisions = b.subdiv
	}
	u := c.wires[wire][use]
	src := u.Source
	if u.Seam >= 0 || src.Branch < 0 || src.Branch >= len(c.recipe.branches) || src.Face < 0 || src.Face >= len(c.recipe.stock.faces) {
		return zero, geom.ErrInvalid
	}
	branch := c.recipe.branches[src.Branch]
	if branch.Face != src.Face || src.Piece < 0 || src.Piece >= len(branch.Pieces) || src.Reversed {
		return zero, geom.ErrConvergence
	}
	patch, e := exactFaceSupportPatch(c.recipe.stock.faces[src.Face], o.SurfacePatchOptions)
	if e != nil {
		return zero, e
	}
	desc, e := polynomialTrimDescriptor(ctx, branch.Pieces[src.Piece], false, o.SurfacePatchOptions)
	if e != nil {
		return zero, e
	}
	if e := b.charge(desc.ExactOperations(), 0); e != nil {
		return zero, e
	}
	o.MaxOperations = min(o.MaxOperations, b.work)
	cells, e := patch.BuildSurfaceCurveCompositionCells(ctx, desc, o)
	if e != nil {
		return zero, e
	}
	if !cells.Complete || len(cells.Cells) != 1 {
		return zero, geom.ErrConvergence
	}
	if e := b.charge(cells.ExactOperations, cells.Subdivisions); e != nil {
		return zero, e
	}
	raw, e := polynomialPlaneControls(branch.Pieces[src.Piece])
	if e != nil {
		return zero, e
	}
	h := make([][4]*big.Rat, len(raw))
	for i, p := range raw {
		h[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), p[2]}
	}
	lim := geom.SurfacePatchOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits}
	uv, e := geom.NewExactHomogeneousBezier(ctx, h, lim)
	if e != nil {
		return zero, e
	}
	if e := b.charge(uv.ExactOperations(), 0); e != nil {
		return zero, e
	}
	lim.MaxOperations = b.work
	chart, work, e := geom.NewExactComposedBezierCellWithWork(ctx, uv, lim)
	if e != nil {
		return zero, e
	}
	if e := b.charge(work, 0); e != nil {
		return zero, e
	}
	lower, _ := u.Cell.EndpointRoot(0)
	upper, _ := u.Cell.EndpointRoot(1)
	chart, e = b.restrict(chart, lower, upper)
	if e != nil {
		return zero, e
	}
	curve, e := b.restrict(cells.Cells[0], lower, upper)
	if e != nil {
		return zero, e
	}
	agreement, e := u.Cell.CompareSameParameter(ctx, curve, b.limits())
	if e != nil {
		return zero, e
	}
	if !agreement.Equal {
		return zero, geom.ErrConvergence
	}
	if e := b.charge(agreement.Operations, agreement.Subdivisions); e != nil {
		return zero, e
	}
	return AlgebraicGraphCapAttachment{Source: src, UV: chart, Model: curve, StockReversed: !u.Reversed,
		ExactOperations: roots.MaxOperations - b.work, Splits: roots.MaxSubdivisions - b.subdiv}, nil
}
