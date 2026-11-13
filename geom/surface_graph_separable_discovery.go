package geom

import (
	"context"
	"errors"
	"math/big"
)

// SeparableGraphDiscoveryOptions adds a bounded Cartesian candidate search to
// the exact separable certificate. MaxCandidates defaults to128. Root and
// arithmetic budgets are cumulative across discovery and proof attempts.
type SeparableGraphDiscoveryOptions struct {
	SeparableGraphContactOptions
	MaxCandidates int
}

// SeparableGraphDiscovery retains any stationary roots whose rational values
// could not be recovered. Their brackets use original UV units. Such candidates
// are never rounded into proposed contact points. Discovery failure is not a
// noncontact certificate, even if all finite proposals have been tested.
type SeparableGraphDiscovery struct {
	Contact                                                            SeparableSplineGraphContact
	Unrecovered                                                        [2][]ExactParameterInterval
	CandidatesTested, ProofAttempts, ExactOperations, RootSubdivisions int
}

// ExactParameterInterval contains caller-owned exact rational bounds.
type ExactParameterInterval struct{ Lower, Upper *big.Rat }

// DiscoverSeparableSplineGraphPointContact isolates stationary roots of both
// exact piecewise univariate slices, removes repeated factors before rational recovery,
// and tries recovered roots and all knot endpoints in bounded pairs. Multi-span
// sources require exact numerator C1 compatibility and cellwise separability. Unrecovered
// roots are refined to normalized interval width at most 1e-10 when resources
// permit. Returned brackets remain exact; that width never decides incidence.
func DiscoverSeparableSplineGraphPointContact(ctx context.Context, a, c *SurfacePatch, o SeparableGraphDiscoveryOptions) (r SeparableGraphDiscovery, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxCandidates < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxRootSubdivisions < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	p, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	if _, _, ok := patchSplitChoice(p); ok {
		return discoverMultispanSeparableContact(ctx, a, c, p, o, b)
	}
	// Prove separability and obtain the polynomial restrictions at v=0/u=0.
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		values := make([]*big.Rat, p.countV)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		rows[i], e = bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
	}
	var up rationalPolynomial
	for j := 0; j < p.countV; j++ {
		values := make([]*big.Rat, p.countU)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		column, e := bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
		if j == 0 {
			up = column
		} else {
			for i := 1; i < len(column); i++ {
				if column[i].Sign() != 0 {
					return r, ErrConvergence
				}
			}
		}
	}
	polynomials := [2]rationalPolynomial{up, rows[0]}
	d := p.Domain()
	spans := [2]ParameterSpan{{Start: d.UMin, End: d.UMax}, {Start: d.VMin, End: d.VMax}}
	var candidates [2][]*big.Rat
	for axis, poly := range polynomials {
		derivative := polynomialDerivative(poly)
		if len(derivative) == 0 {
			return r, ErrConvergence
		}
		gcd, e := polynomialGCD(derivative, polynomialDerivative(derivative), b)
		if e != nil {
			return r, e
		}
		squarefree, rest, e := polynomialDivide(derivative, gcd, b)
		if e != nil {
			return r, e
		}
		if len(rest) != 0 {
			return r, ErrConvergence
		}
		seen := map[string]bool{}
		add := func(x *big.Rat) {
			key := x.RatString()
			if !seen[key] {
				seen[key] = true
				candidates[axis] = append(candidates[axis], x)
			}
		}
		remaining := o.MaxRootSubdivisions - r.RootSubdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		n, e := isolatePolynomialRoots(derivative, new(big.Rat), big.NewRat(1, 1), b, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			root, e := recoverRationalPolynomialRoot(squarefree, lo, hi, b)
			if e != nil {
				return false, e
			}
			if root != nil {
				add(parameterMap(spans[axis], root))
			} else {
				if new(big.Rat).Sub(hi, lo).Cmp(big.NewRat(1, 10000000000)) > 0 {
					return false, nil
				}
				r.Unrecovered[axis] = append(r.Unrecovered[axis], ExactParameterInterval{parameterMap(spans[axis], lo), parameterMap(spans[axis], hi)})
			}
			return true, nil
		})
		r.RootSubdivisions += n
		if e != nil {
			return r, e
		}
		add(new(big.Rat).SetFloat64(spans[axis].Start))
		add(new(big.Rat).SetFloat64(spans[axis].End))
	}
	for _, u := range candidates[0] {
		for _, v := range candidates[1] {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			if r.CandidatesTested >= o.MaxCandidates {
				return r, ErrConvergence
			}
			r.CandidatesTested++
			remaining := o.MaxOperations - b.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			uv := [2]*big.Rat{u, v}
			jet, e := p.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			b.operations += jet.ExactOperations
			if e != nil {
				return r, e
			}
			if jet.Point[2].Sign() != 0 {
				continue
			}
			remaining = o.MaxOperations - b.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			local := o.SeparableGraphContactOptions
			local.MaxOperations = remaining
			local.MaxRootSubdivisions = o.MaxRootSubdivisions - r.RootSubdivisions
			if local.MaxRootSubdivisions < 1 {
				return r, ErrConvergence
			}
			r.ProofAttempts++
			proof, e := CertifySeparableSplineGraphPointContact(ctx, a, c, uv, local)
			b.operations += proof.ExactOperations
			r.RootSubdivisions += proof.RootSubdivisions
			r.Contact = proof
			if e == nil && proof.Proven {
				return r, nil
			}
			if e != nil && !errors.Is(e, ErrConvergence) {
				return r, e
			}
		}
	}
	return r, ErrConvergence
}
