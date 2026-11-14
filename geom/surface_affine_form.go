package geom

import (
	"context"
	"math/big"
)

// CertifySurfaceAffineForm proves normal dot model = uv[0]*U+uv[1]*V+offset
// on the complete original positive-weight tensor chart. All original knot
// cells are compared after exact bidegree elevation; neither chart nor model
// coefficients are rounded. The covector and at least one UV coefficient must
// be nonzero. This is a finite chart identity, not a periodic quotient identity.
func CertifySurfaceAffineForm(ctx context.Context, p *SurfacePatch, normal [3]*big.Rat, uv [2]*big.Rat, offset *big.Rat, o SurfaceAffineCoordinateOptions) (SurfaceAffineCoordinateProof, error) {
	zero := SurfaceAffineCoordinateProof{}
	if ctx == nil || p == nil || offset == nil {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return zero, e
	}
	if o.MaxCells == 0 {
		o.MaxCells = 256
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 {
		return zero, ErrInvalid
	}
	if len(p.net) > opts.MaxPoles {
		return zero, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	nz := false
	for _, v := range normal {
		if v == nil {
			return zero, ErrInvalid
		}
		if e = b.check(v); e != nil {
			return zero, e
		}
		nz = nz || v.Sign() != 0
	}
	if !nz {
		return zero, ErrInvalid
	}
	nz = false
	for _, v := range uv {
		if v == nil {
			return zero, ErrInvalid
		}
		if e = b.check(v); e != nil {
			return zero, e
		}
		nz = nz || v.Sign() != 0
	}
	if !nz {
		return zero, ErrInvalid
	}
	if e = b.check(offset); e != nil {
		return zero, e
	}
	for _, h := range p.net {
		for _, v := range h {
			if e = b.check(v); e != nil {
				return zero, e
			}
		}
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return zero, e
	}
	for _, c := range cells {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		d := c.Domain()
		lo := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
		hi := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.VMax)}
		a := new(big.Rat).Set(offset)
		var delta [2]*big.Rat
		for k := 0; k < 2; k++ {
			term := new(big.Rat).Mul(uv[k], lo[k])
			if e = b.check(term); e != nil {
				return zero, e
			}
			a.Add(a, term)
			if e = b.check(a); e != nil {
				return zero, e
			}
			delta[k] = new(big.Rat).Mul(uv[k], new(big.Rat).Sub(hi[k], lo[k]))
			if e = b.check(delta[k]); e != nil {
				return zero, e
			}
		}
		for i := 0; i <= c.degreeU+1; i++ {
			for j := 0; j <= c.degreeV+1; j++ {
				if e = ctx.Err(); e != nil {
					return zero, e
				}
				t := big.NewRat(int64(i), int64(c.degreeU+1))
				s := big.NewRat(int64(j), int64(c.degreeV+1))
				wu := [2]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), t), t}
				wv := [2]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), s), s}
				residual := new(big.Rat)
				for du := 0; du < 2; du++ {
					for dv := 0; dv < 2; dv++ {
						ii, jj := i-du, j-dv
						if ii < 0 || ii > c.degreeU || jj < 0 || jj > c.degreeV {
							continue
						}
						h := c.net[ii*c.countV+jj]
						x := new(big.Rat)
						for k := 0; k < 3; k++ {
							term := new(big.Rat).Mul(normal[k], h[k])
							if e = b.check(term); e != nil {
								return zero, e
							}
							x.Add(x, term)
							if e = b.check(x); e != nil {
								return zero, e
							}
						}
						coefficient := new(big.Rat).Set(a)
						if du == 1 {
							coefficient.Add(coefficient, delta[0])
						}
						if dv == 1 {
							coefficient.Add(coefficient, delta[1])
						}
						if e = b.check(coefficient); e != nil {
							return zero, e
						}
						term := new(big.Rat).Mul(coefficient, h[3])
						if e = b.check(term); e != nil {
							return zero, e
						}
						x.Sub(x, term)
						if e = b.check(x); e != nil {
							return zero, e
						}
						x.Mul(x, wu[du])
						if e = b.check(x); e != nil {
							return zero, e
						}
						x.Mul(x, wv[dv])
						if e = b.check(x); e != nil {
							return zero, e
						}
						residual.Add(residual, x)
						if e = b.check(residual); e != nil {
							return zero, e
						}
					}
				}
				if residual.Sign() != 0 {
					return SurfaceAffineCoordinateProof{Cells: len(cells), ExactOperations: b.operations}, nil
				}
			}
		}
	}
	return SurfaceAffineCoordinateProof{Proven: true, Cells: len(cells), ExactOperations: b.operations}, nil
}
