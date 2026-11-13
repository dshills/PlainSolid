package trimmed

import (
	"context"
	"plainsolid/geom"
)

// ExactBezierSeamMeshOptions bounds three independent proof phases: whole
// seam agreement, model composition, and display tessellation. Agreement's
// arithmetic/event limits apply to each of the first two phases; Display's
// operation/segment limits apply globally to the resulting polyline.
type ExactBezierSeamMeshOptions struct {
	Agreement geom.SurfaceCurveAgreementOptions
	Display   geom.ComposedCollectionPolylineOptions
}

// CertifiedExactBezierSeamMesh is one shared model polyline with oppositely
// oriented original trim owners. Parameters are normalized to SourceA; the
// corresponding SourceB parameter is exactly 1-t. UsesB reverses both segment
// order and endpoint indices. No coordinate welding or approximate fitting is
// used to establish ownership. This proves curve coverage and two-sided chord
// deviation, including display rounding; it proves no face fill or embedding.
type CertifiedExactBezierSeamMesh struct {
	SourceA, SourceB ExactBezierUV
	Agreement        geom.SurfaceCurveAlgebraicAgreement
	Polyline         geom.CertifiedComposedCollectionPolyline
	UsesB            [][2]int
}

// TessellateExactBezierSeamCertified accepts native cubic/quartic rational
// trims with opposite parameter correspondence on positive-weight C0 supports.
// Whole model equality is proved before any shared display indices are issued.
// Agreement retains both owners' algebraic knot events; display subdivision
// follows A, whose proven identical model bounds also apply to B. Incomplete proofs, exceeded
// budgets and display-degenerate segments fail atomically with an empty result.
func TessellateExactBezierSeamCertified(ctx context.Context, a *geom.SurfacePatch, trimA ExactBezierUV, b *geom.SurfacePatch, trimB ExactBezierUV, o ExactBezierSeamMeshOptions) (CertifiedExactBezierSeamMesh, error) {
	var empty CertifiedExactBezierSeamMesh
	if ctx == nil || a == nil || b == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	ca, e := trimA.descriptor(ctx, o.Agreement.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	cb, e := trimB.reversed().descriptor(ctx, o.Agreement.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	proof, e := geom.CompareSurfaceCurveCompositions(ctx, a, ca, b, cb, o.Agreement)
	if e != nil {
		return empty, e
	}
	if !proof.Complete || !proof.Equal {
		return empty, geom.ErrConvergence
	}
	cells, e := a.BuildSurfaceCurveCompositionCells(ctx, ca, o.Agreement)
	if e != nil {
		return empty, e
	}
	if !cells.Complete {
		return empty, geom.ErrConvergence
	}
	line, e := cells.TessellateCertified(ctx, o.Display)
	if e != nil {
		return empty, e
	}
	if !line.Complete || line.DisplayDegenerate {
		return empty, geom.ErrConvergence
	}
	r := CertifiedExactBezierSeamMesh{SourceA: trimA, SourceB: trimB, Agreement: proof, Polyline: line, UsesB: make([][2]int, len(line.Segments))}
	for i, s := range line.Segments {
		r.UsesB[len(line.Segments)-1-i] = [2]int{s.Vertices[1], s.Vertices[0]}
	}
	return r, nil
}
