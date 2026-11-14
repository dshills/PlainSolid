package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedFaceCellSide retains all contributing original boundary occurrences.
// MateCell/MateSide identify the reverse occurrence on another output cell;
// both are -1 for an exposed union boundary. A side may have both ancestry and
// a mate (an original boundary becomes internal to the cofacial union).
type WrappedFaceCellSide struct {
	Original           []WrappedFaceFragmentOwner
	MateCell, MateSide int
}

// WrappedFaceCell is an independent exact rectangular face on A's immutable
// support, in A's original lifted chart and with A's Reversed flag. Owners is
// 1 for A, 2 for B, and 3 for their overlap. OwnerUV contains the four corners
// in each contributing input face's original chart, or nil entries otherwise.
// Interior-disjoint cells share only their explicitly paired sides/vertices.
type WrappedFaceCell struct {
	Face    SewFace
	Owners  uint8
	OwnerUV [2][4][2]*big.Rat
	Sides   [4]WrappedFaceCellSide
}

// WrappedFaceCells is a certified cofacial subdivision, not a closed shell or
// solid Boolean. B's independent orientation remains in FaceReversed. Output
// faces are not sewn. Each overlap cell appears once, with both source owners.
type WrappedFaceCells struct {
	Complete        bool
	Cells           []WrappedFaceCell
	FaceReversed    [2]bool
	ExactOperations int
}

// MaterializeWrappedRectangleFaceCells independently certifies the inputs and
// embedding using CertifyWrappedRectangleFaceOverlap, then materializes its
// exact disjoint chart cells. Every introduced UV and model vertex must be
// exactly representable in native storage. No rounded cut or vertex is used;
// all failures, including late budget exhaustion, return an empty result.
func MaterializeWrappedRectangleFaceCells(ctx context.Context, a, b SewFace, o WrappedRectangleFaceOverlapOptions) (WrappedFaceCells, error) {
	var zero WrappedFaceCells
	proof, e := CertifyWrappedRectangleFaceOverlap(ctx, a, b, o)
	if e != nil {
		return zero, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	used := proof.ExactOperations
	arrangement := proof.Embedding.Arrangement
	if proof.LiftedEmbedding != nil {
		arrangement = proof.LiftedEmbedding.Arrangement
	}
	check := func(q *big.Rat) error {
		used++
		if used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	source := a.Face.Surface.(*geom.WrappedSurfacePatch)
	target := b.Face.Surface.(*geom.WrappedSurfacePatch)
	var shift [2]*big.Rat
	for axis, p := range target.Periods() {
		shift[axis] = new(big.Rat)
		if p != nil {
			shift[axis].Mul(new(big.Rat).SetInt(arrangement.TargetPeriodShift[axis]), p)
		}
		if e := check(shift[axis]); e != nil {
			return zero, e
		}
	}
	grid := arrangement.Cells
	if len(grid) < 1 || len(grid) > 9 {
		return zero, geom.ErrConvergence
	}
	cells := make([]WrappedFaceCell, len(grid))
	inputs := [2]SewFace{a, b}
	points := map[string]geom.Vec3{}
	key := func(p [2]*big.Rat) string { return p[0].RatString() + "," + p[1].RatString() }
	type occurrence struct {
		cell, side int
		start, end string
	}
	sides := map[string][]occurrence{}
	for ci, g := range grid {
		c := &cells[ci]
		c.Owners = g.Owners
		c.Face.Face = Face{Surface: source, Reversed: a.Face.Reversed}
		exact := [4][2]*big.Rat{{g.A[0][0], g.A[1][0]}, {g.A[0][1], g.A[1][0]}, {g.A[0][1], g.A[1][1]}, {g.A[0][0], g.A[1][1]}}
		var corners [4]geom.Vec2
		for i, uv := range exact {
			corners[i], e = saddleRepresentedUV(uv)
			if e != nil {
				return zero, e
			}
			point, exists := points[key(uv)]
			if !exists {
				if used >= o.MaxOperations {
					return zero, geom.ErrConvergence
				}
				jet, err := source.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
				if err != nil {
					return zero, err
				}
				used += jet.ExactOperations
				point, e = geom.RepresentedExactPoint(jet.Point)
				if e != nil {
					return zero, e
				}
				points[key(uv)] = point
			}
			c.Face.Vertices = append(c.Face.Vertices, point)
			for owner := 0; owner < 2; owner++ {
				if c.Owners&(1<<owner) == 0 {
					continue
				}
				for axis := 0; axis < 2; axis++ {
					q := new(big.Rat).Set(uv[axis])
					if owner == 1 {
						q.Sub(q, shift[axis])
					}
					if e := check(q); e != nil {
						return zero, e
					}
					c.OwnerUV[owner][i][axis] = q
				}
			}
		}
		for i, start := range corners {
			j := (i + 1) % 4
			end := corners[j]
			c.Face.Edges = append(c.Face.Edges, parametric.Edge{Start: i, End: j, Curve: surfaceTrimCurve{surface: source, a: start, b: end}, Range: parametric.Interval{Max: 1}})
			c.Face.Face.Outer = append(c.Face.Face.Outer, Use{Edge: i, UVStart: start, UVEnd: end})
			c.Sides[i].MateCell = -1
			c.Sides[i].MateSide = -1
			from, to := key(exact[i]), key(exact[j])
			lo, hi := from, to
			if hi < lo {
				lo, hi = hi, lo
			}
			sides[lo+";"+hi] = append(sides[lo+";"+hi], occurrence{ci, i, from, to})
			for owner, f := range inputs {
				if c.Owners&(1<<owner) == 0 {
					continue
				}
				uv := [2][2]*big.Rat{c.OwnerUV[owner][i], c.OwnerUV[owner][j]}
				for ui, u := range f.Face.Outer {
					s := [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}
					t := [2]*big.Rat{ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}
					axis := 0
					if s[0].Cmp(t[0]) == 0 {
						axis = 1
					}
					fixed := axis ^ 1
					if uv[0][fixed].Cmp(s[fixed]) != 0 || uv[1][fixed].Cmp(s[fixed]) != 0 {
						continue
					}
					var parameters [2]*big.Rat
					inside := true
					for end := 0; end < 2; end++ {
						fraction := new(big.Rat).Quo(new(big.Rat).Sub(uv[end][axis], s[axis]), new(big.Rat).Sub(t[axis], s[axis]))
						if fraction.Sign() < 0 || fraction.Cmp(big.NewRat(1, 1)) > 0 {
							inside = false
							break
						}
						if u.Reversed {
							fraction.Sub(big.NewRat(1, 1), fraction)
						}
						span := f.Edges[u.Edge].Range
						fraction.Mul(fraction, new(big.Rat).Sub(ratInset(span.Max), ratInset(span.Min)))
						fraction.Add(fraction, ratInset(span.Min))
						if e := check(fraction); e != nil {
							return zero, e
						}
						parameters[end] = fraction
					}
					if inside {
						c.Sides[i].Original = append(c.Sides[i].Original, WrappedFaceFragmentOwner{Face: owner, Use: ui, Edge: u.Edge, Parameters: parameters, UV: uv})
					}
				}
			}
		}
	}
	for _, occurrences := range sides {
		switch len(occurrences) {
		case 1:
			p := occurrences[0]
			if len(cells[p.cell].Sides[p.side].Original) == 0 {
				return zero, geom.ErrConvergence
			}
		case 2:
			p, q := occurrences[0], occurrences[1]
			if p.cell == q.cell || p.start != q.end || p.end != q.start {
				return zero, geom.ErrConvergence
			}
			cells[p.cell].Sides[p.side].MateCell = q.cell
			cells[p.cell].Sides[p.side].MateSide = q.side
			cells[q.cell].Sides[q.side].MateCell = p.cell
			cells[q.cell].Sides[q.side].MateSide = p.side
		default:
			return zero, geom.ErrConvergence
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return WrappedFaceCells{Complete: true, Cells: cells, FaceReversed: proof.FaceReversed, ExactOperations: used}, nil
}
