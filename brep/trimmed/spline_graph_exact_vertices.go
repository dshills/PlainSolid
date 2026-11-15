package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// graphVertexEvaluator is an internal construction policy, never serialized as
// an asserted proof. The native edges retain the original surfaces and pcurves.
type graphVertexEvaluator func(*geom.BSplineSurface, openSectionPiece) (geom.Vec3, error)

// exactGraphVertexEvaluator evaluates original represented UV exactly and only
// accepts losslessly represented XYZ. It does not snap a computed display point.
// One cumulative quota covers source extraction and every vertex evaluation;
// cached patches are immutable and bound to their original surface pointers.
func exactGraphVertexEvaluator(ctx context.Context, o geom.GraphPointContactOptions) graphVertexEvaluator {
	maxWork, bits := o.MaxOperations, o.MaxCoefficientBits
	if maxWork == 0 {
		maxWork = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	used := 0
	patches := map[*geom.BSplineSurface]*geom.SurfacePatch{}
	return func(s *geom.BSplineSurface, x openSectionPiece) (geom.Vec3, error) {
		var zero geom.Vec3
		if ctx == nil || s == nil || maxWork < 1 || bits < 64 {
			return zero, geom.ErrInvalid
		}
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if used >= maxWork {
			return zero, geom.ErrConvergence
		}
		p := patches[s]
		if p == nil {
			var e error
			p, e = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{MaxOperations: maxWork - used, MaxCoefficientBits: bits})
			if e != nil {
				return zero, e
			}
			used += p.ExactOperations()
			patches[s] = p
		}
		if used >= maxWork {
			return zero, geom.ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(x.a.X), new(big.Rat).SetFloat64(x.a.Y), geom.SurfacePatchEvaluationOptions{MaxOperations: maxWork - used, MaxCoefficientBits: bits})
		used += jet.ExactOperations
		if e != nil {
			return zero, e
		}
		xyz := [3]float64{}
		for i, v := range jet.Point {
			xyz[i], e = geom.RepresentedExactScalar(v)
			if e != nil {
				return zero, e
			}
		}
		return geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}, nil
	}
}
