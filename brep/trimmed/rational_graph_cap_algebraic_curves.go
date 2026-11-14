package trimmed

import (
	"context"

	"plainsolid/geom"
)

// AlgebraicGraphShellCurve is the authoritative model continuation of one
// edge in the exact indexed boundary. Each cell has its original algebraic or
// rational domain. Reversed describes indexed edge traversal; cells
// themselves always use increasing parameter.
type AlgebraicGraphShellCurve struct {
	Cells    geom.SurfaceCurveCompositionCells
	Reversed bool
}

// ExactModelBoundary independently reconstructs the closed indexed shell and
// its exact model curves. Original perimeter edges keep their own parameter,
// original cap/stock attachments keep their trim curve parameter, and shared
// cap seams are represented once. Returned diagnostics cannot alter the pair.
func (c AlgebraicGraphCapPair) ExactModelBoundary(ctx context.Context, o RationalGraphShellSectionOptions,
	roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphShellBoundary, []AlgebraicGraphShellCurve, error) {
	var zero AlgebraicGraphShellBoundary
	index, e := c.ExactIndexedBoundary(ctx, o, roots)
	if e != nil {
		return zero, nil, e
	}
	if roots.MaxOperations == 0 {
		roots.MaxOperations = 2000000
	}
	if roots.MaxSubdivisions == 0 {
		roots.MaxSubdivisions = 8192
	}
	if roots.MaxCoefficientBits == 0 {
		roots.MaxCoefficientBits = 16384
	}
	b := capClipBudget{ctx, roots.MaxOperations, roots.MaxSubdivisions, roots.MaxCoefficientBits}
	var curves []AlgebraicGraphShellCurve
	s := c.caps[0].recipe.stock
	for _, x := range index.Edges {
		if e := ctx.Err(); e != nil {
			return zero, nil, e
		}
		if x.StockEdge < 0 {
			cap, owner := x.Cap, x.CapBoundary
			if x.CapSeam >= 0 {
				cap, owner = 0, c.seams[x.CapSeam].Owners[0]
			}
			u := c.caps[cap].wires[owner.Wire][owner.Use]
			curves = append(curves, AlgebraicGraphShellCurve{Cells: geom.SurfaceCurveCompositionCells{Complete: true, Cells: []*geom.ExactComposedCurveCell{u.Cell}}, Reversed: u.Reversed})
			continue
		}
		composition := geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: o.Spans.Faces.Patches, MaxSubdivisions: b.subdiv}
		if composition.MaxOperations == 0 || composition.MaxOperations > b.work {
			composition.MaxOperations = b.work
		}
		if composition.MaxCoefficientBits == 0 || composition.MaxCoefficientBits > b.bits {
			composition.MaxCoefficientBits = b.bits
		}
		m, _, e := algebraicSeamSource(ctx, s, x.StockEdge, composition)
		if e != nil {
			return zero, nil, e
		}
		full, e := m.curve.ModelCurveCells(ctx, m.patch, composition)
		if e != nil || !full.Complete || len(full.Cells) == 0 {
			if e == nil {
				e = geom.ErrConvergence
			}
			return zero, nil, e
		}
		if e := b.charge(full.ExactOperations, full.Subdivisions); e != nil {
			return zero, nil, e
		}
		interval := AlgebraicSubdivisionEdge{}
		for k, p := range x.StockParameters {
			r, used, e := geom.NewExactRationalRoot(ctx, p, b.limits())
			if e != nil {
				return zero, nil, e
			}
			if e := b.charge(used, 0); e != nil {
				return zero, nil, e
			}
			if k == 0 {
				interval.Lower = r
			} else {
				interval.Upper = r
			}
		}
		part, e := restrictSubdivisionCells(ctx, full, interval, b.limits())
		if e != nil || !part.Complete || len(part.Cells) == 0 {
			if e == nil {
				e = geom.ErrConvergence
			}
			return zero, nil, e
		}
		if e := b.charge(part.ExactOperations, part.Subdivisions); e != nil {
			return zero, nil, e
		}
		curves = append(curves, AlgebraicGraphShellCurve{Cells: part})
	}
	return index, curves, nil
}
