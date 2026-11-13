package geom

import (
	"context"
	"math/big"
)

// DirectedPlanarConicContact preserves both original parameter identities.
// CrossingSign is sign(det(dA/dt,dB/ds)) in the requested ordered axes.
// Zero means regular tangency, not coincidence or singular velocity.
type DirectedPlanarConicContact struct {
	Pair         PairedComposedCurveEvent
	CrossingSign int
	Tangent      bool
}

// DirectedPlanarConicContacts is ordered by A's original parameter. CoincidentSupport
// is independent of finite arc overlap; use IntersectCoincidentComposedConics for it.
// On error no contacts are published (work and CoincidentSupport may be reported).
type DirectedPlanarConicContacts struct {
	Complete, CoincidentSupport bool
	Contacts                    []DirectedPlanarConicContact
	Operations                  int
}

// IntersectPlanarComposedConics proves a common constant omitted coordinate,
// pairs finite conic roots, and classifies directed tangent determinants exactly.
// Both homogeneous degrees must be <=2; B must be a nondegenerate conic.
// Singular projected velocities remain unresolved. Endpoint contacts retain their
// original endpoint masks; tangent labels describe continuation, not arc overlap.
// MaxOperations is cumulative; MaxSubdivisions bounds each root proof.
func IntersectPlanarComposedConics(ctx context.Context, a, b *ExactComposedCurveCell, axes [2]int, o ExactCurvePlaneEventOptions) (out DirectedPlanarConicContacts, err error) {
	if ctx == nil || a == nil || b == nil || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() { out.Operations = budget.operations }()
	omitted := 3 - axes[0] - axes[1]
	var height *big.Rat
	for _, c := range []*ExactComposedCurveCell{a, b} {
		if len(c.power[3]) == 0 {
			return out, ErrInvalid
		}
		for _, p := range c.power {
			if len(p) > 3 {
				return out, ErrConvergence
			}
			for _, v := range p {
				if e = budget.check(v); e != nil {
					return out, e
				}
			}
		}
		h := new(big.Rat)
		for i, w := range c.power[3] {
			if w.Sign() != 0 {
				if i < len(c.power[omitted]) {
					h.Quo(c.power[omitted][i], w)
				}
				break
			}
		}
		residual, e := subtractScaledPolynomial(c.power[omitted], c.power[3], h, budget)
		if e != nil {
			return out, e
		}
		if len(residual) != 0 {
			return out, ErrDomain
		}
		if height != nil && height.Cmp(h) != 0 {
			return out, ErrDomain
		}
		height = h
	}
	local := opts
	local.MaxOperations -= budget.operations
	if local.MaxOperations < 1 {
		return out, ErrConvergence
	}
	paired, e := IntersectComposedQuadraticConic(ctx, a, b, local)
	budget.operations += paired.Operations
	out.CoincidentSupport = paired.CoincidentSupport
	if e != nil {
		return out, e
	}
	if !paired.Complete {
		return out, ErrConvergence
	}
	if len(paired.Hits) == 0 {
		out.Complete = true
		return out, nil
	}
	lambda, e := quadraticConicInversePowers(a, b, budget)
	if e != nil {
		return out, e
	}
	var va, vb [2]rationalPolynomial
	for j, axis := range axes {
		for k, c := range []*ExactComposedCurveCell{a, b} {
			x, e := polynomialProduct(polynomialDerivative(c.power[axis]), c.power[3], budget)
			if e != nil {
				return out, e
			}
			y, e := polynomialProduct(c.power[axis], polynomialDerivative(c.power[3]), budget)
			if e != nil {
				return out, e
			}
			v, e := subtractScaledPolynomial(x, y, big.NewRat(1, 1), budget)
			if e != nil {
				return out, e
			}
			if k == 0 {
				va[j] = v
			} else {
				vb[j] = v
			}
		}
	}
	// Quadratic rational velocity numerators have degree at most two. Clear
	// the inverse parameter denominator to degree two, an even positive factor.
	var mapped [2]rationalPolynomial
	for j, p := range vb {
		if len(p) > 3 {
			return out, ErrConvergence
		}
		for k, q := range p {
			term := rationalPolynomial{big.NewRat(1, 1)}
			for n := 0; n < 2; n++ {
				factor := lambda[0]
				if n < k {
					factor = lambda[1]
				}
				term, e = polynomialProduct(term, factor, budget)
				if e != nil {
					return out, e
				}
			}
			mapped[j], e = subtractScaledPolynomial(mapped[j], term, new(big.Rat).Neg(q), budget)
			if e != nil {
				return out, e
			}
		}
	}
	x, e := polynomialProduct(va[0], mapped[1], budget)
	if e != nil {
		return out, e
	}
	y, e := polynomialProduct(va[1], mapped[0], budget)
	if e != nil {
		return out, e
	}
	determinant, e := subtractScaledPolynomial(x, y, big.NewRat(1, 1), budget)
	if e != nil {
		return out, e
	}
	contacts := make([]DirectedPlanarConicContact, 0, len(paired.Hits))
	for _, pair := range paired.Hits {
		for k, event := range []*ExactComposedCurveEvent{pair.A, pair.B} {
			velocity := va
			if k == 1 {
				velocity = vb
			}
			regular := false
			for _, p := range velocity {
				sign, _, e := composedPolynomialRootSign(ctx, event.root, p, budget, opts.MaxSubdivisions)
				if e != nil {
					return out, e
				}
				regular = regular || sign != 0
			}
			if !regular {
				return out, ErrConvergence
			}
		}
		den, _, e := composedPolynomialRootSign(ctx, pair.A.root, lambda[0], budget, opts.MaxSubdivisions)
		if e != nil {
			return out, e
		}
		if den == 0 {
			return out, ErrConvergence
		}
		sign, _, e := composedPolynomialRootSign(ctx, pair.A.root, determinant, budget, opts.MaxSubdivisions)
		if e != nil {
			return out, e
		}
		contacts = append(contacts, DirectedPlanarConicContact{Pair: pair, CrossingSign: sign, Tangent: sign == 0})
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Complete = true
	out.Contacts = contacts
	return out, nil
}
