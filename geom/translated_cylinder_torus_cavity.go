package geom

import (
	"context"
	"math/big"
)

// TranslatedCylinderTorusCavityMaterial retains an original cylinder at XY0
// and a ring torus centered(dx,dy,dz). Only strict containment is supported.
// Shifted cap values must be exactly represented; no rounding is substituted.
type TranslatedCylinderTorusCavityMaterial struct {
	source EccentricCylinderTorusCavityMaterial
	z      *big.Rat
}

func NewTranslatedCylinderTorusCavityMaterial(R, r, c, lower, upper, dx, dy, dz float64) (TranslatedCylinderTorusCavityMaterial, error) {
	var zero TranslatedCylinderTorusCavityMaterial
	if !finite(dz) || !finite(lower) || !finite(upper) {
		return zero, ErrInvalid
	}
	z := new(big.Rat).SetFloat64(dz)
	lo, e := RepresentedExactScalar(new(big.Rat).Sub(new(big.Rat).SetFloat64(lower), z))
	if e != nil {
		return zero, e
	}
	hi, e := RepresentedExactScalar(new(big.Rat).Sub(new(big.Rat).SetFloat64(upper), z))
	if e != nil {
		return zero, e
	}
	q, e := NewEccentricCylinderTorusCavityMaterial(R, r, c, lo, hi, dx, dy)
	if e != nil {
		return zero, e
	}
	return TranslatedCylinderTorusCavityMaterial{q, z}, nil
}

// ShiftedSource returns immutable material in coordinates (x,y,z-centerZ),
// and an owned exact translation. This is not the original world chart.
func (s TranslatedCylinderTorusCavityMaterial) ShiftedSource() (EccentricCylinderTorusCavityMaterial, *big.Rat, error) {
	if s.z == nil {
		return EccentricCylinderTorusCavityMaterial{}, nil, ErrInvalid
	}
	return s.source, new(big.Rat).Set(s.z), nil
}
func (s TranslatedCylinderTorusCavityMaterial) shift(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) ([3]*big.Rat, error) {
	if ctx == nil || s.z == nil {
		return [3]*big.Rat{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return [3]*big.Rat{}, e
	}
	lim, e := patchOptions(o)
	if e != nil {
		return [3]*big.Rat{}, e
	}
	b := &polynomialBudget{maxOperations: lim.MaxOperations, maxBits: lim.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, v := range p {
		if v == nil {
			return [3]*big.Rat{}, ErrInvalid
		}
		a.check(v)
	}
	a.check(s.z)
	out := [3]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1]), a.sub(p[2], s.z)}
	if a.err != nil {
		return [3]*big.Rat{}, a.err
	}
	return out, nil
}

// ClassifyRational retains original masks after exact translation. Translation
// has its own bounded preflight ledger (constant-size input checks and one subtraction); source proof retains
// its unchanged arithmetic budget. Either failure is atomic.
func (s TranslatedCylinderTorusCavityMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
	q, e := s.shift(ctx, p, o)
	if e != nil {
		return BoreOutside, 0, e
	}
	return s.source.ClassifyRational(ctx, q, o)
}

// ClipRationalLine preserves exact original line t and original support IDs.
func (s TranslatedCylinderTorusCavityMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
	q, e := s.shift(ctx, p, o.Surface)
	if e != nil {
		return TorusCylinderLine{}, e
	}
	return s.source.ClipRationalLine(ctx, q, d, min, max, o)
}

// BoundAffineRange uses the unchanged original cylinder support extrema.
// Strict translated containment proves the inner torus cannot attain them.
func (s TranslatedCylinderTorusCavityMaterial) BoundAffineRange(ctx context.Context, m [3][4]*big.Rat, bits int, o SurfacePatchOptions) (CylinderTorusCavityBounds, error) {
	if s.z == nil {
		return CylinderTorusCavityBounds{}, ErrInvalid
	}
	outer := s.source.source
	outer.lower = new(big.Rat).Add(outer.lower, s.z)
	outer.upper = new(big.Rat).Add(outer.upper, s.z)
	return (CylinderTorusCavityMaterial{source: outer}).BoundAffineRange(ctx, m, bits, o)
}
