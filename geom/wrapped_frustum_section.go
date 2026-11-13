package geom

import (
	"context"
	"math"
	"math/big"
)

// WrappedFrustumSection is the exact closed profile at a represented height.
// Curve keeps rational homogeneous controls and original circumferential knots;
// ParameterV and Radius are exact rationals, even when not binary representable.
// Cap is zero for an interior section, otherwise the coincident cap identity.
type WrappedFrustumSection struct {
	Curve                      *ExactSplineCurve
	ParameterV, Radius, Height *big.Rat
	Cap                        CylinderBoundaryPart
	ExactOperations            int
}

// SectionWrappedFrustum extracts the full horizontal section of a certified
// canonical positive-weight rational frustum/cylinder. Heights outside its
// closed range return ErrDomain. No conventional float pole reconstruction is
// used. The section descriptor can be bounded by BoundExactCurveLengthContext.
func SectionWrappedFrustum(ctx context.Context, w *WrappedSurfacePatch, height float64, o SurfacePatchOptions) (WrappedFrustumSection, error) {
	empty := WrappedFrustumSection{}
	if !finite(height) {
		return empty, ErrInvalid
	}
	proof, e := CertifyWrappedFrustumEmbedding(ctx, w, o)
	if e != nil {
		return empty, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return empty, e
	}
	z := new(big.Rat).SetFloat64(height)
	if z.Cmp(proof.LowerZ) < 0 || z.Cmp(proof.UpperZ) > 0 {
		return empty, ErrDomain
	}
	budget := &polynomialBudget{operations: proof.ExactOperations, maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	t := new(big.Rat).Quo(new(big.Rat).Sub(z, proof.LowerZ), new(big.Rat).Sub(proof.UpperZ, proof.LowerZ))
	one := new(big.Rat).Sub(big.NewRat(1, 1), t)
	source := w.patch
	net := make([]exactHomogeneous, 2*source.countU)
	for i := 0; i < source.countU; i++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		for k := 0; k < 4; k++ {
			v := new(big.Rat).Add(new(big.Rat).Mul(one, source.net[2*i][k]), new(big.Rat).Mul(t, source.net[2*i+1][k]))
			if e := budget.check(v); e != nil {
				return empty, e
			}
			net[2*i][k] = new(big.Rat).Set(v)
			net[2*i+1][k] = new(big.Rat).Set(v)
		}
	}
	p := &SurfacePatch{degreeU: source.degreeU, degreeV: 1, countU: source.countU, countV: 2, knotsU: append([]float64(nil), source.knotsU...), knotsV: []float64{0, 0, 1, 1}, net: net, options: options}
	if e := p.computeBounds(budget); e != nil {
		return empty, e
	}
	p.operations = budget.operations
	d := source.Domain()
	v := new(big.Rat).Add(new(big.Rat).SetFloat64(d.VMin), new(big.Rat).Mul(t, new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), new(big.Rat).SetFloat64(d.VMin))))
	radius := new(big.Rat).Add(proof.LowerRadius, new(big.Rat).Mul(t, new(big.Rat).Sub(proof.UpperRadius, proof.LowerRadius)))
	for _, q := range []*big.Rat{v, radius} {
		if e := budget.check(q); e != nil {
			return empty, e
		}
	}
	cap := CylinderBoundaryPart(0)
	if z.Cmp(proof.LowerZ) == 0 {
		cap = CylinderLowerCap
	}
	if z.Cmp(proof.UpperZ) == 0 {
		cap = CylinderUpperCap
	}
	return WrappedFrustumSection{&SurfaceIsoCurve{patch: p, fixedAxis: -1, fixedParameter: math.NaN(), operations: budget.operations}, v, radius, z, cap, budget.operations}, nil
}
