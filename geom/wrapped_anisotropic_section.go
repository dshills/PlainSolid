package geom

import (
	"context"
	"math"
	"math/big"
)

// WrappedAnisotropicSection retains the original U knots and exact homogeneous
// controls at a rational model height. Scale is the two independent control
// scales, not a claim of circular geometry. Returned rationals are owned.
type WrappedAnisotropicSection struct {
	Curve              *ExactSplineCurve
	ParameterV, Height *big.Rat
	Center, Scale      [2]*big.Rat
	Cap                CylinderBoundaryPart
	ExactOperations    int
}

func SectionWrappedAnisotropicRational(ctx context.Context, w *WrappedSurfacePatch, height *big.Rat, o SurfacePatchOptions) (WrappedAnisotropicSection, error) {
	var empty WrappedAnisotropicSection
	if ctx == nil || height == nil {
		return empty, ErrInvalid
	}
	proof, e := CertifyWrappedAnisotropicEmbedding(ctx, w, o)
	if e != nil {
		return empty, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return empty, e
	}
	b := &wrappedAnisotropicBudget{ctx: ctx, work: polynomialBudget{operations: proof.ExactOperations, maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}}
	z := b.keep(new(big.Rat).Set(height))
	if b.err != nil {
		return empty, b.err
	}
	if z.Cmp(proof.LowerZ) < 0 || z.Cmp(proof.UpperZ) > 0 {
		return empty, ErrDomain
	}
	t := b.div(b.sub(z, proof.LowerZ), b.sub(proof.UpperZ, proof.LowerZ))
	one := b.sub(big.NewRat(1, 1), t)
	source := w.patch
	net := make([]exactHomogeneous, len(source.net))
	for i := 0; i < source.countU; i++ {
		for k := 0; k < 4; k++ {
			value := b.add(b.mul(one, source.net[2*i][k]), b.mul(t, source.net[2*i+1][k]))
			if b.err != nil {
				return empty, b.err
			}
			net[2*i][k] = new(big.Rat).Set(value)
			net[2*i+1][k] = new(big.Rat).Set(value)
		}
	}
	p := &SurfacePatch{degreeU: source.degreeU, degreeV: 1, countU: source.countU, countV: 2, knotsU: append([]float64(nil), source.knotsU...), knotsV: []float64{0, 0, 1, 1}, net: net, options: options}
	if e = p.computeBounds(&b.work); e != nil {
		return empty, e
	}
	d := source.Domain()
	v := b.add(new(big.Rat).SetFloat64(d.VMin), b.mul(t, b.sub(new(big.Rat).SetFloat64(d.VMax), new(big.Rat).SetFloat64(d.VMin))))
	scale := [2]*big.Rat{}
	for axis := 0; axis < 2; axis++ {
		scale[axis] = b.add(proof.LowerScale[axis], b.mul(t, b.sub(proof.UpperScale[axis], proof.LowerScale[axis])))
	}
	if b.err != nil {
		return empty, b.err
	}
	p.operations = b.work.operations
	cap := CylinderBoundaryPart(0)
	if z.Cmp(proof.LowerZ) == 0 {
		cap = CylinderLowerCap
	}
	if z.Cmp(proof.UpperZ) == 0 {
		cap = CylinderUpperCap
	}
	return WrappedAnisotropicSection{Curve: &SurfaceIsoCurve{patch: p, fixedAxis: -1, fixedParameter: math.NaN(), operations: b.work.operations}, ParameterV: v, Height: z, Center: proof.Center, Scale: scale, Cap: cap, ExactOperations: b.work.operations}, nil
}
