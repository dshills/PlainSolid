package geom

import (
	"context"
	"math/big"
)

// ToroidAxialOptions bounds exact axial-line root isolation. AbsoluteTolerance
// is a required positive model-Z bracket width. MaxBisections defaults to256
// (maximum4096), shared between both roots; Surface bounds proof/arithmetic.
type ToroidAxialOptions struct {
	AbsoluteTolerance float64
	MaxBisections     int
	Surface           SurfacePatchOptions
}

// ToroidAxialRoot is a closed exact rational Z interval containing one root.
// Multiplicity is two for tangent contact, otherwise one. Equal bounds denote
// an exact rational root. The intervals and values are caller-owned.
type ToroidAxialRoot struct {
	MinZ, MaxZ   *big.Rat
	Multiplicity int
}

// IntersectWrappedCircularToroidAxial intersects the complete Z-directed line
// at represented model coordinates x,y with a canonical circular-profile
// wrapped toroid. Results are sorted by Z. It returns zero, one double, or two
// simple roots, proving circle identities before using the implicit equation.
// Brackets are rational algebraic enclosures, not rounded surface samples.
// Failure returns no partial roots. No general line or noncircular claim.
func IntersectWrappedCircularToroidAxial(ctx context.Context, w *WrappedSurfacePatch, x, y float64, o ToroidAxialOptions) ([]ToroidAxialRoot, error) {
	if !finite(x) || !finite(y) || !finite(o.AbsoluteTolerance) || o.AbsoluteTolerance <= 0 {
		return nil, ErrInvalid
	}
	if o.MaxBisections == 0 {
		o.MaxBisections = 256
	}
	if o.MaxBisections < 1 || o.MaxBisections > 4096 {
		return nil, ErrInvalid
	}
	p, e := CertifyWrappedToroidEmbedding(ctx, w, o.Surface)
	if e != nil {
		return nil, e
	}
	options, e := patchOptions(o.Surface)
	if e != nil {
		return nil, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: p.ExactOperations}
	for _, stride := range []int{1, 9} {
		for j := 0; j < 8; j += 2 {
			a, b, c := w.patch.net[j*stride][3], w.patch.net[(j+1)*stride][3], w.patch.net[(j+2)*stride][3]
			l, r := new(big.Rat).Mul(a, c), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(b, b))
			if e := budget.check(l); e != nil {
				return nil, e
			}
			if e := budget.check(r); e != nil {
				return nil, e
			}
			if l.Cmp(r) != 0 {
				return nil, ErrConvergence
			}
		}
	}
	dx, dy := new(big.Rat).Sub(new(big.Rat).SetFloat64(x), p.Center[0]), new(big.Rat).Sub(new(big.Rat).SetFloat64(y), p.Center[1])
	q := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	R2 := new(big.Rat).Mul(p.MajorRadius, p.MajorRadius)
	r2 := new(big.Rat).Mul(p.RadialMinorRadius, p.RadialMinorRadius)
	h2 := new(big.Rat).Mul(p.AxialMinorRadius, p.AxialMinorRadius)
	rhs := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(R2, q))
	// sign returns negative inside, zero boundary, positive outside, at exact
	// height relative to Cz. A<0 is tested before the squared equation.
	sign := func(z *big.Rat) (int, error) {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		v := new(big.Rat).Quo(new(big.Rat).Mul(z, z), h2)
		a := new(big.Rat).Add(q, R2)
		a.Add(a, new(big.Rat).Mul(r2, new(big.Rat).Sub(v, big.NewRat(1, 1))))
		lhs := new(big.Rat).Mul(a, a)
		for _, n := range []*big.Rat{q, R2, r2, h2, rhs, v, a, lhs} {
			if e := budget.check(n); e != nil {
				return 0, e
			}
		}
		if a.Sign() < 0 {
			return -1, nil
		}
		return lhs.Cmp(rhs), nil
	}
	center, e := sign(new(big.Rat))
	if e != nil {
		return nil, e
	}
	if center > 0 {
		return []ToroidAxialRoot{}, nil
	}
	if center == 0 {
		return []ToroidAxialRoot{{new(big.Rat).Set(p.Center[2]), new(big.Rat).Set(p.Center[2]), 2}}, nil
	}
	low, high := new(big.Rat), new(big.Rat).Set(p.AxialMinorRadius)
	s, e := sign(high)
	if e != nil {
		return nil, e
	}
	if s == 0 {
		low.Set(high)
	} else if s < 0 {
		return nil, ErrConvergence
	}
	tolerance := new(big.Rat).SetFloat64(o.AbsoluteTolerance)
	for iterations := 0; new(big.Rat).Sub(high, low).Cmp(tolerance) > 0; iterations++ {
		if iterations >= o.MaxBisections {
			return nil, ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(low, high), big.NewRat(2, 1))
		s, e := sign(mid)
		if e != nil {
			return nil, e
		}
		if s == 0 {
			low.Set(mid)
			high.Set(mid)
		} else if s < 0 {
			low.Set(mid)
		} else {
			high.Set(mid)
		}
	}
	return []ToroidAxialRoot{
		{new(big.Rat).Sub(p.Center[2], high), new(big.Rat).Sub(p.Center[2], low), 1},
		{new(big.Rat).Add(p.Center[2], low), new(big.Rat).Add(p.Center[2], high), 1},
	}, nil
}
