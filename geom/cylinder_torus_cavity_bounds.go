package geom

import (
	"context"
	"math/big"
)

// CylinderTorusCavityExtremum describes an attained ORIGINAL source family.
// If RadialSquared>0, (x,y)=RadialVector/sqrt(RadialSquared).
// Otherwise the whole radius-C cap disk attains the support. Height is an
// exact singleton or the whole wall axial interval. World mapping uses the
// supplied affine matrix exactly; an irrational representative is not rounded.
type CylinderTorusCavityExtremum struct {
	RadialVector          [2]*big.Rat
	RadialSquared, Radius *big.Rat
	Height                [2]*big.Rat
	Owners                BoreBoundaryMask
	CapDisk               bool
}

// CylinderTorusCavityBounds encloses each true minimum/maximum separately.
// Extrema[axis][0/1] are exact attained families for the corresponding bound.
// The original torus is strictly internal and cannot attain any world extreme.
type CylinderTorusCavityBounds struct {
	Minimum, Maximum [3][2]*big.Rat
	Extrema          [3][2]CylinderTorusCavityExtremum
	ExactOperations  int
}

// BoundAffineRange accepts a nonsingular rational affine matrix with translation.
// One bounded arithmetic ledger covers determinant, support and sqrt bounds.
// Precision defaults64 bits (16..256). No mesh or approximate pose is used.
func (s CylinderTorusCavityMaterial) BoundAffineRange(ctx context.Context, m [3][4]*big.Rat, bits int, o SurfacePatchOptions) (out CylinderTorusCavityBounds, err error) {
	defer func() {
		if err != nil {
			out = CylinderTorusCavityBounds{}
		}
	}()
	if ctx == nil || s.source.R == nil {
		return out, ErrInvalid
	}
	if bits == 0 {
		bits = 64
	}
	if bits < 16 || bits > 256 {
		return out, ErrInvalid
	}
	lim, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	budget := &polynomialBudget{maxOperations: lim.MaxOperations, maxBits: lim.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: budget}
	for _, row := range m {
		for _, v := range row {
			if v == nil {
				return out, ErrInvalid
			}
			a.check(v)
		}
	}
	for _, v := range []*big.Rat{s.source.c, s.source.lower, s.source.upper} {
		a.check(v)
	}
	if a.err != nil {
		return out, a.err
	}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det = a.add(det, a.mul(m[0][i], a.sub(a.mul(m[1][j], m[2][k]), a.mul(m[1][k], m[2][j]))))
	}
	if a.err != nil {
		return out, a.err
	}
	if det.Sign() == 0 {
		return out, ErrDomain
	}
	for axis, row := range m {
		q := a.add(a.mul(row[0], row[0]), a.mul(row[1], row[1]))
		lo, hi, e := rationalSqrtBounds(q, bits, budget)
		if e != nil {
			return out, e
		}
		lo, hi = a.mul(s.source.c, lo), a.mul(s.source.c, hi)
		for side := 0; side < 2; side++ {
			sign := int64(2*side - 1)
			z := s.source.lower
			if row[2].Sign()*int(sign) > 0 {
				z = s.source.upper
			}
			base := a.add(row[3], a.mul(row[2], z))
			v := [2]*big.Rat{a.add(base, lo), a.add(base, hi)}
			if side == 0 {
				v = [2]*big.Rat{a.sub(base, hi), a.sub(base, lo)}
				out.Minimum[axis] = v
			} else {
				out.Maximum[axis] = v
			}
			w := CylinderTorusCavityExtremum{RadialSquared: new(big.Rat).Set(q), Radius: new(big.Rat).Set(s.source.c), Height: [2]*big.Rat{new(big.Rat).Set(z), new(big.Rat).Set(z)}, CapDisk: q.Sign() == 0}
			for j := 0; j < 2; j++ {
				w.RadialVector[j] = a.mul(big.NewRat(sign, 1), a.mul(s.source.c, row[j]))
			}
			if q.Sign() > 0 {
				w.Owners |= TorusCylinderWall
			}
			if row[2].Sign() == 0 {
				w.Height = [2]*big.Rat{new(big.Rat).Set(s.source.lower), new(big.Rat).Set(s.source.upper)}
			} else if z.Cmp(s.source.lower) == 0 {
				w.Owners |= TorusCylinderLowerCap
			} else {
				w.Owners |= TorusCylinderUpperCap
			}
			out.Extrema[axis][side] = w
		}
	}
	if a.err != nil {
		return out, a.err
	}
	out.ExactOperations = budget.operations
	return out, nil
}
