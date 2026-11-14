package geom

import (
	"context"
	"math/big"
)

// SurfaceAffineTriangleMate proves two original triangular chart restrictions
// intersect exactly in the common image of their first side. It does not prove
// either individual restriction injective, manifold incidence, or a solid.
// Triangle vertices 0 and 1 specify corresponding full mate traversals; vertex
// 2 is strictly off the separating plane on opposite sides for the two owners.
type SurfaceAffineTriangleMate struct {
	source     [2]*SurfacePatch
	triangles  [2][3][2]*big.Rat
	sides      [2]int
	operations int
}

func (p *SurfaceAffineTriangleMate) Sides() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.sides
}
func (p *SurfaceAffineTriangleMate) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}
func (p *SurfaceAffineTriangleMate) Source(owner int) (*SurfacePatch, [3][2]*big.Rat, error) {
	var triangle [3][2]*big.Rat
	if p == nil || owner < 0 || owner > 1 {
		return nil, triangle, ErrInvalid
	}
	for i := range triangle {
		for j := range triangle[i] {
			triangle[i][j] = new(big.Rat).Set(p.triangles[owner][i][j])
		}
	}
	return p.source[owner], triangle, nil
}

// CertifySurfaceAffineTriangleMate verifies the full-chart affine covector
// identities, homogeneous linear trim signs, and continuous original model
// edge equality across independent knots. UV[owner] and offset[owner] describe
// normal dot model; level gives the common plane. One cumulative budget covers
// both identities, both original edge compositions and their comparison.
func CertifySurfaceAffineTriangleMate(ctx context.Context, a, b *SurfacePatch, triangles [2][3][2]*big.Rat, normal [3]*big.Rat, uv [2][2]*big.Rat, offset [2]*big.Rat, level *big.Rat, o SurfaceAffineCoordinateOptions) (*SurfaceAffineTriangleMate, error) {
	if ctx == nil || a == nil || b == nil || level == nil {
		return nil, ErrInvalid
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	if e = budget.check(level); e != nil {
		return nil, e
	}
	out := &SurfaceAffineTriangleMate{source: [2]*SurfacePatch{a, b}}
	var pieces [2]SurfaceAffineCurvePieces
	for owner, p := range out.source {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		if budget.operations >= opts.MaxOperations {
			return nil, ErrConvergence
		}
		proofOpts := o
		proofOpts.SurfacePatchOptions = opts
		proofOpts.MaxOperations -= budget.operations
		proof, e := CertifySurfaceAffineForm(ctx, p, normal, uv[owner], offset[owner], proofOpts)
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
			residual := new(big.Rat).Sub(offset[owner], level)
			if e = budget.check(residual); e != nil {
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
				term := new(big.Rat).Mul(uv[owner][j], v)
				if e = budget.check(term); e != nil {
					return nil, e
				}
				residual.Add(residual, term)
				if e = budget.check(residual); e != nil {
					return nil, e
				}
			}
			if i < 2 && residual.Sign() != 0 {
				return nil, ErrDomain
			}
			if i == 2 {
				out.sides[owner] = residual.Sign()
				if residual.Sign() == 0 {
					return nil, ErrDomain
				}
			}
		}
		t := out.triangles[owner]
		if t[0][0].Cmp(t[1][0]) == 0 && t[0][1].Cmp(t[1][1]) == 0 {
			return nil, ErrDomain
		}
		// Two distinct points on the nonconstant affine zero line and one strictly
		// off it imply a nondegenerate triangle; no sampled winding test is needed.
		if budget.operations >= opts.MaxOperations {
			return nil, ErrConvergence
		}
		pieces[owner], e = p.AffineParameterCurvePieces(ctx, t[0], t[1], SurfaceAffineCurveOptions{MaxPieces: o.MaxCells, TubeAffineLineOptions: TubeAffineLineOptions{MaxPoles: opts.MaxPoles, MaxOperations: opts.MaxOperations - budget.operations, MaxCoefficientBits: opts.MaxCoefficientBits}})
		if e != nil {
			return nil, e
		}
		budget.operations += pieces[owner].ExactOperations
		if !pieces[owner].Complete || !pieces[owner].Continuous {
			return nil, ErrConvergence
		}
	}
	if out.sides[0] != -out.sides[1] {
		return nil, ErrDomain
	}
	if budget.operations >= opts.MaxOperations {
		return nil, ErrConvergence
	}
	compared, e := CompareAffineParameterCurvePieces(ctx, pieces[0], pieces[1], ExactCurveComparisonOptions{MaxOperations: opts.MaxOperations - budget.operations, MaxCoefficientBits: opts.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	budget.operations += compared.ExactOperations
	if !compared.Complete || !compared.Equal {
		return nil, ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.operations = budget.operations
	return out, nil
}
