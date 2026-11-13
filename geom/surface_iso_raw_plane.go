package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactCurvePlaneHit retains a rational root bracket in original parameters.
// Parameter and Point are nonnil only when a rational root is recovered exactly.
// Multiplicity is local to Span; knot contacts occur once per incident span.
type ExactCurvePlaneHit struct {
	Lower, Upper      *big.Rat
	Parameter         *big.Rat
	Point             [3]*big.Rat
	Multiplicity      int
	Tangent, Crossing bool
	Span              ParameterSpan
}

// ExactCurvePlaneIntersection is exhaustive only when Complete is true. Roots
// without rational recovery still have certified brackets and multiplicities.
type ExactCurvePlaneIntersection struct {
	Hits                     []ExactCurvePlaneHit
	Coincident               []ParameterSpan
	Complete                 bool
	Operations, Subdivisions int
}

// IntersectRawPlane intersects the exact retained curve with the determinant
// plane ((b-a) cross (d-a)) dot (P-a)=0. No rounded unit normal is formed.
// ParameterTolerance bounds brackets; DistanceTolerance is unused because no
// rounded model residual participates. Other limits follow SplinePlaneOptions.
// Linear roots and quadratic roots with rational square discriminants are
// recovered exactly, as are rational singleton roots of any degree. Higher
// degree roots may be recovered by bounded continued-fraction reconstruction
// and exact substitution; failure to recover one retains its certified bracket.
func (c *SurfaceIsoCurve) IntersectRawPlane(ctx context.Context, a, b, d Vec3, o SplinePlaneOptions) (r ExactCurvePlaneIntersection, err error) {
	if ctx == nil || c == nil || c.patch == nil || !a.IsFinite() || !b.IsFinite() || !d.IsFinite() {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 12
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxDegree < 1 || o.MaxDegree > 64 || o.MaxControlPoints < 2 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	if c.patch.degreeU > o.MaxDegree || c.patch.countU > o.MaxControlPoints {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		r.Operations = budget.operations
		sort.SliceStable(r.Hits, func(i, j int) bool {
			cmp := r.Hits[i].Lower.Cmp(r.Hits[j].Lower)
			if cmp == 0 {
				return r.Hits[i].Span.Index < r.Hits[j].Span.Index
			}
			return cmp < 0
		})
	}()
	var origin, x, y, normal [3]*big.Rat
	for i := 0; i < 3; i++ {
		origin[i] = new(big.Rat).SetFloat64(coordinate(a, i))
		x[i] = new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(b, i)), origin[i])
		y[i] = new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(d, i)), origin[i])
	}
	nonzero := false
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		normal[i] = new(big.Rat).Sub(new(big.Rat).Mul(x[j], y[k]), new(big.Rat).Mul(x[k], y[j]))
		if e := budget.check(normal[i]); e != nil {
			return r, e
		}
		nonzero = nonzero || normal[i].Sign() != 0
	}
	if !nonzero {
		return r, ErrDegenerate
	}
	stack := []*SurfacePatch{c.patch}
	index := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if knot, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
			left, right, e := p.split(knot, true, budget)
			if e != nil {
				return r, e
			}
			stack = append(stack, right, left)
			continue
		}
		domain := p.Domain()
		span := ParameterSpan{Start: domain.UMin, End: domain.UMax, Index: index}
		index++
		coefficients := make([]*big.Rat, p.countU)
		for i := range coefficients {
			h := p.net[i*p.countV]
			v := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				delta := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(origin[axis], h[3]))
				v.Add(v, new(big.Rat).Mul(normal[axis], delta))
				if e := budget.check(v); e != nil {
					return r, e
				}
			}
			coefficients[i] = v
		}
		polynomial, e := bernsteinToPower(coefficients, budget)
		if e != nil {
			return r, e
		}
		if len(polynomial) == 0 {
			r.Coincident = append(r.Coincident, span)
			continue
		}
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(polynomial, new(big.Rat), big.NewRat(1, 1), budget, remaining, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			parameter, e := recoverRationalPolynomialRoot(polynomial, lo, hi, budget)
			if e != nil {
				return false, e
			}
			lower, upper := parameterMap(span, lo), parameterMap(span, hi)
			if parameter == nil && new(big.Rat).Sub(upper, lower).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			hit := ExactCurvePlaneHit{Lower: lower, Upper: upper, Multiplicity: m, Tangent: m > 1, Crossing: m%2 == 1, Span: span}
			if parameter != nil {
				hit.Parameter = parameterMap(span, parameter)
				hit.Lower = new(big.Rat).Set(hit.Parameter)
				hit.Upper = new(big.Rat).Set(hit.Parameter)
				point, _, e := exactSurfaceJet(p, hit.Parameter, new(big.Rat), budget)
				if e != nil {
					return false, e
				}
				for axis := 0; axis < 3; axis++ {
					hit.Point[axis] = new(big.Rat).Set(point[axis])
				}
			}
			r.Hits = append(r.Hits, hit)
			return true, nil
		})
		r.Subdivisions += steps
		if e != nil {
			return r, e
		}
	}
	r.Complete = true
	return r, nil
}

// recoverRationalPolynomialRoot never guesses: every returned candidate is
// checked against the exact polynomial and the supplied isolating interval.
func recoverRationalPolynomialRoot(p rationalPolynomial, lo, hi *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	candidates := []*big.Rat{}
	if lo.Cmp(hi) == 0 {
		candidates = append(candidates, new(big.Rat).Set(lo))
	} else if len(p) == 2 {
		candidates = append(candidates, new(big.Rat).Quo(new(big.Rat).Neg(p[0]), p[1]))
	} else if len(p) == 3 {
		disc := new(big.Rat).Sub(new(big.Rat).Mul(p[1], p[1]), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(p[0], p[2])))
		if e := b.check(disc); e != nil {
			return nil, e
		}
		if disc.Sign() >= 0 {
			n, d := new(big.Int).Sqrt(disc.Num()), new(big.Int).Sqrt(disc.Denom())
			if new(big.Int).Mul(n, n).Cmp(disc.Num()) == 0 && new(big.Int).Mul(d, d).Cmp(disc.Denom()) == 0 {
				root := new(big.Rat).SetFrac(n, d)
				den := new(big.Rat).Mul(big.NewRat(2, 1), p[2])
				for _, sign := range []int64{-1, 1} {
					candidates = append(candidates, new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Neg(p[1]), new(big.Rat).Mul(big.NewRat(sign, 1), root)), den))
				}
			}
		}
	}
	if len(p) > 3 && lo.Cmp(hi) < 0 {
		reconstructed, e := intervalRationalConvergents(lo, hi, b)
		if e != nil {
			return nil, e
		}
		candidates = append(candidates, reconstructed...)
	}
	var found *big.Rat
	for _, x := range candidates {
		if e := b.check(x); e != nil {
			return nil, e
		}
		if lo.Cmp(hi) == 0 {
			if x.Cmp(lo) != 0 {
				continue
			}
		} else if x.Cmp(lo) <= 0 || x.Cmp(hi) >= 0 {
			// Non-singleton Sturm intervals contain the active root strictly
			// inside. Boundary roots may have been removed earlier.
			continue
		}
		value, e := polynomialValue(p, x, b)
		if e != nil {
			return nil, e
		}
		if value.Sign() == 0 {
			if found != nil && found.Cmp(x) != 0 {
				return nil, nil
			}
			found = x
		}
	}
	return found, nil
}
