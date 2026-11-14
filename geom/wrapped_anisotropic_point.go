package geom

import (
	"context"
	"math/big"
)

// WrappedAnisotropicPoint classifies the capped original band. Parts identifies
// actual lateral/terminal support attainment; a surrounding loft must discard
// artificial interior band caps. No closest point or unique UV is asserted.
type WrappedAnisotropicPoint struct {
	Location        CylinderLocation
	Parts           CylinderBoundaryPart
	ExactOperations int
}

// LocateWrappedAnisotropicRational uses an exact independent diagonal inverse
// at the query height and the guarded original weighted-quadrant conic sign.
// It neither substitutes a circle nor evaluates a rounded point. Source proof
// and all query arithmetic share the supplied geometric work allowance.
func LocateWrappedAnisotropicRational(ctx context.Context, w *WrappedSurfacePatch, point [3]*big.Rat, o SurfacePatchOptions) (WrappedAnisotropicPoint, error) {
	empty := WrappedAnisotropicPoint{}
	options, e := validateRationalPoint(ctx, point, o)
	if e != nil {
		return empty, e
	}
	proof, e := CertifyWrappedAnisotropicEmbedding(ctx, w, options)
	if e != nil {
		return empty, e
	}
	b := &wrappedAnisotropicBudget{ctx: ctx, work: polynomialBudget{operations: proof.ExactOperations, maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}}
	for _, q := range point {
		b.keep(q)
	}
	if b.err != nil {
		return empty, b.err
	}
	result := WrappedAnisotropicPoint{Location: CylinderOutside}
	finish := func() (WrappedAnisotropicPoint, error) {
		if b.err != nil {
			return empty, b.err
		}
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		result.ExactOperations = b.work.operations
		return result, nil
	}
	z := point[2]
	if z.Cmp(proof.LowerZ) < 0 || z.Cmp(proof.UpperZ) > 0 {
		return finish()
	}
	t := b.div(b.sub(z, proof.LowerZ), b.sub(proof.UpperZ, proof.LowerZ))
	uv := [2]*big.Rat{}
	for axis := 0; axis < 2; axis++ {
		scale := b.add(proof.LowerScale[axis], b.mul(t, b.sub(proof.UpperScale[axis], proof.LowerScale[axis])))
		uv[axis] = b.div(b.sub(point[axis], proof.Center[axis]), scale)
	}
	if b.err != nil {
		return empty, b.err
	}
	u, v := uv[0], uv[1]
	quadrant := 0
	zero := new(big.Rat)
	if u.Sign() < 0 {
		if v.Sign() >= 0 {
			quadrant = 1
			u, v = v, b.sub(zero, u)
		} else {
			quadrant = 2
			u, v = b.sub(zero, u), b.sub(zero, v)
		}
	} else if v.Sign() < 0 {
		quadrant = 3
		u, v = b.sub(zero, v), u
	}
	one := big.NewRat(1, 1)
	if u.Cmp(one) > 0 || v.Cmp(one) > 0 {
		return finish()
	}
	sum := b.sub(b.add(u, v), one)
	sign := -1
	if sum.Sign() >= 0 {
		j := 4 * quadrant
		a, c, d := w.patch.net[j][3], w.patch.net[j+2][3], w.patch.net[j+4][3]
		left := b.mul(b.mul(a, d), b.mul(sum, sum))
		right := b.mul(b.mul(big.NewRat(4, 1), b.mul(c, c)), b.mul(b.sub(one, u), b.sub(one, v)))
		if b.err != nil {
			return empty, b.err
		}
		sign = left.Cmp(right)
	}
	if sign > 0 {
		return finish()
	}
	result.Location = CylinderInside
	if sign == 0 {
		result.Parts |= CylinderLateral
	}
	if z.Cmp(proof.LowerZ) == 0 {
		result.Parts |= CylinderLowerCap
	}
	if z.Cmp(proof.UpperZ) == 0 {
		result.Parts |= CylinderUpperCap
	}
	if result.Parts != 0 {
		result.Location = CylinderBoundary
	}
	return finish()
}
