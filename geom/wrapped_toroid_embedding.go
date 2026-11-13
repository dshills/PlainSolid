package geom

import (
	"context"
	"math/big"
)

// WrappedToroidEmbedding proves quotient embedding and outward chart
// orientation for a canonical tensor rational toroid about the Z axis.
// AxialMinorRadius may differ from RadialMinorRadius. Values are caller-owned.
type WrappedToroidEmbedding struct {
	Center                                           [3]*big.Rat
	MajorRadius, RadialMinorRadius, AxialMinorRadius *big.Rat
	Complete, Outward, CircularProfiles              bool
	ExactOperations                                  int
}

// CertifyWrappedToroidEmbedding proves a sufficient exact geometric subset.
// Both axes have four clamped quadratic spans with double interior knots and
// the canonical nine quadrant controls; their positive weight tensor must
// factor into independent U and V weights. The chart must have the form
//
//	(Cx + (R+r*Vx)*Ux, Cy + (R+r*Vx)*Uy, Cz + h*Vy)
//
// with R>r>0 and h>0. U and V each have the counterclockwise control
// polygon (1,0),(1,1),(0,1),..., (1,0). Rotated or edited bases may be
// geometrically valid but outside this certificate.
//
// Each positive-weight quadrant has strictly increasing polar angle:
// the derivative numerator is a positive combination of the pairwise
// determinants of its ordered controls. It has no zero speed. Thus the
// model azimuth determines U uniquely modulo its seam. With U fixed,
// radius and height determine (Vx,Vy), hence V uniquely modulo its seam.
// The same coordinates prove regularity inside every span and nonzero
// one-sided derivatives at knots. The meridian encloses a star-shaped
// disk with strictly positive model radius. The chart normal at its
// exterior equator points outward, establishing the orientation globally.
func CertifyWrappedToroidEmbedding(ctx context.Context, w *WrappedSurfacePatch, o SurfacePatchOptions) (r WrappedToroidEmbedding, err error) {
	o, e := patchOptions(o)
	if ctx == nil || e != nil || w == nil || w.patch == nil {
		return r, ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return r, e
	}
	p := w.patch
	if w.seam.PositionMatched != ([2]bool{true, true}) || p.countU != 9 || p.countV != 9 ||
		p.degreeU != 2 || p.degreeV != 2 || len(p.net) != 81 || len(p.net) > o.MaxPoles {
		return r, ErrConvergence
	}
	quarters := func(k []float64) bool {
		return len(k) == 12 && k[0] == k[1] && k[1] == k[2] && k[2] < k[3] &&
			k[3] == k[4] && k[4] < k[5] && k[5] == k[6] && k[6] < k[7] &&
			k[7] == k[8] && k[8] < k[9] && k[9] == k[10] && k[10] == k[11]
	}
	if !quarters(p.knotsU) || !quarters(p.knotsV) {
		return r, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	at := func(i, j, axis int) *big.Rat {
		k := p.net[9*i+j]
		return new(big.Rat).Quo(k[axis], k[3])
	}
	average := func(x, y *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Add(x, y), big.NewRat(2, 1)) }
	x0, y0, z0 := average(at(0, 0, 0), at(4, 0, 0)), at(0, 0, 1), at(0, 0, 2)
	xOuter := new(big.Rat).Sub(at(0, 0, 0), x0)
	xInner := new(big.Rat).Sub(at(0, 4, 0), x0)
	major := average(xOuter, xInner)
	minor := new(big.Rat).Sub(xOuter, major)
	axial := new(big.Rat).Sub(at(0, 2, 2), z0)
	for _, q := range []*big.Rat{x0, y0, z0, major, minor, axial} {
		if e = b.check(q); e != nil {
			return r, e
		}
	}
	if minor.Sign() <= 0 || axial.Sign() <= 0 || major.Cmp(minor) <= 0 {
		return r, ErrConvergence
	}
	q := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	scale := func(a *big.Rat, n int64) *big.Rat { return new(big.Rat).Mul(a, big.NewRat(n, 1)) }
	for i, u := range q {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		for j, v := range q {
			c := p.net[9*i+j]
			left := new(big.Rat).Mul(c[3], p.net[0][3])
			right := new(big.Rat).Mul(p.net[9*i][3], p.net[j][3])
			for _, w := range []*big.Rat{left, right} {
				if e = b.check(w); e != nil {
					return r, e
				}
			}
			if left.Cmp(right) != 0 {
				return r, ErrConvergence
			}
			rho := new(big.Rat).Add(major, scale(minor, v[0]))
			want := [3]*big.Rat{
				new(big.Rat).Add(x0, scale(rho, u[0])),
				new(big.Rat).Add(y0, scale(rho, u[1])),
				new(big.Rat).Add(z0, scale(axial, v[1])),
			}
			for k, x := range want {
				h := new(big.Rat).Mul(x, c[3])
				if e = b.check(h); e != nil {
					return r, e
				}
				if h.Cmp(c[k]) != 0 {
					return r, ErrConvergence
				}
			}
		}
	}
	r.Center, r.MajorRadius, r.RadialMinorRadius, r.AxialMinorRadius = [3]*big.Rat{x0, y0, z0}, major, minor, axial
	r.CircularProfiles, e = proveWrappedToroidCircleWeights(w, b)
	if e != nil {
		return r, e
	}
	r.Complete, r.Outward = true, true
	return r, nil
}

func proveWrappedToroidCircleWeights(w *WrappedSurfacePatch, b *polynomialBudget) (bool, error) {
	for _, stride := range []int{1, 9} {
		for j := 0; j < 8; j += 2 {
			a, x, c := w.patch.net[j*stride][3], w.patch.net[(j+1)*stride][3], w.patch.net[(j+2)*stride][3]
			left := new(big.Rat).Mul(a, c)
			right := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(x, x))
			if e := b.check(left); e != nil {
				return false, e
			}
			if e := b.check(right); e != nil {
				return false, e
			}
			if left.Cmp(right) != 0 {
				return false, nil
			}
		}
	}
	return true, nil
}
