package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// MultiSpanCurvedTrimMeshOptions preserves the existing mesh/trim budgets.
// Span bounds each exact triangle partition. MaxSpanCells and MaxSpanPieces
// bound cumulative work over ALL triangles/refinement passes; each defaults
// to100000 with maximum1M. Cells include the initial whole-chart Lipschitz proof.
type MultiSpanCurvedTrimMeshOptions struct {
	MaxResidualOperations       int // Cumulative higher-degree residual work; default128M,maximum1B.
	Mesh                        CurvedQuadraticTrimMeshOptions
	Span                        MultiSpanTriangleBoundOptions
	MaxSpanCells, MaxSpanPieces int
}

// TessellateMultiSpanCurvedTrimRegionCertified extends curved-trim image-distance
// certification to C0 multi-span positive rational patches of degree<=2 per
// coordinate. It proves a global Lipschitz bound from every exact knot cell,
// transports the certified UV fill bound, and clips each residual triangle at
// all knots. The shared exact rational UV grid remains conforming even when
// triangles cross knots. It is not a source or display embedding certificate.
// Exhausted cumulative cell/piece/triangle limits return no partial mesh.
func TessellateMultiSpanCurvedTrimRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []SplineUV, holes [][]SplineUV, o MultiSpanCurvedTrimMeshOptions) (CertifiedCurvedQuadraticTrimMesh, error) {
	empty := CertifiedCurvedQuadraticTrimMesh{}
	if o.MaxSpanCells == 0 {
		o.MaxSpanCells = 100000
	}
	if o.MaxSpanPieces == 0 {
		o.MaxSpanPieces = 100000
	}
	if o.Span.MaxCells == 0 {
		o.Span.MaxCells = 1024
	}
	if o.Span.MaxPieces == 0 {
		o.Span.MaxPieces = 8192
	}
	if o.Span.MaxInputBits == 0 {
		o.Span.MaxInputBits = 16384
	}
	if o.MaxSpanCells < 1 || o.MaxSpanCells > 1000000 || o.MaxSpanPieces < 1 || o.MaxSpanPieces > 1000000 || o.Span.MaxCells < 1 || o.Span.MaxCells > 65536 || o.Span.MaxPieces < 1 || o.Span.MaxPieces > 1000000 || o.Span.MaxInputBits < 1 || o.Span.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	work := multiSpanMeshWork{options: o}
	return tessellateCurvedQuadraticTrimWithSpans(ctx, p, outer, holes, o.Mesh, 1, &work)
}

type multiSpanMeshWork struct {
	options                 MultiSpanCurvedTrimMeshOptions
	cells, pieces           int
	operations              int
	mappedDisplay           mappedRoofDisplayOptions
	mappedStraightTrimImage bool
}

// A segment crossing finitely many C0 knot cells is absolutely continuous.
// Integrating the cellwise derivative bound along it proves the same global
// Lipschitz constant on the convex domain, including points on knot joins.
func (w *multiSpanMeshWork) lipschitz(ctx context.Context, p *geom.SurfacePatch) (*big.Rat, *big.Rat, error) {
	us, vs := p.KnotBreaks()
	if len(us) < 2 || len(vs) < 2 {
		return nil, nil, geom.ErrInvalid
	}
	if len(us)-1 > (w.options.MaxSpanCells-w.cells)/(len(vs)-1) {
		return nil, nil, geom.ErrConvergence
	}
	maximum, sum := new(big.Rat), new(big.Rat)
	for i := 0; i+1 < len(us); i++ {
		for j := 0; j+1 < len(vs); j++ {
			if e := ctx.Err(); e != nil {
				return nil, nil, e
			}
			cell, e := p.Trim(geom.ParameterRectangle{UMin: us[i], UMax: us[i+1], VMin: vs[j], VMax: vs[j+1]})
			w.cells++
			if e != nil {
				return nil, nil, e
			}
			l2, l1, ok := quadraticTensorLipschitz(cell)
			if !ok && w.options.Span.MaxDegree > 2 {
				l2, l1, ok = cell.RationalBezierDerivativeBounds(w.options.Span.MaxDegree)
			}
			if !ok {
				return nil, nil, geom.ErrConvergence
			}
			if l2.Cmp(maximum) > 0 {
				maximum = l2
			}
			if l1.Cmp(sum) > 0 {
				sum = l1
			}
		}
	}
	return maximum, sum, nil
}

func (w *multiSpanMeshWork) bound(ctx context.Context, p *geom.SurfacePatch, uv [3][2]*big.Rat, model [3]geom.Vec3) (*big.Rat, error) {
	options := w.options.Span
	if options.MaxDegree > 2 {
		limit := w.options.MaxResidualOperations
		if limit == 0 {
			limit = 128000000
		}
		if limit < 1 || limit > 1000000000 {
			return nil, geom.ErrInvalid
		}
		remaining := limit - w.operations
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		if options.MaxResidualOperations == 0 {
			options.MaxResidualOperations = 2000000
		}
		if options.MaxResidualOperations > remaining {
			options.MaxResidualOperations = remaining
		}
	}
	cells, pieces := w.options.MaxSpanCells-w.cells, w.options.MaxSpanPieces-w.pieces
	if cells < 1 || pieces < 1 {
		return nil, geom.ErrConvergence
	}
	if options.MaxCells > cells {
		options.MaxCells = cells
	}
	if options.MaxPieces > pieces {
		options.MaxPieces = pieces
	}
	result, e := BoundRationalMultiSpanSplineTriangleDeviation(ctx, p, uv, model, options)
	if e != nil {
		return nil, e
	}
	w.operations += result.ResidualOperations
	w.cells += result.Cells
	w.pieces += len(result.Pieces)
	return result.SquaredDistance, nil
}

// Exact quotient-rule cancellation precedes the tensor Bernstein hull. The
// six Jacobian numerator coordinates have bidegree at most(4,4); a common
// positive W² denominator then bounds their Frobenius norm simultaneously.
func quadraticTensorLipschitz(p *geom.SurfacePatch) (*big.Rat, *big.Rat, bool) {
	c, minW, ok := p.RationalBiquadraticCoefficients()
	if !ok {
		return nil, nil, false
	}
	var q [2][3][5][5]*big.Rat
	for axis := 0; axis < 2; axis++ {
		for k := 0; k < 3; k++ {
			for i := 0; i < 5; i++ {
				for j := 0; j < 5; j++ {
					q[axis][k][i][j] = new(big.Rat)
				}
			}
		}
	}
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					for axis := 0; axis < 2; axis++ {
						x, y, factor := a+i-1, b+j, a-i
						if axis == 1 {
							x, y, factor = a+i, b+j-1, b-j
						}
						if factor == 0 {
							continue
						}
						for k := 0; k < 3; k++ {
							term := new(big.Rat).Mul(big.NewRat(int64(factor), 1), new(big.Rat).Mul(c[a][b][k], c[i][j][3]))
							q[axis][k][x][y].Add(q[axis][k][x][y], term)
						}
					}
				}
			}
		}
	}
	choose := func(n, k int) int64 {
		r := int64(1)
		for i := 1; i <= k; i++ {
			r = r * int64(n-i+1) / int64(i)
		}
		return r
	}
	d := p.Domain()
	width := [2]*big.Rat{new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin)), new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))}
	denominator := new(big.Rat).Mul(minW, minW)
	maximum, sum := new(big.Rat), new(big.Rat)
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			norm, l1 := new(big.Rat), new(big.Rat)
			for axis := 0; axis < 2; axis++ {
				for k := 0; k < 3; k++ {
					control := new(big.Rat)
					for a := 0; a <= i; a++ {
						for b := 0; b <= j; b++ {
							factor := big.NewRat(choose(i, a)*choose(j, b), choose(4, a)*choose(4, b))
							control.Add(control, new(big.Rat).Mul(q[axis][k][a][b], factor))
						}
					}
					control.Quo(control, new(big.Rat).Mul(width[axis], denominator))
					norm.Add(norm, new(big.Rat).Mul(control, control))
					l1.Add(l1, new(big.Rat).Abs(control))
				}
			}
			if norm.Cmp(maximum) > 0 {
				maximum = norm
			}
			if l1.Cmp(sum) > 0 {
				sum = l1
			}
		}
	}
	return maximum, sum, true
}
