//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicShellAreaBounds encloses area in squared model units. Ambiguous
// cells cover the parts of the original charts that could straddle the trim.
type AlgebraicShellAreaBounds struct {
	Area                         geom.ExactParameterInterval
	VisitedCells, AmbiguousCells int
}

// MixedOriginalFaceAreaBounds encloses the part of one original support face
// contributing to a mixed Boolean result. Separate retained fragments on
// that face are totaled together. An omitted face has zero area; unresolved
// trim cells can contribute a positive upper bound even if no face survives.
// SelectedCells counts retained and unresolved terminal cells only.
type MixedOriginalFaceAreaBounds struct {
	Operand, OriginalFace         int
	Area                          geom.ExactParameterInterval
	SelectedCells, AmbiguousCells int
}

// MixedOriginalSurfaceAreaBounds reports the aggregate and its contributions
// in original operand and face order. Their exact interval sums equal Area.
type MixedOriginalSurfaceAreaBounds struct {
	AlgebraicShellAreaBounds
	Faces []MixedOriginalFaceAreaBounds
}

// BoundOriginalFaceAreas reports retained source-face areas without
// substituting triangle areas. Exhausting the work budget yields no ledger.
func (s *MixedAlgebraicShellBoolean) BoundOriginalFaceAreas(ctx context.Context, o AlgebraicShellMeasureOptions) (MixedOriginalSurfaceAreaBounds, error) {
	var zero MixedOriginalSurfaceAreaBounds
	if s == nil || s.section == nil || s.boundary.source == nil {
		return zero, geom.ErrInvalid
	}
	a, b := len(s.boundary.source.a.faces), len(s.boundary.source.b.faces)
	faces := make([]MixedOriginalFaceAreaBounds, 0, a+b)
	for op, size := range [2]int{a, b} {
		for id := 0; id < size; id++ {
			faces = append(faces, MixedOriginalFaceAreaBounds{Operand: op, OriginalFace: id, Area: zeroAreaInterval()})
		}
	}
	total, err := s.boundary.boundSurfaceArea(ctx, o, func(op, id int, partial bool, area geom.ExactParameterInterval) {
		if op == 1 {
			id += a
		}
		row := &faces[id]
		row.Area.Lower.Add(row.Area.Lower, area.Lower)
		row.Area.Upper.Add(row.Area.Upper, area.Upper)
		row.SelectedCells++
		if partial {
			row.AmbiguousCells++
		}
	})
	if err != nil {
		return zero, err
	}
	return MixedOriginalSurfaceAreaBounds{AlgebraicShellAreaBounds: total, Faces: faces}, nil
}

// BoundSurfaceArea encloses the boundary area on the original trimmed
// supports. Cells proved inside the retained face fragments contribute their
// full bounded area; unresolved cells contribute between zero and a rigorous
// upper derivative bound. The returned interval exists only on success.
func (s AlgebraicShellBoolean) BoundSurfaceArea(ctx context.Context, o AlgebraicShellMeasureOptions) (AlgebraicShellAreaBounds, error) {
	return s.boundSurfaceArea(ctx, o, nil)
}

func zeroAreaInterval() geom.ExactParameterInterval {
	return geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
}

func (s AlgebraicShellBoolean) boundSurfaceArea(ctx context.Context, o AlgebraicShellMeasureOptions,
	visit func(int, int, bool, geom.ExactParameterInterval)) (out AlgebraicShellAreaBounds, err error) {
	area := zeroAreaInterval()
	limits := o.Shells.Embedding.Patches
	counts, e := s.visitBooleanSurfaceCells(ctx, o, func(p *geom.SurfacePatch, _ geom.Vec3, partial, _ bool, operand, face int) error {
		u, v := p.KnotBreaks()
		if len(u) < 2 || len(v) < 2 || (len(u)-1)*(len(v)-1) > 4096 {
			return geom.ErrConvergence
		}
		cell := zeroAreaInterval()
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
				if !finite(lo) || !finite(hi) || lo < 0 || hi < lo {
					return geom.ErrUnrepresentable
				}
				if !partial {
					cell.Lower.Add(cell.Lower, ratInset(lo))
				}
				cell.Upper.Add(cell.Upper, ratInset(hi))
			}
		}
		area.Lower.Add(area.Lower, cell.Lower)
		area.Upper.Add(area.Upper, cell.Upper)
		if visit != nil {
			visit(operand, face, partial, cell)
		}
		return nil
	})
	out.VisitedCells, out.AmbiguousCells = counts.VisitedCells, counts.AmbiguousCells
	if e == nil {
		e = ctx.Err()
	}
	if e == nil {
		out.Area = area
	}
	return out, e
}
