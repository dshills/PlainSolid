package geom

import (
	"context"
	"math/big"
)

// ComposedPlaneContact classifies the germ within one immutable composed cell.
// SideSigns are the signs immediately before/after its original parameter;
// zero denotes an unavailable side at an exact algebraic cell endpoint.
// Order is plane residual vanishing order, not curve velocity order. Singular
// means all velocity components vanish; such contacts are not called Tangent
// or Transverse. Crossing/Touching apply only to interior contacts.
type ComposedPlaneContact struct {
	Proven, Transverse, Tangent, Singular, Crossing, Touching bool
	Order                                                     int
	EndpointMask                                              uint8
	SideSigns                                                 [2]int
	Operations, Subdivisions                                  int
}

// ClassifyPlaneContact rechecks the supplied raw plane at this immutable event,
// including events from BindRoot. It supports irrational endpoint cuts and
// nonrepresentable composed coefficients without rebuilding a float spline.
// Coincident continuations return ErrConvergence, nonincident planes ErrDomain.
// Failure may report work counters but never a Proven or partial classification.
func (e *ExactComposedPlaneEvent) ClassifyPlaneContact(ctx context.Context, plane [4]*big.Rat, o ExactCurvePlaneEventOptions) (out ComposedPlaneContact, err error) {
	if ctx == nil || e == nil || e.cell == nil || e.root == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	o, err = graphRootOptions(o)
	if err != nil {
		return out, err
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.Operations = b.operations }()
	var p rationalPolynomial
	normal := false
	for axis, x := range plane {
		if x == nil {
			continue
		}
		if err = b.check(x); err != nil {
			return out, err
		}
		normal = normal || (axis < 3 && x.Sign() != 0)
		for len(p) < len(e.cell.power[axis]) {
			p = append(p, new(big.Rat))
		}
		for i, c := range e.cell.power[axis] {
			p[i].Add(p[i], new(big.Rat).Mul(x, c))
			if err = b.check(p[i]); err != nil {
				return out, err
			}
		}
	}
	if !normal {
		return out, ErrInvalid
	}
	p = trimPolynomial(p)
	if len(p) == 0 {
		return out, ErrConvergence
	}
	signAt := func(p rationalPolynomial) (int, error) {
		remaining := o.MaxSubdivisions - out.Subdivisions
		if remaining < 1 {
			return 0, ErrConvergence
		}
		sign, n, er := composedPolynomialRootSign(ctx, e.root, trimPolynomial(p), b, remaining)
		out.Subdivisions += n
		return sign, er
	}
	sign, er := signAt(p)
	if er != nil {
		return out, er
	}
	if sign != 0 {
		return out, ErrDomain
	}
	order := 0
	leading := 0
	for len(p) > 1 {
		order++
		p = polynomialDerivative(p)
		leading, err = signAt(p)
		if err != nil {
			return out, err
		}
		if leading != 0 {
			break
		}
	}
	if leading == 0 {
		return out, ErrConvergence
	}
	singular := true
	wd := polynomialDerivative(e.cell.power[3])
	for axis := 0; axis < 3; axis++ {
		a, er := polynomialProduct(polynomialDerivative(e.cell.power[axis]), e.cell.power[3], b)
		if er != nil {
			return out, er
		}
		c, er := polynomialProduct(e.cell.power[axis], wd, b)
		if er != nil {
			return out, er
		}
		n := make(rationalPolynomial, max(len(a), len(c)))
		for i := range n {
			n[i] = new(big.Rat)
			if i < len(a) {
				n[i].Set(a[i])
			}
			if i < len(c) {
				n[i].Sub(n[i], c[i])
			}
			if err = b.check(n[i]); err != nil {
				return out, err
			}
		}
		sign, er := signAt(n)
		if er != nil {
			return out, er
		}
		singular = singular && sign == 0
	}
	// EndpointMask comes from exact root membership, never a bracket endpoint.
	mask := e.endpointMask
	if err = ctx.Err(); err != nil {
		return out, err
	}
	out.Proven = true
	out.Order = order
	out.EndpointMask = mask
	out.Singular = singular
	out.Transverse = !singular && order == 1
	out.Tangent = !singular && order > 1
	out.Crossing = mask == 0 && order%2 == 1
	out.Touching = mask == 0 && order%2 == 0
	out.SideSigns = [2]int{leading, leading}
	if order%2 == 1 {
		out.SideSigns[0] = -leading
	}
	if mask&1 != 0 {
		out.SideSigns[0] = 0
	}
	if mask&2 != 0 {
		out.SideSigns[1] = 0
	}
	return out, nil
}
