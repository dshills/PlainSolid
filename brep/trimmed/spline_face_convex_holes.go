//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CoincidentConvexHoleOptions bounds the additional fragmentation separately
// from the outer-support certificate. MaxCells defaults to 4096 (maximum 16384).
type CoincidentConvexHoleOptions struct {
	Outer    CoincidentConvexOverlapOptions
	MaxCells int
}

// CoincidentConvexMaterialCell is a closed convex cell of positive area. Cells
// have disjoint interiors; their common partition edges are not source trims.
// Boundary ranges identify portions of original uses on this cell's boundary.
// This is an area partition, not a merged wire or a closed-set contact ledger.
// A Boundary entry denotes the clipped source interval, not an entire cell edge.
type CoincidentConvexMaterialCell struct {
	A, B     [][2]*big.Rat
	UVArea   [2]*big.Rat
	Boundary []CoincidentChartUse
}
type CoincidentConvexHoleMaterial struct {
	Outer    CoincidentConvexOverlap
	Cells    []CoincidentConvexMaterialCell
	UVArea   [2]*big.Rat
	Work     int
	Complete bool
}

// IntersectCoincidentConvexHoleMaterial partitions the regularized intersection
// of two strictly convex straight outer loops, each with at most one strictly
// convex straight hole. It makes no claim about isolated point/edge contacts.
func (a Solid) IntersectCoincidentConvexHoleMaterial(ctx context.Context, fa int, b Solid, fb int, o CoincidentConvexHoleOptions) (CoincidentConvexHoleMaterial, error) {
	fail := func(e error) (CoincidentConvexHoleMaterial, error) { return CoincidentConvexHoleMaterial{}, e }
	if ctx == nil || fa < 0 || fa >= len(a.faces) || fb < 0 || fb >= len(b.faces) {
		return fail(geom.ErrInvalid)
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxCells < 1 || o.MaxCells > 16384 {
		return fail(geom.ErrInvalid)
	}
	limit := o.Outer.MaxUses
	if limit == 0 {
		limit = 64
	}
	workLimit := o.Outer.MaxWork
	if workLimit == 0 {
		workLimit = 100000
	}
	bits := o.Outer.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	faces := [2]Face{a.faces[fa], b.faces[fb]}
	count := 0
	for _, f := range faces {
		count += len(f.Outer)
		for _, h := range f.Holes {
			count += len(h)
		}
	}
	if count > limit {
		return fail(parametric.ErrBudget)
	}
	work := 0
	check := func(r *big.Rat) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > workLimit || r.Num().BitLen() > bits || r.Denom().BitLen() > bits {
			return parametric.ErrBudget
		}
		return nil
	}
	for _, f := range faces {
		if len(f.Holes) > 1 {
			return fail(geom.ErrConvergence)
		}
		proof, e := ValidateFaceTrimLoops(ctx, f, o.Outer.Trims)
		if e != nil {
			return fail(e)
		}
		if proof.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
		for _, h := range f.Holes {
			if len(h) < 3 {
				return fail(geom.ErrConvergence)
			}
			for i, u := range h {
				if u.Curve != nil {
					return fail(geom.ErrConvergence)
				}
				v := h[(i+1)%len(h)].UVEnd
				cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(ratInset(u.UVEnd.X), ratInset(u.UVStart.X)), new(big.Rat).Sub(ratInset(v.Y), ratInset(u.UVEnd.Y))), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(u.UVEnd.Y), ratInset(u.UVStart.Y)), new(big.Rat).Sub(ratInset(v.X), ratInset(u.UVEnd.X))))
				if e := check(cross); e != nil {
					return fail(e)
				}
				if cross.Sign() >= 0 {
					return fail(geom.ErrConvergence)
				}
			}
		}
	}
	aa, bb := a, b
	aa.faces = append([]Face(nil), a.faces...)
	bb.faces = append([]Face(nil), b.faces...)
	aa.faces[fa].Holes = nil
	bb.faces[fb].Holes = nil
	outer, e := aa.IntersectCoincidentConvexTrims(ctx, fa, bb, fb, o.Outer)
	if e != nil {
		return fail(e)
	}
	result := CoincidentConvexHoleMaterial{Outer: outer, UVArea: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	if outer.Dimension != 2 {
		result.Complete = true
		result.Work = work
		return result, nil
	}
	matrix := splineCoincidenceUVMatrix(outer.Map)
	identity := [3][4]*big.Rat{}
	for i := range identity {
		for j := range identity[i] {
			identity[i][j] = new(big.Rat)
		}
	}
	identity[0][0].SetInt64(1)
	identity[1][1].SetInt64(1)
	rowFor := func(u Use, source int, insideHole bool) [3]*big.Rat {
		m := identity
		if source == 1 {
			m = matrix
		}
		x, y := ratInset(u.UVStart.X), ratInset(u.UVStart.Y)
		dx, dy := new(big.Rat).Sub(ratInset(u.UVEnd.X), x), new(big.Rat).Sub(ratInset(u.UVEnd.Y), y)
		var row [3]*big.Rat
		for j, col := range []int{0, 1, 3} {
			row[j] = new(big.Rat).Sub(new(big.Rat).Mul(dx, m[1][col]), new(big.Rat).Mul(dy, m[0][col]))
		}
		row[2].Add(row[2], new(big.Rat).Sub(new(big.Rat).Mul(dy, x), new(big.Rat).Mul(dx, y)))
		if insideHole {
			for _, r := range row {
				r.Neg(r)
			}
		}
		return row
	}
	initial := lensUVPolygon{}
	for _, v := range outer.Vertices {
		initial = append(initial, coincidentCopyPoint(v.A))
	}
	cells := []lensUVPolygon{initial}
	for source, f := range faces {
		for _, hole := range f.Holes {
			var next []lensUVPolygon
			for _, cell := range cells {
				remaining := cell
				for _, u := range hole {
					row := rowFor(u, source, true)
					outside := [3]*big.Rat{}
					for i, r := range row {
						if e := check(r); e != nil {
							return fail(e)
						}
						outside[i] = new(big.Rat).Neg(r)
					}
					piece, e := remaining.clip(outside, check)
					if e != nil {
						return fail(e)
					}
					area, e := piece.area(check)
					if e != nil {
						return fail(e)
					}
					if area.Sign() > 0 {
						next = append(next, piece)
						if len(next) > o.MaxCells {
							return fail(parametric.ErrBudget)
						}
					}
					remaining, e = remaining.clip(row, check)
					if e != nil {
						return fail(e)
					}
					if len(remaining) == 0 {
						break
					}
				}
			}
			cells = next
		}
	}
	determinant := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(matrix[0][0], matrix[1][1]), new(big.Rat).Mul(matrix[0][1], matrix[1][0])))
	for _, cell := range cells {
		clean := make(lensUVPolygon, 0, len(cell))
		for _, p := range cell {
			if len(clean) == 0 || p[0].Cmp(clean[len(clean)-1][0]) != 0 || p[1].Cmp(clean[len(clean)-1][1]) != 0 {
				clean = append(clean, p)
			}
		}
		if len(clean) > 1 && clean[0][0].Cmp(clean[len(clean)-1][0]) == 0 && clean[0][1].Cmp(clean[len(clean)-1][1]) == 0 {
			clean = clean[:len(clean)-1]
		}
		cell = clean
		area, e := cell.area(check)
		if e != nil {
			return fail(e)
		}
		c := CoincidentConvexMaterialCell{UVArea: [2]*big.Rat{area, new(big.Rat).Mul(area, determinant)}}
		for _, p := range cell {
			c.A = append(c.A, coincidentCopyPoint(p))
			c.B = append(c.B, coincidentMapPoint(matrix, p))
		}
		// Clip every original source use to this closed convex cell. Positive
		// intervals preserve original direction and do not label artificial cuts.
		for source, f := range faces {
			loops := append([][]Use{f.Outer}, f.Holes...)
			points := c.A
			if source == 1 {
				points = c.B
			}
			orientation := 1
			if source == 1 {
				d := new(big.Rat).Sub(new(big.Rat).Mul(matrix[0][0], matrix[1][1]), new(big.Rat).Mul(matrix[0][1], matrix[1][0]))
				orientation = d.Sign()
			}
			for li, loop := range loops {
				for ui, u := range loop {
					lo, hi := new(big.Rat), big.NewRat(1, 1)
					valid := true
					onBoundary := false
					for i, p := range points {
						q := points[(i+1)%len(points)]
						dx, dy := new(big.Rat).Sub(q[0], p[0]), new(big.Rat).Sub(q[1], p[1])
						val := func(v geom.Vec2) *big.Rat {
							return new(big.Rat).Mul(big.NewRat(int64(orientation), 1), new(big.Rat).Sub(new(big.Rat).Mul(dx, new(big.Rat).Sub(ratInset(v.Y), p[1])), new(big.Rat).Mul(dy, new(big.Rat).Sub(ratInset(v.X), p[0]))))
						}
						v, w := val(u.UVStart), val(u.UVEnd)
						s := new(big.Rat).Sub(w, v)
						if v.Sign() == 0 && w.Sign() == 0 {
							onBoundary = true
						}
						if e := check(s); e != nil {
							return fail(e)
						}
						if s.Sign() == 0 {
							if v.Sign() < 0 {
								valid = false
								break
							}
							continue
						}
						t := new(big.Rat).Quo(new(big.Rat).Neg(v), s)
						if e := check(t); e != nil {
							return fail(e)
						}
						if s.Sign() > 0 && t.Cmp(lo) > 0 {
							lo = t
						}
						if s.Sign() < 0 && t.Cmp(hi) < 0 {
							hi = t
						}
					}
					if valid && onBoundary && lo.Cmp(hi) < 0 {
						c.Boundary = append(c.Boundary, CoincidentChartUse{Owner: CoincidentTrimOwner{source, []int{fa, fb}[source], li, ui, u.Edge}, Range: [2]*big.Rat{lo, hi}})
					}
				}
			}
		}
		result.Cells = append(result.Cells, c)
		for k := 0; k < 2; k++ {
			result.UVArea[k].Add(result.UVArea[k], c.UVArea[k])
			if e := check(result.UVArea[k]); e != nil {
				return fail(e)
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	result.Complete = true
	result.Work = work
	return result, nil
}
