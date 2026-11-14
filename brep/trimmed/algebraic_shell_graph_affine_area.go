package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphAffineAreaOptions reuses one original whole partition. Original
// keeps its existing limits. ImageCellOperations defaults to 65536 per owner/leaf
// (maximum 16M), MaxImageCells to 1024 (maximum 8192). RootBits defaults to 64,
// range 8..256. The extra exact cofactor preflight reserves 256 operations against
// Original.Refinement.CellOptions.Exact, separately from its per-cell work.
type AlgebraicGraphAffineAreaOptions struct {
	Original                                     AlgebraicGraphWindowAreaOptions
	MaxImageCells, ImageCellOperations, RootBits int
}

// AlgebraicGraphAffineArea encloses retained surface area in the exact affine
// IMAGE, independently for each original graph owner. Original and its partition
// remain in original coordinates; image area is not determinant-scaled volume.
// Only material leaves contribute lower bounds; unresolved rim leaves contribute
// upper bounds and known empty/hole leaves contribute zero. No new native shell
// or affine-image boundary classification is claimed.
type AlgebraicGraphAffineArea struct {
	Original                                            AlgebraicGraphWholeMeasures
	Placement                                           geom.Affine
	Owners                                              [2]AlgebraicGraphOwnerArea
	Total                                               [2]*big.Rat
	ImageCells, ImageQuotaCharged, CofactorQuotaCharged int
}

// BoundAffineWholeSurfaceArea applies the exact cofactor to graph area vectors
// per original partition leaf, then encloses their norm with rational square
// root bounds. Translation has no effect on tangents. No surface-area scaling
// by the affine determinant is used.
func (q *AlgebraicGraphMaterialQuery) BoundAffineWholeSurfaceArea(ctx context.Context, m geom.Affine, o AlgebraicGraphAffineAreaOptions) (out AlgebraicGraphAffineArea, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphAffineArea{ImageCells: out.ImageCells, ImageQuotaCharged: out.ImageQuotaCharged, CofactorQuotaCharged: out.CofactorQuotaCharged}
		}
	}()
	if ctx == nil || q == nil || !m.IsFinite() {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxImageCells == 0 {
		o.MaxImageCells = 1024
	}
	if o.ImageCellOperations == 0 {
		o.ImageCellOperations = 65536
	}
	if o.RootBits == 0 {
		o.RootBits = 64
	}
	if o.MaxImageCells < 1 || o.MaxImageCells > 8192 || o.ImageCellOperations < 1 || o.ImageCellOperations > 16000000 || o.RootBits < 8 || o.RootBits > 256 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(o.Original.Refinement.CellOptions.Exact)
	if e != nil {
		return out, e
	}
	if exact.MaxOperations < 256 {
		return out, geom.ErrConvergence
	}
	out.CofactorQuotaCharged = 256
	check := func(x *big.Rat) error {
		if !graphPolynomialBits(x, exact.MaxCoefficientBits) {
			return geom.ErrConvergence
		}
		return nil
	}
	var a, cofactor [3][3]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			a[i][j] = ratInset(m[i][j])
			if e := check(a[i][j]); e != nil {
				return out, e
			}
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			rows, cols := []int{}, []int{}
			for k := 0; k < 3; k++ {
				if k != i {
					rows = append(rows, k)
				}
				if k != j {
					cols = append(cols, k)
				}
			}
			x := new(big.Rat).Mul(a[rows[0]][cols[0]], a[rows[1]][cols[1]])
			y := new(big.Rat).Mul(a[rows[0]][cols[1]], a[rows[1]][cols[0]])
			if e := check(x); e != nil {
				return out, e
			}
			if e := check(y); e != nil {
				return out, e
			}
			cofactor[i][j] = new(big.Rat).Sub(x, y)
			if (i+j)%2 != 0 {
				cofactor[i][j].Neg(cofactor[i][j])
			}
			if e := check(cofactor[i][j]); e != nil {
				return out, e
			}
		}
	}
	det := new(big.Rat)
	for j := 0; j < 3; j++ {
		term := new(big.Rat).Mul(a[0][j], cofactor[0][j])
		if e := check(term); e != nil {
			return out, e
		}
		det.Add(det, term)
		if e := check(det); e != nil {
			return out, e
		}
	}
	if det.Sign() == 0 {
		return out, geom.ErrConvergence
	}
	out.Original, err = q.BoundWholeMeasures(ctx, o.Original)
	if err != nil {
		return out, err
	}
	for owner := 0; owner < 2; owner++ {
		out.Owners[owner] = AlgebraicGraphOwnerArea{Operand: owner, NativeFaces: append([]int(nil), out.Original.Boundary.Owners[owner].NativeFaces...), Area: [2]*big.Rat{new(big.Rat), new(big.Rat)}}
	}
	for _, leaf := range out.Original.Boundary.Partition.Leaves {
		if leaf.NoFootprint {
			continue
		}
		if e := ctx.Err(); e != nil {
			return out, e
		}
		area := new(big.Rat).Mul(new(big.Rat).Sub(leaf.Window.Max[0], leaf.Window.Min[0]), new(big.Rat).Sub(leaf.Window.Max[1], leaf.Window.Min[1]))
		if e := check(area); e != nil {
			return out, e
		}
		for owner := 0; owner < 2; owner++ {
			if out.ImageCells >= o.MaxImageCells || o.ImageCellOperations > int(^uint(0)>>1)-out.ImageQuotaCharged {
				return out, geom.ErrConvergence
			}
			out.ImageCells++
			out.ImageQuotaCharged += o.ImageCellOperations
			density, e := q.graphCellAffineAreaDensity(ctx, owner, leaf.Window, o.ImageCellOperations, exact.MaxCoefficientBits, o.RootBits, &cofactor)
			if e != nil {
				return out, e
			}
			for end := 0; end < 2; end++ {
				if end == 0 && !leaf.FullMaterial {
					continue
				}
				v := new(big.Rat).Mul(area, density[end])
				if e := check(v); e != nil {
					return out, e
				}
				out.Owners[owner].Area[end].Add(out.Owners[owner].Area[end], v)
				if e := check(out.Owners[owner].Area[end]); e != nil {
					return out, e
				}
			}
		}
	}
	for end := 0; end < 2; end++ {
		out.Total[end] = new(big.Rat).Add(out.Owners[0].Area[end], out.Owners[1].Area[end])
		if e := check(out.Total[end]); e != nil {
			return out, e
		}
	}
	out.Placement = m
	return out, nil
}
