//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CoincidentSimpleRegionOptions bounds original triangulation and all pairwise
// convex clipping cumulatively. MaxWork defaults to1M,max4M; MaxCells defaults
// to4096,max16384. Source uses and coefficient sizes use Regions.Material.Outer.
type CoincidentSimpleRegionOptions struct {
	Regions           CoincidentConvexRegionOptions
	MaxWork, MaxCells int
}

// BuildCoincidentSimpleRegions accepts independently certified simple straight
// outer rings, including nonorthogonal concave polygons, without holes. Exact
// triangulation partitions each original ring; triangle-pair intersections have
// disjoint interiors. All internal diagonals cancel before source-authenticated
// exposed loops are admitted. Pinches fail; isolated contacts are regularized out.
// Areas are exact UV areas, not curved model-space surface areas. The result
// is a chart arrangement, with no solid Boolean or mesh acceptance implied.
func (a Solid) BuildCoincidentSimpleRegions(ctx context.Context, fa int, b Solid, fb int, o CoincidentSimpleRegionOptions) (CoincidentChartRegions, error) {
	return a.buildCoincidentSimpleRegions(ctx, fa, b, fb, o, 0)
}

// BuildCoincidentSimpleHoledRegions additionally accepts one independently
// validated simple straight hole per face; the hole may itself be concave.
// Bridge diagonals are temporary and
// must cancel; all exposed hole ranges retain original source use ownership.
func (a Solid) BuildCoincidentSimpleHoledRegions(ctx context.Context, fa int, b Solid, fb int, o CoincidentSimpleRegionOptions) (CoincidentChartRegions, error) {
	return a.buildCoincidentSimpleRegions(ctx, fa, b, fb, o, 1)
}

// BuildCoincidentMultiHoledRegions accepts up to maxHoles original holes per
// face (1..8). Original loop/use indices are retained without canonicalizing
// input hole order. MaxUses bounds all outer and hole uses across both faces;
// triangulation, clipping and reconstruction retain their cumulative quotas.
func (a Solid) BuildCoincidentMultiHoledRegions(ctx context.Context, fa int, b Solid, fb int, maxHoles int, o CoincidentSimpleRegionOptions) (CoincidentChartRegions, error) {
	if maxHoles < 1 || maxHoles > 8 {
		return CoincidentChartRegions{}, geom.ErrInvalid
	}
	return a.buildCoincidentSimpleRegions(ctx, fa, b, fb, o, maxHoles)
}

func (a Solid) buildCoincidentSimpleRegions(ctx context.Context, fa int, b Solid, fb int, o CoincidentSimpleRegionOptions, maxHoles int) (CoincidentChartRegions, error) {
	fail := func(e error) (CoincidentChartRegions, error) { return CoincidentChartRegions{}, e }
	if ctx == nil || fa < 0 || fa >= len(a.faces) || fb < 0 || fb >= len(b.faces) {
		return fail(geom.ErrInvalid)
	}
	if o.MaxWork == 0 {
		o.MaxWork = 1000000
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxWork < 1 || o.MaxWork > 4000000 || o.MaxCells < 1 || o.MaxCells > 16384 {
		return fail(geom.ErrInvalid)
	}
	proofOptions := o.Regions.Material.Outer
	uses := proofOptions.MaxUses
	if uses == 0 {
		uses = 64
	}
	bits := proofOptions.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if uses < 6 || uses > 128 || bits < 64 || bits > 65536 {
		return fail(geom.ErrInvalid)
	}
	faces := [2]Face{a.faces[fa], b.faces[fb]}
	count := len(faces[0].Outer) + len(faces[1].Outer)
	for _, f := range faces {
		for _, h := range f.Holes {
			count += len(h)
		}
	}
	if count > uses {
		return fail(parametric.ErrBudget)
	}
	aa, bb := a, b
	aa.faces = append([]Face(nil), a.faces...)
	bb.faces = append([]Face(nil), b.faces...)
	var points [2][][2]*big.Rat
	var tris [2][][3]int
	work := 0
	check := func(r *big.Rat) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > o.MaxWork || r.Num().BitLen() > bits || r.Denom().BitLen() > bits {
			return parametric.ErrBudget
		}
		return nil
	}
	for source, f := range faces {
		if len(f.Holes) > maxHoles || len(f.Outer) < 3 {
			return fail(geom.ErrConvergence)
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if u.Curve != nil {
					return fail(geom.ErrConvergence)
				}
			}
		}
		validation, e := ValidateFaceTrimLoops(ctx, f, proofOptions.Trims)
		if e != nil {
			return fail(e)
		}
		if validation.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
		patch, e := exactFaceSupportPatch(f, proofOptions.Coincidence.Patches)
		if e != nil {
			return fail(e)
		}
		d := patch.Domain()
		var loops [][]int
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			var indices []int
			for _, u := range loop {
				if u.UVStart.X < d.UMin || u.UVStart.X > d.UMax || u.UVStart.Y < d.VMin || u.UVStart.Y > d.VMax {
					return fail(geom.ErrDomain)
				}
				points[source] = append(points[source], [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)})
				for _, r := range points[source][len(points[source])-1] {
					if e := check(r); e != nil {
						return fail(e)
					}
				}
				indices = append(indices, len(points[source])-1)
			}
			loops = append(loops, indices)
		}
		if work >= o.MaxWork {
			return fail(parametric.ErrBudget)
		}
		t, n, e := exactHoledFootprintTriangulation(ctx, points[source], loops, o.MaxWork-work)
		work += n
		if e != nil {
			return fail(e)
		}
		tris[source] = t
		// Whole-domain rectangles certify support identity/injectivity only. Their
		// synthetic edge IDs never enter the output source ownership ledger.
		uv := []geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
		f.Outer = nil
		f.Holes = nil
		for i, p := range uv {
			f.Outer = append(f.Outer, Use{UVStart: p, UVEnd: uv[(i+1)%4]})
		}
		if source == 0 {
			aa.faces[fa] = f
		} else {
			bb.faces[fb] = f
		}
	}
	support, e := aa.IntersectCoincidentConvexTrims(ctx, fa, bb, fb, proofOptions)
	if e != nil {
		return fail(e)
	}
	matrix := splineCoincidenceUVMatrix(support.Map)
	det := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(matrix[0][0], matrix[1][1]), new(big.Rat).Mul(matrix[0][1], matrix[1][0])))
	partition := CoincidentConvexHoleMaterial{Outer: support, UVArea: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	for _, ta := range tris[0] {
		for _, tb := range tris[1] {
			if e := check(new(big.Rat)); e != nil {
				return fail(e)
			}
			poly := lensUVPolygon{points[0][ta[0]], points[0][ta[1]], points[0][ta[2]]}
			for j, id := range tb {
				x, y := points[1][id], points[1][tb[(j+1)%3]]
				dx, dy := new(big.Rat).Sub(y[0], x[0]), new(big.Rat).Sub(y[1], x[1])
				var row [3]*big.Rat
				for k, col := range []int{0, 1, 3} {
					row[k] = new(big.Rat).Sub(new(big.Rat).Mul(dx, matrix[1][col]), new(big.Rat).Mul(dy, matrix[0][col]))
				}
				row[2].Add(row[2], new(big.Rat).Sub(new(big.Rat).Mul(dy, x[0]), new(big.Rat).Mul(dx, x[1])))
				for _, r := range row {
					if e := check(r); e != nil {
						return fail(e)
					}
				}
				poly, e = poly.clip(row, check)
				if e != nil {
					return fail(e)
				}
			}
			area, e := poly.area(check)
			if e != nil {
				return fail(e)
			}
			if area.Sign() == 0 {
				continue
			}
			var clean lensUVPolygon
			for _, p := range poly {
				if len(clean) == 0 || p[0].Cmp(clean[len(clean)-1][0]) != 0 || p[1].Cmp(clean[len(clean)-1][1]) != 0 {
					clean = append(clean, p)
				}
			}
			if len(clean) > 1 && clean[0][0].Cmp(clean[len(clean)-1][0]) == 0 && clean[0][1].Cmp(clean[len(clean)-1][1]) == 0 {
				clean = clean[:len(clean)-1]
			}
			cell := CoincidentConvexMaterialCell{A: clean, UVArea: [2]*big.Rat{area, new(big.Rat).Mul(area, det)}}
			for _, p := range clean {
				cell.B = append(cell.B, coincidentMapPoint(matrix, p))
			}
			partition.Cells = append(partition.Cells, cell)
			if len(partition.Cells) > o.MaxCells {
				return fail(parametric.ErrBudget)
			}
			for k := 0; k < 2; k++ {
				partition.UVArea[k].Add(partition.UVArea[k], cell.UVArea[k])
				if e := check(partition.UVArea[k]); e != nil {
					return fail(e)
				}
			}
		}
	}
	partition.Complete = true
	partition.Work = work
	return a.buildCoincidentConvexCells(ctx, fa, b, fb, partition, o.Regions)
}
