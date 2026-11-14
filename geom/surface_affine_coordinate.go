package geom

import (
	"context"
	"math/big"
)

// SurfaceAffineCoordinateOptions bounds a whole original tensor chart proof.
type SurfaceAffineCoordinateOptions struct {
	SurfacePatchOptions
	MaxCells int
}
type SurfaceAffineCoordinateProof struct {
	Proven                 bool
	Cells, ExactOperations int
}

// CertifySurfaceAffineCoordinate proves model[modelAxis] = scale*UV[parameterAxis]
// + offset over the full positive-weight rational patch, across every knot cell.
// It compares exact homogeneous Bernstein coefficients after one degree
// elevation, rather than sampling or requiring polynomial weights. Nonidentity
// returns Proven=false; arithmetic/resource failure returns an empty result.
func CertifySurfaceAffineCoordinate(ctx context.Context, p *SurfacePatch, modelAxis, parameterAxis int, scale, offset *big.Rat, o SurfaceAffineCoordinateOptions) (SurfaceAffineCoordinateProof, error) {
	if ctx == nil || p == nil || modelAxis < 0 || modelAxis > 2 || parameterAxis < 0 || parameterAxis > 1 || scale == nil || offset == nil || scale.Sign() == 0 {
		return SurfaceAffineCoordinateProof{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SurfaceAffineCoordinateProof{}, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return SurfaceAffineCoordinateProof{}, e
	}
	if o.MaxCells == 0 {
		o.MaxCells = 256
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 {
		return SurfaceAffineCoordinateProof{}, ErrInvalid
	}
	if len(p.net) > opts.MaxPoles {
		return SurfaceAffineCoordinateProof{}, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, q := range []*big.Rat{scale, offset} {
		if e := b.check(q); e != nil {
			return SurfaceAffineCoordinateProof{}, e
		}
	}
	for _, h := range p.net {
		for _, q := range h {
			if e := b.check(q); e != nil {
				return SurfaceAffineCoordinateProof{}, e
			}
		}
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return SurfaceAffineCoordinateProof{}, e
	}
	for _, cell := range cells {
		if e := ctx.Err(); e != nil {
			return SurfaceAffineCoordinateProof{}, e
		}
		d := cell.Domain()
		lo, hi := d.UMin, d.UMax
		degree, rows := cell.degreeU, cell.countV
		if parameterAxis == 1 {
			lo, hi = d.VMin, d.VMax
			degree, rows = cell.degreeV, cell.countU
		}
		a := new(big.Rat).Add(offset, new(big.Rat).Mul(scale, new(big.Rat).SetFloat64(lo)))
		delta := new(big.Rat).Mul(scale, new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)))
		for _, q := range []*big.Rat{a, delta} {
			if e := b.check(q); e != nil {
				return SurfaceAffineCoordinateProof{}, e
			}
		}
		at := func(row, k int) exactHomogeneous {
			if parameterAxis == 0 {
				return cell.net[k*cell.countV+row]
			}
			return cell.net[row*cell.countV+k]
		}
		for row := 0; row < rows; row++ {
			if e := ctx.Err(); e != nil {
				return SurfaceAffineCoordinateProof{}, e
			}
			for k := 0; k <= degree+1; k++ {
				t := big.NewRat(int64(k), int64(degree+1))
				one := new(big.Rat).Sub(big.NewRat(1, 1), t)
				x, w, tw := new(big.Rat), new(big.Rat), new(big.Rat)
				if k > 0 {
					h := at(row, k-1)
					x.Mul(t, h[modelAxis])
					tw.Mul(t, h[3])
					w.Set(tw)
				}
				if k <= degree {
					h := at(row, k)
					x.Add(x, new(big.Rat).Mul(one, h[modelAxis]))
					w.Add(w, new(big.Rat).Mul(one, h[3]))
				}
				residual := new(big.Rat).Sub(x, new(big.Rat).Add(new(big.Rat).Mul(a, w), new(big.Rat).Mul(delta, tw)))
				for _, q := range []*big.Rat{x, w, tw, residual} {
					if e := b.check(q); e != nil {
						return SurfaceAffineCoordinateProof{}, e
					}
				}
				if residual.Sign() != 0 {
					return SurfaceAffineCoordinateProof{Cells: len(cells), ExactOperations: b.operations}, nil
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return SurfaceAffineCoordinateProof{}, e
	}
	return SurfaceAffineCoordinateProof{Proven: true, Cells: len(cells), ExactOperations: b.operations}, nil
}
