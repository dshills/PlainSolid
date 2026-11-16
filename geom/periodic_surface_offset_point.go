package geom

import (
	"context"
	"math/big"
)

// NormalOffsetPointProof describes the differential of S + distance*n at
// one source parameter, with n oriented by Su × Sv. The signs refer to the
// first leading minor and determinant of the offset differential paired
// with the source first fundamental form.
type NormalOffsetPointProof struct {
	FirstMinorSign, DeterminantSign int
	FocalFree                       bool
	ExactOperations                 int
}

// CheckNormalOffset proves whether every displacement from zero through
// distance has a nonsingular normal offset differential at this source
// point. It uses the exact one-sided derivatives of the jet and does not
// round the unit normal or eigenvalues to float64. It does not prove
// regularity over a face or exclude collision with another source point.
// A degenerate source tangent pair returns ErrDegenerate.
func (s RationalPeriodicSurfaceSecondJet) CheckNormalOffset(ctx context.Context, distance *big.Rat,
	o SurfacePatchOptions) (result NormalOffsetPointProof, err error) {
	if distance == nil {
		return result, ErrInvalid
	}
	b, err := periodicDerivativeBudget(ctx, 2, 0, o)
	if err != nil {
		return result, err
	}
	defer func() { result.ExactOperations = b.operations }()
	if err := b.check(distance); err != nil {
		return result, err
	}
	for _, v := range [5][3]*big.Rat{s.Du, s.Dv, s.Duu, s.Duv, s.Dvv} {
		for _, x := range v {
			if x == nil {
				return result, ErrInvalid
			}
			if err := b.check(x); err != nil {
				return result, err
			}
		}
	}
	a := boundedRatArithmetic{b: b}
	var normal [3]*big.Rat
	for i := range normal {
		j, k := (i+1)%3, (i+2)%3
		normal[i] = a.sub(a.mul(s.Du[j], s.Dv[k]), a.mul(s.Du[k], s.Dv[j]))
	}
	dot := func(x, y [3]*big.Rat) *big.Rat {
		return a.add(a.add(a.mul(x[0], y[0]), a.mul(x[1], y[1])), a.mul(x[2], y[2]))
	}
	d := dot(normal, normal)
	if a.err != nil {
		return result, a.err
	}
	if d.Sign() == 0 {
		return result, ErrDegenerate
	}
	e, f, g := dot(s.Du, s.Du), dot(s.Du, s.Dv), dot(s.Dv, s.Dv)
	l, m, n := dot(s.Duu, normal), dot(s.Duv, normal), dot(s.Dvv, normal)
	p := a.sub(a.mul(l, n), a.mul(m, m))
	fm := a.mul(f, m)
	t := a.sub(a.add(a.mul(g, l), a.mul(e, n)), a.add(fm, fm))
	// I - distance*shape is self-adjoint for the first fundamental
	// form. Sylvester's test reduces to these two exact radical signs
	// after multiplying by sqrt(d) and d, respectively.
	first := a.radicalSign(a.sub(new(big.Rat), a.mul(distance, l)), e, d)
	det := a.radicalSign(a.add(a.mul(d, d), a.mul(a.mul(distance, distance), p)),
		a.sub(new(big.Rat), a.mul(distance, t)), d)
	if a.err != nil {
		return result, a.err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// Positive definiteness at both ends proves it along the entire
	// interval by convexity. If it fails at the far end, an eigenvalue
	// crosses zero on the way from the positive definite source.
	return NormalOffsetPointProof{FirstMinorSign: first, DeterminantSign: det, FocalFree: first > 0 && det > 0}, nil
}

type boundedRatArithmetic struct {
	b   *polynomialBudget
	err error
}

func (a *boundedRatArithmetic) op(f func(*big.Rat, *big.Rat, *big.Rat) *big.Rat, x, y *big.Rat) *big.Rat {
	out := new(big.Rat)
	if a.err == nil {
		a.err = a.b.check(f(out, x, y))
	}
	return out
}

func (a *boundedRatArithmetic) add(x, y *big.Rat) *big.Rat { return a.op((*big.Rat).Add, x, y) }
func (a *boundedRatArithmetic) sub(x, y *big.Rat) *big.Rat { return a.op((*big.Rat).Sub, x, y) }
func (a *boundedRatArithmetic) mul(x, y *big.Rat) *big.Rat { return a.op((*big.Rat).Mul, x, y) }
func (a *boundedRatArithmetic) quo(x, y *big.Rat) *big.Rat { return a.op((*big.Rat).Quo, x, y) }

// radicalSign returns the sign of x+y*sqrt(d) for strictly positive d.
// Squaring is valid only when the summands have opposing signs.
func (a *boundedRatArithmetic) radicalSign(x, y, d *big.Rat) int {
	sx, sy := x.Sign(), y.Sign()
	switch {
	case sx == 0:
		return sy
	case sy == 0 || sx == sy:
		return sx
	default:
		return sx * a.mul(x, x).Cmp(a.mul(a.mul(y, y), d))
	}
}
