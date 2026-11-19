//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphOccurrenceAreaOptions reuses the existing affine-area source,
// image-cell and square-root limits. The exact rational pose cofactor has a
// separate 256-operation reservation; no float64 pose conversion is performed.
type AlgebraicGraphOccurrenceAreaOptions = AlgebraicGraphAffineAreaOptions

// AlgebraicGraphOccurrenceArea encloses the WORLD area of each retained native
// graph owner. Original remains in the source frame. Unknown rim cells add
// upper area only; known empty/hole cells add none. Pose and owner IDs are owned
// copies. No source geometry, topology or area is approximated by rounded poses.
type AlgebraicGraphOccurrenceArea struct {
	Original                                            AlgebraicGraphWholeMeasures
	Pose                                                [3][4]*big.Rat
	Owners                                              [2]AlgebraicGraphOwnerArea
	Total                                               [2]*big.Rat
	ImageCells, ImageQuotaCharged, CofactorQuotaCharged int
}

// BoundWholeSurfaceArea reuses one proved original whole partition and the
// exact cofactor-transformed tangent enclosure in graphCellAffineAreaDensity.
// It supports every immutable rational occurrence pose, including shear and
// reflection. Source degree/knot restrictions and all original limits remain
// unchanged. Failure/cancellation publishes no partial area certificate.
func (o *AlgebraicGraphOccurrence) BoundWholeSurfaceArea(ctx context.Context, options AlgebraicGraphOccurrenceAreaOptions) (out AlgebraicGraphOccurrenceArea, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphOccurrenceArea{ImageCells: out.ImageCells, ImageQuotaCharged: out.ImageQuotaCharged, CofactorQuotaCharged: out.CofactorQuotaCharged}
		}
	}()
	if ctx == nil || o == nil || o.query == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if options.MaxImageCells == 0 {
		options.MaxImageCells = 1024
	}
	if options.ImageCellOperations == 0 {
		options.ImageCellOperations = 65536
	}
	if options.RootBits == 0 {
		options.RootBits = 64
	}
	if options.MaxImageCells < 1 || options.MaxImageCells > 8192 || options.ImageCellOperations < 1 || options.ImageCellOperations > 16000000 || options.RootBits < 8 || options.RootBits > 256 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(options.Original.Refinement.CellOptions.Exact)
	if e != nil {
		return out, e
	}
	if exact.MaxOperations < 256 {
		return out, geom.ErrConvergence
	}
	out.CofactorQuotaCharged = 256
	preflight := exact
	preflight.MaxOperations = 256
	a := graphOccurrenceArithmetic{ctx: ctx, options: preflight}
	var cofactor [3][3]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			var rows, cols []int
			for k := 0; k < 3; k++ {
				if k != i {
					rows = append(rows, k)
				}
				if k != j {
					cols = append(cols, k)
				}
			}
			cofactor[i][j] = a.calc('-', a.calc('*', o.pose[rows[0]][cols[0]], o.pose[rows[1]][cols[1]]), a.calc('*', o.pose[rows[0]][cols[1]], o.pose[rows[1]][cols[0]]))
			if (i+j)%2 != 0 {
				cofactor[i][j].Neg(cofactor[i][j])
			}
		}
	}
	if a.err != nil {
		return out, a.err
	}
	q := o.query
	out.Original, err = q.BoundWholeMeasures(ctx, options.Original)
	if err != nil {
		return out, err
	}
	check := func(x *big.Rat) error {
		if !graphPolynomialBits(x, exact.MaxCoefficientBits) {
			return geom.ErrConvergence
		}
		return nil
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
			if out.ImageCells >= options.MaxImageCells || options.ImageCellOperations > int(^uint(0)>>1)-out.ImageQuotaCharged {
				return out, geom.ErrConvergence
			}
			out.ImageCells++
			out.ImageQuotaCharged += options.ImageCellOperations
			density, e := q.graphCellAffineAreaDensity(ctx, owner, leaf.Window, options.ImageCellOperations, exact.MaxCoefficientBits, options.RootBits, &cofactor)
			if e != nil {
				return out, e
			}
			for end := 0; end < 2; end++ {
				if end == 0 && !leaf.FullMaterial {
					continue
				}
				value := new(big.Rat).Mul(area, density[end])
				if e := check(value); e != nil {
					return out, e
				}
				out.Owners[owner].Area[end].Add(out.Owners[owner].Area[end], value)
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
	out.Pose = o.Pose()
	return out, nil
}
