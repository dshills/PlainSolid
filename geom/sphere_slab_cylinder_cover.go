package geom

import (
	"context"
	"math/big"
)

type SphereSlabCylinderCoverage uint8

const (
	SphereSlabNoMaterial SphereSlabCylinderCoverage = iota
	SphereSlabStrictlyCovered
	SphereSlabCoveredTouching
	SphereSlabNotCovered
)

// SphereSlabCylinderCover applies to the sphere material in a finite raw-axis
// slab. NearestParameter is the closest slab parameter to the sphere center,
// with physical signed height NearestHeight (along the signed axis unit).
// MaximumSectionRadiusSquared is exact even when its square root is irrational.
// Cylinder axial caps are exactly Window; the cylinder is closed material.
// StrictlyCovered/Touching refer to radial wall clearance; coincident slab
// cap planes are allowed in both cases.
type SphereSlabCylinderCover struct {
	Complete                                                                                  bool
	Coverage                                                                                  SphereSlabCylinderCoverage
	NearestParameter, NearestHeight, MaximumSectionRadiusSquared, CenterRadialDistanceSquared *big.Rat
	ExactOperations                                                                           int
}

// ClassifySphereSlabCylinderCover decides whether a coordinate-axis cylinder
// covers the ENTIRE sphere material in its finite axial slab. The largest
// sphere cross-section lies at the height closest to the center; all sections
// have the same radial center. Exact squared comparisons prove
// sqrt(centerDistance²)+sqrt(maxSectionRadius²) <= cylinderRadius without
// rounding either radical. Equality is retained distinctly from strict cover.
// NoMaterial is vacuous coverage, not a nonempty solid certificate.
func ClassifySphereSlabCylinderCover(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, cylinderRadius float64, window [2]*big.Rat, o SurfacePatchEvaluationOptions) (out SphereSlabCylinderCover, err error) {
	if ctx == nil || !center.IsFinite() || !origin.IsFinite() || !axis.IsFinite() || !finite(sphereRadius) || !finite(cylinderRadius) || sphereRadius <= 0 || cylinderRadius <= 0 || window[0] == nil || window[1] == nil || window[0].Cmp(window[1]) >= 0 {
		return out, ErrInvalid
	}
	ax := -1
	for i := 0; i < 3; i++ {
		if coordinate(axis, i) != 0 {
			if ax >= 0 {
				return out, ErrDomain
			}
			ax = i
		}
	}
	if ax < 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for _, q := range window {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	scale := r(coordinate(axis, ax))
	t0 := new(big.Rat).Quo(new(big.Rat).Sub(r(coordinate(center, ax)), r(coordinate(origin, ax))), scale)
	nearest := new(big.Rat).Set(t0)
	if nearest.Cmp(window[0]) < 0 {
		nearest.Set(window[0])
	}
	if nearest.Cmp(window[1]) > 0 {
		nearest.Set(window[1])
	}
	height := new(big.Rat).Mul(new(big.Rat).Abs(scale), new(big.Rat).Sub(nearest, t0))
	S := new(big.Rat).Sub(new(big.Rat).Mul(r(sphereRadius), r(sphereRadius)), new(big.Rat).Mul(height, height))
	D := new(big.Rat)
	for i := 0; i < 3; i++ {
		if i != ax {
			delta := new(big.Rat).Sub(r(coordinate(center, i)), r(coordinate(origin, i)))
			D.Add(D, new(big.Rat).Mul(delta, delta))
		}
	}
	out.NearestParameter = nearest
	out.NearestHeight = height
	out.MaximumSectionRadiusSquared = S
	out.CenterRadialDistanceSquared = D
	for _, q := range []*big.Rat{t0, nearest, height, S, D} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	if S.Sign() < 0 {
		out.Coverage = SphereSlabNoMaterial
		out.Complete = true
		return out, nil
	}
	// r²-D-S compared against 2sqrt(D*S); squaring is only valid after
	// checking its nonnegative sign. S=0 includes a sphere-pole point.
	gap := new(big.Rat).Sub(new(big.Rat).Sub(new(big.Rat).Mul(r(cylinderRadius), r(cylinderRadius)), D), S)
	disc := new(big.Rat).Sub(new(big.Rat).Mul(gap, gap), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(D, S)))
	for _, q := range []*big.Rat{gap, disc} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.Coverage = SphereSlabNotCovered
	if gap.Sign() >= 0 && disc.Sign() >= 0 {
		out.Coverage = SphereSlabStrictlyCovered
		if disc.Sign() == 0 {
			out.Coverage = SphereSlabCoveredTouching
		}
	}
	out.Complete = true
	return out, nil
}
