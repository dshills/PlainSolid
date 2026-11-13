package geom

import (
	"context"
	"math/big"
)

// SpherePlaneSupport is an original sphere's section by a raw plane. Sign is
// the sign of RadiusSquared: -1 empty, 0 a point, +1 a physical circle.
// Center holds basis coordinates; WorldCenter is its orthogonal projection.
// SignedCenterResidual is normal dot (sphere center - plane origin).
type SpherePlaneSupport struct {
	Source                              BoreBoundaryMask
	Center                              [2]*big.Rat
	WorldCenter                         [3]*big.Rat
	RadiusSquared, SignedCenterResidual *big.Rat
	Sign                                int
}

// SpherePairPlaneSupports retains exact caller-owned rational chart data.
// World points are Origin + Basis[0]*u + Basis[1]*v. A support circle obeys
// (uv-Center)^T Gram (uv-Center) = RadiusSquared. Basis is not orthonormal;
// Basis[0] cross Basis[1] need not point along Normal. No material boundary
// selection, planar topology, or solid assembly is asserted by this report.
type SpherePairPlaneSupports struct {
	Origin, Normal [3]*big.Rat
	NormalSquared  *big.Rat
	Basis          [2][3]*big.Rat
	Gram           [2][2]*big.Rat
	Supports       [2]SpherePlaneSupport
}

// PlaneSupports intersects original sphere supports with the represented raw
// plane normal dot (p-origin)=0 without floating normalization. All arithmetic
// after input conversion is rational. Failure returns no partial report.
func (s SpherePairMaterial) PlaneSupports(ctx context.Context, origin, normal Vec3, o SurfacePatchEvaluationOptions) (SpherePairPlaneSupports, error) {
	var zero SpherePairPlaneSupports
	if ctx == nil || s.radius2[0] == nil || !origin.IsFinite() || !normal.IsFinite() {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return zero, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	var failure error
	check := func(x *big.Rat) *big.Rat {
		if failure == nil {
			failure = budget.check(x)
		}
		return x
	}
	add := func(a, b *big.Rat) *big.Rat { return check(new(big.Rat).Add(a, b)) }
	sub := func(a, b *big.Rat) *big.Rat { return check(new(big.Rat).Sub(a, b)) }
	mul := func(a, b *big.Rat) *big.Rat { return check(new(big.Rat).Mul(a, b)) }
	div := func(a, b *big.Rat) *big.Rat { return check(new(big.Rat).Quo(a, b)) }
	out := SpherePairPlaneSupports{NormalSquared: new(big.Rat)}
	ov, nv := [3]float64{origin.X, origin.Y, origin.Z}, [3]float64{normal.X, normal.Y, normal.Z}
	pivot := -1
	for j := 0; j < 3; j++ {
		out.Origin[j] = check(new(big.Rat).SetFloat64(ov[j]))
		out.Normal[j] = check(new(big.Rat).SetFloat64(nv[j]))
		out.NormalSquared = add(out.NormalSquared, mul(out.Normal[j], out.Normal[j]))
		if nv[j] != 0 {
			pivot = j
		}
	}
	if failure != nil {
		return zero, failure
	}
	if pivot < 0 {
		return zero, ErrDegenerate
	}
	var free [2]int
	k := 0
	for j := 0; j < 3; j++ {
		if j != pivot {
			free[k] = j
			k++
		}
	}
	for i := 0; i < 2; i++ {
		for j := 0; j < 3; j++ {
			out.Basis[i][j] = new(big.Rat)
		}
		out.Basis[i][free[i]].SetInt64(1)
		out.Basis[i][pivot] = div(new(big.Rat).Neg(out.Normal[free[i]]), out.Normal[pivot])
	}
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			out.Gram[i][j] = new(big.Rat)
			for k := 0; k < 3; k++ {
				out.Gram[i][j] = add(out.Gram[i][j], mul(out.Basis[i][k], out.Basis[j][k]))
			}
		}
	}
	for i := 0; i < 2; i++ {
		r := SpherePlaneSupport{Source: 1 << uint(i), SignedCenterResidual: new(big.Rat)}
		for j := 0; j < 3; j++ {
			check(s.center[i][j])
			r.SignedCenterResidual = add(r.SignedCenterResidual, mul(out.Normal[j], sub(s.center[i][j], out.Origin[j])))
		}
		check(s.radius2[i])
		q := div(r.SignedCenterResidual, out.NormalSquared)
		r.RadiusSquared = sub(s.radius2[i], mul(r.SignedCenterResidual, q))
		r.Sign = r.RadiusSquared.Sign()
		for j := 0; j < 3; j++ {
			r.WorldCenter[j] = sub(s.center[i][j], mul(q, out.Normal[j]))
		}
		for j := 0; j < 2; j++ {
			r.Center[j] = sub(r.WorldCenter[free[j]], out.Origin[free[j]])
		}
		out.Supports[i] = r
	}
	if failure != nil {
		return zero, failure
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
