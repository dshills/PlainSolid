package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicShellAreaBounds encloses area in squared model units. Ambiguous
// cells cover the parts of the original charts that could straddle the trim.
type AlgebraicShellAreaBounds struct {
	Area                         geom.ExactParameterInterval
	VisitedCells, AmbiguousCells int
}

// BoundSurfaceArea encloses the boundary area on the original trimmed
// supports. Cells proved inside the retained face fragments contribute their
// full bounded area; unresolved cells contribute between zero and a rigorous
// upper derivative bound. The returned interval exists only on success.
func (s AlgebraicShellBoolean) BoundSurfaceArea(ctx context.Context, o AlgebraicShellMeasureOptions) (out AlgebraicShellAreaBounds, err error) {
	area := geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	limits := o.Shells.Embedding.Patches
	counts, e := s.visitBooleanSurfaceCells(ctx, o, func(p *geom.SurfacePatch, _ geom.Vec3, partial, _ bool) error {
		u, v := p.KnotBreaks()
		if len(u) < 2 || len(v) < 2 || (len(u)-1)*(len(v)-1) > 4096 {
			return geom.ErrConvergence
		}
		for i := 0; i+1 < len(u); i++ {
			for j := 0; j+1 < len(v); j++ {
				if e := ctx.Err(); e != nil {
					return e
				}
				q, e := p.Trim(geom.ParameterRectangle{UMin: u[i], UMax: u[i+1], VMin: v[j], VMax: v[j+1]})
				if e != nil {
					return e
				}
				lo, hi, e := geom.EncloseSurfaceCellArea(ctx, q, geom.SurfacePatchEvaluationOptions{
					MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits})
				if e != nil {
					return e
				}
				if math.IsInf(hi, 1) {
					return geom.ErrUnrepresentable
				}
				if !partial {
					area.Lower.Add(area.Lower, ratInset(lo))
				}
				area.Upper.Add(area.Upper, ratInset(hi))
			}
		}
		return nil
	})
	out.VisitedCells, out.AmbiguousCells = counts.VisitedCells, counts.AmbiguousCells
	if e == nil {
		out.Area = area
	}
	return out, e
}
