package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CombineRepresentable reuses a complete shell section to propose a native
// Boolean. All source coverage, split edges, endpoint geometry, face
// classification and sewing are proved again by the native assembler.
// A nonrepresentable algebraic terminal remains available in the section,
// but cannot be coerced into the current native Solid representation.
func (r AlgebraicShellSectionResult) CombineRepresentable(ctx context.Context, op parametric.BooleanOp,
	o SplineShellOpenOptions) (Solid, SplineShellOpenHistory, error) {
	var h SplineShellOpenHistory
	if ctx == nil || r.source == nil || op > parametric.Difference {
		return Solid{}, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	if len(r.source.sections) == 0 {
		x, _, e := CombineSeparatedSplineShells(ctx, r.source.a, r.source.b, op, o.Shells)
		return x, h, e
	}
	var sections []SplineShellSection
	for i, p := range r.source.sections {
		ids := r.source.pairs[i]
		open := 0
		for j, b := range p.Trace.BranchGraph.Branches {
			if e := ctx.Err(); e != nil {
				return Solid{}, h, e
			}
			x := SplineShellSection{FaceA: ids[0], FaceB: ids[1], Branch: j, Closed: b.Closed}
			if j == 0 {
				x.TubeCells = p.Trace.Seeds
			}
			if !b.Closed {
				if open >= len(p.Open) {
					return Solid{}, h, geom.ErrConvergence
				}
				for k, v := range p.Open[open].Ends {
					t := SplineShellOpenEndpoint{Operand: v.Operand, Edge: SplineTubeEndpoint{Side: v.Side}}
					for j, q := range v.Certificate.Parameters {
						c, e := q.Classify(ctx, geom.ExactCurvePlaneEventOptions{MaxOperations: o.Endpoints.MaxOperations,
							MaxCoefficientBits: o.Endpoints.MaxCoefficientBits})
						if e != nil {
							return Solid{}, h, e
						}
						if c.Kind != geom.ExactRootKindRational {
							return Solid{}, h, geom.ErrUnrepresentable
						}
						if _, e := geom.RepresentedExactScalar(c.Rational); e != nil {
							return Solid{}, h, e
						}
						t.Edge.Hint[j] = c.Rational
					}
					if k == 0 {
						x.Start = t
					} else {
						x.End = t
					}
				}
				open++
			}
			sections = append(sections, x)
		}
		if open != len(p.Open) {
			return Solid{}, h, geom.ErrConvergence
		}
	}
	return CombineSectionedSplineShells(ctx, r.source.a, r.source.b, op, sections, o)
}
