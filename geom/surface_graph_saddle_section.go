package geom

import (
	"context"
	"math/big"
)

// GraphSaddleTerminal is the endpoint of one ray on the clipping rectangle.
// SideMask bits 0/1/2/3 denote UMin/UMax/VMin/VMax. Both coordinates are
// singleton iff Exact; otherwise one boundary coordinate is still exact.
// The unique factor root in SlopeLower/Upper identifies the algebraic endpoint.
type GraphSaddleTerminal struct {
	Ray                    int
	SideMask               uint8
	Lower, Upper           [2]*big.Rat
	Exact                  bool
	Point                  [3]*big.Rat
	SlopeLower, SlopeUpper *big.Rat
}

// QuadraticGraphSection is a complete star graph: one shared exact saddle node
// Certificate.Critical and four edges indexed like Certificate.Rays. Each edge
// connects that node to Terminals[Ray]. No other intersections occur inside
// Rectangle, by the exact quadratic identity and certified projected injectivity.
type QuadraticGraphSection struct {
	Complete                      bool
	Certificate                   QuadraticGraphSaddleBranches
	Rectangle                     [2][2]*big.Rat
	Terminals                     []GraphSaddleTerminal
	ExactOperations, Subdivisions int
}

// ClipQuadraticGraphSaddleSection clips the two exact factor lines to a closed
// rational rectangle strictly containing the saddle and contained in its proved
// cell. Algebraic coordinates remain exact rational enclosures, never rounded
// topological vertices. Finite root refinement is bounded by exact work and 8192
// subdivisions per terminal; coefficient-bit limits also remain enforced.
func ClipQuadraticGraphSaddleSection(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, rectangle [2][2]*big.Rat, o GraphPointContactOptions) (r QuadraticGraphSection, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	certificate, e := CertifyQuadraticGraphSaddleBranches(ctx, a, c, uv, o)
	r.Certificate = certificate
	r.ExactOperations = certificate.ExactOperations
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations - certificate.ExactOperations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations < 1 {
		return r, ErrConvergence
	}
	defer func() { r.ExactOperations = certificate.ExactOperations + budget.operations }()
	d := certificate.Critical.Cell
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for axis, span := range rectangle {
		if span[0] == nil || span[1] == nil || span[0].Cmp(uv[axis]) >= 0 || span[1].Cmp(uv[axis]) <= 0 {
			return r, ErrDomain
		}
		for j, x := range span {
			if e := budget.check(x); e != nil {
				return r, e
			}
			r.Rectangle[axis][j] = new(big.Rat).Set(x)
		}
		if span[0].Cmp(new(big.Rat).SetFloat64(limits[axis][0])) < 0 || span[1].Cmp(new(big.Rat).SetFloat64(limits[axis][1])) > 0 {
			return r, ErrDomain
		}
	}
	h := certificate.Critical.Hessian
	poly := trimPolynomial(rationalPolynomial{h[0][0], new(big.Rat).Mul(big.NewRat(2, 1), h[0][1]), h[1][1]})
	for index, ray := range certificate.Rays {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		lo, hi := new(big.Rat).Set(ray.SlopeLower), new(big.Rat).Set(ray.SlopeUpper)
		if ray.Axis == 0 {
			candidate, e := recoverRationalPolynomialRoot(poly, lo, hi, budget)
			if e != nil {
				return r, e
			}
			if candidate != nil {
				lo.Set(candidate)
				hi.Set(candidate)
			}
		}
		independent, dependent := ray.Axis, 1-ray.Axis
		side := 0
		if ray.Direction > 0 {
			side = 1
		}
		fixed := rectangle[independent][side]
		delta := new(big.Rat).Sub(fixed, uv[independent])
		var terminal GraphSaddleTerminal
		terminal.Ray = index
		resolved := false
		stepsUsed := 0
		for !resolved {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			// Detect exact corner slopes before deciding a strict side inequality.
			if ray.Axis == 0 {
				for depSide := 0; depSide < 2; depSide++ {
					corner := new(big.Rat).Quo(new(big.Rat).Sub(rectangle[dependent][depSide], uv[dependent]), delta)
					if corner.Cmp(lo) < 0 || corner.Cmp(hi) > 0 {
						continue
					}
					value, e := polynomialValue(poly, corner, budget)
					if e != nil {
						return r, e
					}
					if value.Sign() == 0 {
						lo.Set(corner)
						hi.Set(corner)
						break
					}
				}
			}
			first := new(big.Rat).Add(uv[dependent], new(big.Rat).Mul(lo, delta))
			last := new(big.Rat).Add(uv[dependent], new(big.Rat).Mul(hi, delta))
			if first.Cmp(last) > 0 {
				first, last = last, first
			}
			if first.Cmp(rectangle[dependent][0]) >= 0 && last.Cmp(rectangle[dependent][1]) <= 0 {
				terminal.Lower[independent], terminal.Upper[independent] = new(big.Rat).Set(fixed), new(big.Rat).Set(fixed)
				terminal.Lower[dependent], terminal.Upper[dependent] = first, last
				terminal.SideMask = 1 << uint(2*independent+side)
				resolved = true
				if first.Cmp(last) == 0 {
					for depSide := 0; depSide < 2; depSide++ {
						if first.Cmp(rectangle[dependent][depSide]) == 0 {
							terminal.SideMask |= 1 << uint(2*dependent+depSide)
						}
					}
				}
			} else {
				depSide := -1
				if last.Cmp(rectangle[dependent][0]) < 0 {
					depSide = 0
				}
				if first.Cmp(rectangle[dependent][1]) > 0 {
					depSide = 1
				}
				if depSide >= 0 && lo.Sign()*hi.Sign() > 0 {
					depFixed := rectangle[dependent][depSide]
					numerator := new(big.Rat).Sub(depFixed, uv[dependent])
					x := new(big.Rat).Add(uv[independent], new(big.Rat).Quo(numerator, lo))
					y := new(big.Rat).Add(uv[independent], new(big.Rat).Quo(numerator, hi))
					if x.Cmp(y) > 0 {
						x, y = y, x
					}
					terminal.Lower[independent], terminal.Upper[independent] = x, y
					terminal.Lower[dependent], terminal.Upper[dependent] = new(big.Rat).Set(depFixed), new(big.Rat).Set(depFixed)
					terminal.SideMask = 1 << uint(2*dependent+depSide)
					resolved = true
				}
			}
			if resolved {
				break
			}
			if ray.Axis != 0 || lo.Cmp(hi) == 0 || stepsUsed >= 8192 {
				return r, ErrConvergence
			}
			target := new(big.Rat).Quo(new(big.Rat).Sub(hi, lo), big.NewRat(2, 1))
			steps, e := isolatePolynomialRoots(poly, lo, hi, budget, 8192-stepsUsed, func(x, y *big.Rat, _ int) (bool, error) {
				if new(big.Rat).Sub(y, x).Cmp(target) > 0 {
					return false, nil
				}
				lo.Set(x)
				hi.Set(y)
				return true, ctx.Err()
			})
			stepsUsed += steps
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
		}
		terminal.Exact = true
		for axis := 0; axis < 2; axis++ {
			for _, x := range []*big.Rat{terminal.Lower[axis], terminal.Upper[axis]} {
				if e := budget.check(x); e != nil {
					return r, e
				}
			}
			terminal.Exact = terminal.Exact && terminal.Lower[axis].Cmp(terminal.Upper[axis]) == 0
		}
		terminal.SlopeLower, terminal.SlopeUpper = new(big.Rat).Set(lo), new(big.Rat).Set(hi)
		if terminal.Exact {
			remaining := budget.maxOperations - budget.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			jet, e := a.EvaluateRational(ctx, terminal.Lower[0], terminal.Lower[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			budget.operations += jet.ExactOperations
			if e != nil {
				return r, e
			}
			terminal.Point = jet.Point
		}
		r.Terminals = append(r.Terminals, terminal)
	}
	r.Complete = true
	return r, nil
}
