package geom

import (
	"context"
	"math/big"
)

// ToroidMeridianLocation is an exact material classification. Unknown is only
// returned with an error; Boundary includes the two periodic meridian seams.
type ToroidMeridianLocation uint8

const (
	ToroidMeridianUnknown ToroidMeridianLocation = iota
	ToroidMeridianOutside
	ToroidMeridianInside
	ToroidMeridianBoundary
)

// LocateWrappedToroidMeridian classifies a represented point on either exact
// axial meridian plane x=Center.X or y=Center.Y. It first proves the canonical
// wrapped toroid embedding. Points off both planes return ErrDomain; this is
// not a general 3D locator. No tolerance or rounded circle equation is used.
// The rational quadrant conic is evaluated directly with exact homogeneous
// weights, allowing asymmetric positive quadrant weights and elliptical axial
// scaling. All arithmetic consumes the supplied operation/coefficient budget.
func LocateWrappedToroidMeridian(ctx context.Context, w *WrappedSurfacePatch, point Vec3, o SurfacePatchOptions) (ToroidMeridianLocation, error) {
	unknown := ToroidMeridianUnknown
	if !point.IsFinite() {
		return unknown, ErrInvalid
	}
	proof, e := CertifyWrappedToroidEmbedding(ctx, w, o)
	if e != nil {
		return unknown, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return unknown, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: proof.ExactOperations}
	x := new(big.Rat).Sub(new(big.Rat).SetFloat64(point.X), proof.Center[0])
	y := new(big.Rat).Sub(new(big.Rat).SetFloat64(point.Y), proof.Center[1])
	z := new(big.Rat).Sub(new(big.Rat).SetFloat64(point.Z), proof.Center[2])
	if x.Sign() != 0 && y.Sign() != 0 {
		return unknown, ErrDomain
	}
	radius := new(big.Rat).Abs(x)
	if x.Sign() == 0 {
		radius.Abs(y)
	}
	u := new(big.Rat).Quo(new(big.Rat).Sub(radius, proof.MajorRadius), proof.RadialMinorRadius)
	v := new(big.Rat).Quo(z, proof.AxialMinorRadius)
	for _, r := range []*big.Rat{x, y, z, radius, u, v} {
		if e := budget.check(r); e != nil {
			return unknown, e
		}
	}
	// Rotate the selected quadrant to the canonical control triangle
	// (1,0),(1,1),(0,1), preserving the original weight order.
	quadrant := 0
	if u.Sign() < 0 {
		if v.Sign() >= 0 {
			quadrant = 1
			u, v = new(big.Rat).Set(v), new(big.Rat).Neg(u)
		} else {
			quadrant = 2
			u.Neg(u)
			v.Neg(v)
		}
	} else if v.Sign() < 0 {
		quadrant = 3
		u, v = new(big.Rat).Neg(v), new(big.Rat).Set(u)
	}
	one := big.NewRat(1, 1)
	if u.Cmp(one) > 0 || v.Cmp(one) > 0 {
		return ToroidMeridianOutside, nil
	}
	sum := new(big.Rat).Sub(new(big.Rat).Add(u, v), one)
	if sum.Sign() < 0 {
		return ToroidMeridianInside, nil
	}
	j := 2 * quadrant
	w0, w1, w2 := w.patch.net[j][3], w.patch.net[j+1][3], w.patch.net[j+2][3]
	left := new(big.Rat).Mul(w0, w2)
	left.Mul(left, new(big.Rat).Mul(sum, sum))
	right := new(big.Rat).Mul(w1, w1)
	right.Mul(right, big.NewRat(4, 1))
	right.Mul(right, new(big.Rat).Sub(one, u))
	right.Mul(right, new(big.Rat).Sub(one, v))
	for _, r := range []*big.Rat{sum, left, right} {
		if e := budget.check(r); e != nil {
			return unknown, e
		}
	}
	if e := ctx.Err(); e != nil {
		return unknown, e
	}
	switch left.Cmp(right) {
	case -1:
		return ToroidMeridianInside, nil
	case 0:
		return ToroidMeridianBoundary, nil
	default:
		return ToroidMeridianOutside, nil
	}
}

// LocateWrappedCircularToroid classifies arbitrary represented 3D points when
// both exact rational profiles are proven Euclidean circles. Axial scaling may
// still make the meridian ellipse noncircular in model space. Every quadrant
// must satisfy w0*w2=2*w1²; other rational toroids return ErrConvergence.
// The radial square root is eliminated by exact signs before squaring, avoiding
// extraneous roots. This is independent of numerical nearest-point queries.
func LocateWrappedCircularToroid(ctx context.Context, w *WrappedSurfacePatch, point Vec3, o SurfacePatchOptions) (ToroidMeridianLocation, error) {
	if !point.IsFinite() {
		return ToroidMeridianUnknown, ErrInvalid
	}
	proof, e := CertifyWrappedToroidEmbedding(ctx, w, o)
	if e != nil {
		return ToroidMeridianUnknown, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return ToroidMeridianUnknown, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: proof.ExactOperations}
	for _, stride := range []int{1, 9} {
		for j := 0; j < 8; j += 2 {
			a, b, c := w.patch.net[j*stride][3], w.patch.net[(j+1)*stride][3], w.patch.net[(j+2)*stride][3]
			left, right := new(big.Rat).Mul(a, c), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(b, b))
			for _, v := range []*big.Rat{left, right} {
				if e := budget.check(v); e != nil {
					return ToroidMeridianUnknown, e
				}
			}
			if left.Cmp(right) != 0 {
				return ToroidMeridianUnknown, ErrConvergence
			}
		}
	}
	x := new(big.Rat).Sub(new(big.Rat).SetFloat64(point.X), proof.Center[0])
	y := new(big.Rat).Sub(new(big.Rat).SetFloat64(point.Y), proof.Center[1])
	z := new(big.Rat).Sub(new(big.Rat).SetFloat64(point.Z), proof.Center[2])
	q := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	major2 := new(big.Rat).Mul(proof.MajorRadius, proof.MajorRadius)
	minor2 := new(big.Rat).Mul(proof.RadialMinorRadius, proof.RadialMinorRadius)
	axial2 := new(big.Rat).Mul(proof.AxialMinorRadius, proof.AxialMinorRadius)
	vertical := new(big.Rat).Quo(new(big.Rat).Mul(z, z), axial2)
	a := new(big.Rat).Add(q, major2)
	a.Add(a, new(big.Rat).Mul(minor2, new(big.Rat).Sub(vertical, big.NewRat(1, 1))))
	left := new(big.Rat).Mul(a, a)
	right := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(major2, q))
	for _, v := range []*big.Rat{x, y, z, q, major2, minor2, axial2, vertical, a, left, right} {
		if e := budget.check(v); e != nil {
			return ToroidMeridianUnknown, e
		}
	}
	if e := ctx.Err(); e != nil {
		return ToroidMeridianUnknown, e
	}
	if a.Sign() < 0 {
		return ToroidMeridianInside, nil
	}
	switch left.Cmp(right) {
	case -1:
		return ToroidMeridianInside, nil
	case 0:
		return ToroidMeridianBoundary, nil
	default:
		return ToroidMeridianOutside, nil
	}
}
