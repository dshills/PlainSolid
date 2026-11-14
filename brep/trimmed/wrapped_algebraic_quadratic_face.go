package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// wrappedAlgebraicTrimCurve preserves the original evaluator and unsplit
// authored edge. The unexported marker selects bounded algebraic agreement.
// Existing rational-only recipe encoders do not recognize this new curve.
type wrappedAlgebraicTrimCurve struct {
	surfaceTrimCurve
	proof *geom.WrappedCurveCells
}

// WrappedAlgebraicQuadraticFace is a certified embedded open face whose proof
// cells may end at irrational seam/knot roots. Native topology retains only
// original represented UV/model vertices and unsplit exact authored trims.
// This separate type makes no persistence, closed-shell or volume claim.
type WrappedAlgebraicQuadraticFace struct{ face *WrappedQuadraticFace }

func (r *WrappedAlgebraicQuadraticFace) NativeFace() SewFace {
	if r == nil {
		return SewFace{}
	}
	return r.face.NativeFace()
}
func (r *WrappedAlgebraicQuadraticFace) OriginalLoops() [][]ExactQuadraticUV {
	if r == nil {
		return nil
	}
	return r.face.OriginalLoops()
}
func (r *WrappedAlgebraicQuadraticFace) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.face.ExactOperations()
}
func (r *WrappedAlgebraicQuadraticFace) Composition(loop, edge int) (*geom.WrappedCurveCells, error) {
	if r == nil || r.face == nil || loop < 0 || loop >= len(r.face.algebraicProofs) || edge < 0 || edge >= len(r.face.algebraicProofs[loop]) {
		return nil, geom.ErrInvalid
	}
	return r.face.algebraicProofs[loop][edge], nil
}

// NewWrappedAlgebraicQuadraticFace is the opt-in counterpart of the strict
// rational-seam constructor. The same exact loop/embedding and representable
// endpoint gates apply. MaxPieces is capped at8192; all construction and whole
// edge comparisons share MaxOperations. No seam cut becomes a native vertex.
func NewWrappedAlgebraicQuadraticFace(ctx context.Context, w *geom.WrappedSurfacePatch, outer []ExactQuadraticUV, holes [][]ExactQuadraticUV, o WrappedQuadraticFaceOptions) (*WrappedAlgebraicQuadraticFace, error) {
	if o.MaxPieces > 8192 {
		return nil, geom.ErrInvalid
	}
	face, e := newWrappedQuadraticFace(ctx, w, outer, holes, o, true)
	if e != nil {
		return nil, e
	}
	return &WrappedAlgebraicQuadraticFace{face: face}, nil
}

func compareWrappedAlgebraicQuadraticUse(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, int, error) {
	marker, ok := model.Curve.(wrappedAlgebraicTrimCurve)
	if !ok || model.Range != (parametric.Interval{Max: 1}) {
		return false, 0, nil
	}
	m := marker.surfaceTrimCurve
	qa, ok := m.curve.(ExactQuadraticUV)
	if !ok {
		return false, 0, nil
	}
	qb, ok := use.Curve.(ExactQuadraticUV)
	if !ok {
		return false, 0, nil
	}
	abase, am := parametric.SurfaceMapping(m.surface)
	bbase, bm := parametric.SurfaceMapping(surface)
	a, ok := abase.(*geom.WrappedSurfacePatch)
	if !ok {
		return false, 0, nil
	}
	b, ok := bbase.(*geom.WrappedSurfacePatch)
	if !ok || am != bm {
		return false, 0, nil
	}
	if o.MaxWork == 0 {
		o.MaxWork = 8000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 4096
	}
	if o.MaxWork < 1 || o.MaxBits < 64 || o.MaxIntervals < 1 || o.MaxIntervals > 8192 {
		return false, 0, geom.ErrInvalid
	}
	used := 0
	var groups [2]*geom.WrappedCurveCells
	for i, q := range []ExactQuadraticUV{qa, qb} {
		if used >= o.MaxWork {
			return false, used, geom.ErrConvergence
		}
		uv, e := q.modelParameterCurve(ctx, geom.SurfacePatchOptions{MaxOperations: o.MaxWork - used, MaxCoefficientBits: o.MaxBits})
		if e != nil {
			return false, used, e
		}
		used += uv.ExactOperations()
		if used >= o.MaxWork {
			return false, used, geom.ErrConvergence
		}
		w := a
		if i == 1 {
			w = b
		}
		groups[i], e = w.BuildParameterCurveCells(ctx, uv, geom.WrappedCurveCellsOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: o.MaxWork - used, MaxCoefficientBits: o.MaxBits}, MaxEvents: o.MaxIntervals + 1})
		if e != nil {
			return false, used, e
		}
		used += groups[i].ExactOperations()
	}
	if used >= o.MaxWork {
		return false, used, geom.ErrConvergence
	}
	result, e := groups[0].CompareModelBounded(ctx, groups[1], use.Reversed, o.MaxIntervals, geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxWork - used, MaxCoefficientBits: o.MaxBits})
	used += result.ExactOperations
	return e == nil && result.Proven && result.Equivalent, used, e
}
