package geom

import (
	"context"
	"math/big"
)

// SurfaceAffineTriangleSeparation proves complete disjointness of two original
// closed triangular restrictions. Margins are positive in the supplied raw
// covector's scalar units, not normalized Euclidean distances. Injectivity of
// either individual restriction is intentionally not inferred.
type SurfaceAffineTriangleSeparation struct {
	source     [2]*SurfacePatch
	triangles  [2][3][2]*big.Rat
	separator  SurfaceAffineTriangleSeparator
	sides      [2]int
	margins    [2]*big.Rat
	operations int
}

func (p *SurfaceAffineTriangleSeparation) Sides() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.sides
}
func (p *SurfaceAffineTriangleSeparation) Margins() [2]*big.Rat {
	if p == nil {
		return [2]*big.Rat{}
	}
	return [2]*big.Rat{new(big.Rat).Set(p.margins[0]), new(big.Rat).Set(p.margins[1])}
}
func (p *SurfaceAffineTriangleSeparation) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}
func (p *SurfaceAffineTriangleSeparation) Separator() SurfaceAffineTriangleSeparator {
	if p == nil {
		return SurfaceAffineTriangleSeparator{}
	}
	return (&SurfaceAffineTriangleMate{separator: p.separator}).Separator()
}
func (p *SurfaceAffineTriangleSeparation) Source(i int) (*SurfacePatch, [3][2]*big.Rat, error) {
	if p == nil {
		return nil, [3][2]*big.Rat{}, ErrInvalid
	}
	return (&SurfaceAffineTriangleMate{source: p.source, triangles: p.triangles}).Source(i)
}

// CertifySurfaceAffineTriangleSeparation proves the exact bivariate affine
// covector identities across all original tensor knot cells, then checks every
// triangle vertex strictly in its owner's halfspace. Affinity makes these
// extrema complete over each closed triangle, including every boundary edge.
// Touching hulls are rejected; no nonadjacent chain contact is silently ignored.
func CertifySurfaceAffineTriangleSeparation(ctx context.Context, a, b *SurfacePatch, triangles [2][3][2]*big.Rat, q SurfaceAffineTriangleSeparator, o SurfaceAffineCoordinateOptions) (*SurfaceAffineTriangleSeparation, error) {
	if ctx == nil || a == nil || b == nil || q.Level == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	if e = budget.check(q.Level); e != nil {
		return nil, e
	}
	out := &SurfaceAffineTriangleSeparation{source: [2]*SurfacePatch{a, b}, separator: q}
	for owner, p := range out.source {
		if budget.operations >= opts.MaxOperations {
			return nil, ErrConvergence
		}
		co := o
		co.SurfacePatchOptions = opts
		co.MaxOperations -= budget.operations
		proof, e := CertifySurfaceAffineForm(ctx, p, q.Normal, q.UV[owner], q.Offset[owner], co)
		if e != nil {
			return nil, e
		}
		budget.operations += proof.ExactOperations
		if !proof.Proven {
			return nil, ErrConvergence
		}
		d := p.Domain()
		lo := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
		hi := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.VMax)}
		for i, point := range triangles[owner] {
			if e = ctx.Err(); e != nil {
				return nil, e
			}
			value := new(big.Rat).Sub(q.Offset[owner], q.Level)
			if e = budget.check(value); e != nil {
				return nil, e
			}
			for j, v := range point {
				if v == nil {
					return nil, ErrInvalid
				}
				if e = budget.check(v); e != nil {
					return nil, e
				}
				if v.Cmp(lo[j]) < 0 || v.Cmp(hi[j]) > 0 {
					return nil, ErrDomain
				}
				out.triangles[owner][i][j] = new(big.Rat).Set(v)
				term := new(big.Rat).Mul(q.UV[owner][j], v)
				if e = budget.check(term); e != nil {
					return nil, e
				}
				value.Add(value, term)
				if e = budget.check(value); e != nil {
					return nil, e
				}
			}
			sign := value.Sign()
			if sign == 0 {
				return nil, ErrDomain
			}
			if out.sides[owner] == 0 {
				out.sides[owner] = sign
			} else if out.sides[owner] != sign {
				return nil, ErrDomain
			}
			margin := new(big.Rat).Abs(value)
			if out.margins[owner] == nil || margin.Cmp(out.margins[owner]) < 0 {
				out.margins[owner] = margin
			}
		}
		t := out.triangles[owner]
		var delta [2][2]*big.Rat
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				delta[i][j] = new(big.Rat).Sub(t[i+1][j], t[0][j])
				if e = budget.check(delta[i][j]); e != nil {
					return nil, e
				}
			}
		}
		x := new(big.Rat).Mul(delta[0][0], delta[1][1])
		y := new(big.Rat).Mul(delta[0][1], delta[1][0])
		for _, v := range []*big.Rat{x, y} {
			if e = budget.check(v); e != nil {
				return nil, e
			}
		}
		x.Sub(x, y)
		if e = budget.check(x); e != nil {
			return nil, e
		}
		if x.Sign() == 0 {
			return nil, ErrDomain
		}
	}
	if out.sides[0] != -out.sides[1] {
		return nil, ErrDomain
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.separator = out.Separator()
	out.operations = budget.operations
	return out, nil
}
