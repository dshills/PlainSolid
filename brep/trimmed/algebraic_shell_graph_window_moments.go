package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphWindowMomentOptions retains the polynomial cell proof and
// refinement limits. MomentCellOperations defaults to262144, maximum16M per
// retained cell. It includes original coefficient extraction and degree<=12
// products for the residual-coordinate moment. No mesh participates.
type AlgebraicGraphWindowMomentOptions struct {
	Polynomial           AlgebraicGraphPolynomialWindowOptions
	MomentCellOperations int
}
type AlgebraicGraphExactCellMoments struct {
	Leaf         int
	FirstMoments [3]*big.Rat
}

// AlgebraicGraphWindowMoments bounds world-coordinate first moments. Unknown
// cells contribute signed interval moments using their nonnegative volume
// bound and full source-coordinate enclosure. HasCentroid requires a proved
// positive total volume lower bound; centroid intervals need not be attained.
// ExactCells retain original native face ownership through Volume.Bounds.Leaves.
type AlgebraicGraphWindowMoments struct {
	Volume                 AlgebraicGraphPolynomialWindowIntegral
	FirstMoments, Centroid [3][2]*big.Rat
	HasCentroid            bool
	ExactCells             []AlgebraicGraphExactCellMoments
	MomentQuotaCharged     int
}

func (q *AlgebraicGraphMaterialQuery) IntegrateWindowMoments(ctx context.Context, w AlgebraicGraphProjectionWindow, o AlgebraicGraphWindowMomentOptions) (out AlgebraicGraphWindowMoments, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphWindowMoments{MomentQuotaCharged: out.MomentQuotaCharged}
		}
	}()
	if o.MomentCellOperations == 0 {
		o.MomentCellOperations = 262144
	}
	if o.MomentCellOperations < 1 || o.MomentCellOperations > 16000000 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(o.Polynomial.Refinement.CellOptions.Exact)
	if e != nil {
		return out, e
	}
	check := func(x *big.Rat) error {
		if !graphPolynomialBits(x, exact.MaxCoefficientBits) {
			return geom.ErrConvergence
		}
		return nil
	}
	out.Volume, err = q.IntegratePolynomialWindow(ctx, w, o.Polynomial)
	if err != nil {
		return out, err
	}
	var residual [2]*big.Rat
	for _, p := range q.patches {
		b, e := p.Bounds()
		if e != nil {
			return out, e
		}
		lo := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}
		hi := [3]float64{b.Max.X, b.Max.Y, b.Max.Z}
		if !finite(lo[q.residual]) || !finite(hi[q.residual]) {
			return out, geom.ErrConvergence
		}
		a, z := ratInset(lo[q.residual]), ratInset(hi[q.residual])
		if e := check(a); e != nil {
			return out, e
		}
		if e := check(z); e != nil {
			return out, e
		}
		if residual[0] == nil || a.Cmp(residual[0]) < 0 {
			residual[0] = a
		}
		if residual[1] == nil || z.Cmp(residual[1]) > 0 {
			residual[1] = z
		}
	}
	for axis := 0; axis < 3; axis++ {
		out.FirstMoments[axis] = [2]*big.Rat{new(big.Rat), new(big.Rat)}
	}
	known := map[int]*big.Rat{}
	for _, cell := range out.Volume.ExactCells {
		known[cell.Leaf] = cell.Volume
	}
	for i, leaf := range out.Volume.Bounds.Leaves {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if leaf.NoFootprint {
			continue
		}
		var moments [3][2]*big.Rat
		if leaf.FullMaterial {
			if o.MomentCellOperations > int(^uint(0)>>1)-out.MomentQuotaCharged {
				return out, geom.ErrConvergence
			}
			out.MomentQuotaCharged += o.MomentCellOperations
			value, e := q.integrateGraphPolynomialCellMode(ctx, leaf, o.MomentCellOperations, exact.MaxCoefficientBits, 1)
			if e != nil {
				return out, e
			}
			v, ok := known[i]
			if !ok || value[0].Cmp(v) != 0 {
				return out, geom.ErrConvergence
			}
			record := AlgebraicGraphExactCellMoments{Leaf: i}
			for axis := 0; axis < 3; axis++ {
				moments[axis] = [2]*big.Rat{value[axis+1], value[axis+1]}
				record.FirstMoments[axis] = new(big.Rat).Set(value[axis+1])
			}
			out.ExactCells = append(out.ExactCells, record)
		} else if leaf.Unresolved {
			area := new(big.Rat).Mul(new(big.Rat).Sub(leaf.Window.Max[0], leaf.Window.Min[0]), new(big.Rat).Sub(leaf.Window.Max[1], leaf.Window.Min[1]))
			if e := check(area); e != nil {
				return out, e
			}
			v := new(big.Rat).Mul(area, leaf.Depth[1])
			if e := check(v); e != nil {
				return out, e
			}
			var coordinates [3][2]*big.Rat
			coordinates[q.residual] = residual
			for projection, axis := range q.axes {
				coordinates[axis] = [2]*big.Rat{leaf.Window.Min[projection], leaf.Window.Max[projection]}
			}
			for axis, c := range coordinates {
				a, z := new(big.Rat).Mul(v, c[0]), new(big.Rat).Mul(v, c[1])
				if e := check(a); e != nil {
					return out, e
				}
				if e := check(z); e != nil {
					return out, e
				}
				if a.Sign() > 0 {
					a.SetInt64(0)
				}
				if z.Sign() < 0 {
					z.SetInt64(0)
				}
				moments[axis] = [2]*big.Rat{a, z}
			}
		} else {
			return out, geom.ErrConvergence
		}
		for axis := 0; axis < 3; axis++ {
			for end := 0; end < 2; end++ {
				out.FirstMoments[axis][end].Add(out.FirstMoments[axis][end], moments[axis][end])
				if e := check(out.FirstMoments[axis][end]); e != nil {
					return out, e
				}
			}
		}
	}
	if out.Volume.Bounds.Volume[0].Sign() > 0 {
		for axis := 0; axis < 3; axis++ {
			for _, m := range out.FirstMoments[axis] {
				for _, v := range out.Volume.Bounds.Volume {
					x := new(big.Rat).Quo(m, v)
					if e := check(x); e != nil {
						return out, e
					}
					if out.Centroid[axis][0] == nil || x.Cmp(out.Centroid[axis][0]) < 0 {
						out.Centroid[axis][0] = new(big.Rat).Set(x)
					}
					if out.Centroid[axis][1] == nil || x.Cmp(out.Centroid[axis][1]) > 0 {
						out.Centroid[axis][1] = new(big.Rat).Set(x)
					}
				}
			}
			allowed := residual
			for projection, a := range q.axes {
				if axis == a {
					allowed = [2]*big.Rat{w.Min[projection], w.Max[projection]}
				}
			}
			if out.Centroid[axis][0].Cmp(allowed[0]) < 0 {
				out.Centroid[axis][0].Set(allowed[0])
			}
			if out.Centroid[axis][1].Cmp(allowed[1]) > 0 {
				out.Centroid[axis][1].Set(allowed[1])
			}
			if out.Centroid[axis][0].Cmp(out.Centroid[axis][1]) > 0 {
				return out, geom.ErrConvergence
			}
		}
		out.HasCentroid = true
	}
	return out, nil
}
