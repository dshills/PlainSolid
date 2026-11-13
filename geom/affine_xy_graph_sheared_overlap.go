package geom

import (
	"context"
	"math/big"
)

// ShearedGraphOverlap describes the full positive-area common projection.
// A and B are paired convex polygon vertices in original chart units, ordered
// counterclockwise in A. Orientation is sign(det(A→B)); NoArea excludes only
// area, never isolated or edge contacts. Rational results are caller-owned.
type ShearedGraphOverlap struct {
	Coincident, NoArea    bool
	A, B                  [][2]*big.Rat
	Map                   [2][3]*big.Rat
	Orientation           int
	ProjectionOrientation [2]int
	ExactOperations       int
}

// CertifyShearedAffineXYGraphOverlap supports one Bézier cell per input with
// degrees at most three per axis. Exact polygon clipping and homogeneous power
// identities prove coincidence on the entire overlap, including rational
// nonbinary corners. Nonidentity returns ErrConvergence, not disjointness.
func CertifyShearedAffineXYGraphOverlap(ctx context.Context, source, target *SurfacePatch, o SurfacePatchOptions) (out ShearedGraphOverlap, err error) {
	if ctx == nil || source == nil || target == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { out.ExactOperations = b.operations }()
	for _, p := range []*SurfacePatch{source, target} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.degreeU > 3 || p.degreeV > 3 {
			return out, ErrConvergence
		}
	}
	var inverse [2][2][3]*big.Rat
	for i, p := range []*SurfacePatch{source, target} {
		_, inverse[i], e = normalizeAffineXYGraph(ctx, p, b)
		if e != nil {
			return out, e
		}
	}

	for i, inv := range inverse {
		determinant := new(big.Rat).Sub(new(big.Rat).Mul(inv[0][0], inv[1][1]), new(big.Rat).Mul(inv[0][1], inv[1][0]))
		if e := b.check(determinant); e != nil {
			return out, e
		}
		out.ProjectionOrientation[i] = determinant.Sign()
	}
	// Invert source inverse XY map, then compose with target inverse.
	a := inverse[0]
	det := new(big.Rat).Sub(new(big.Rat).Mul(a[0][0], a[1][1]), new(big.Rat).Mul(a[0][1], a[1][0]))
	forward := [2][3]*big.Rat{{new(big.Rat).Quo(a[1][1], det), new(big.Rat).Quo(new(big.Rat).Neg(a[0][1]), det), nil}, {new(big.Rat).Quo(new(big.Rat).Neg(a[1][0]), det), new(big.Rat).Quo(a[0][0], det), nil}}
	for i := 0; i < 2; i++ {
		forward[i][2] = new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(forward[i][0], a[0][2]), new(big.Rat).Mul(forward[i][1], a[1][2])))
	}

	for i := 0; i < 2; i++ {
		for j := 0; j < 3; j++ {
			q := new(big.Rat)
			for k := 0; k < 2; k++ {
				q.Add(q, new(big.Rat).Mul(inverse[1][i][k], forward[k][j]))
			}
			if j == 2 {
				q.Add(q, inverse[1][i][2])
			}
			if e = b.check(q); e != nil {
				return out, e
			}
			out.Map[i][j] = q
		}
	}

	determinant := new(big.Rat).Sub(new(big.Rat).Mul(out.Map[0][0], out.Map[1][1]), new(big.Rat).Mul(out.Map[0][1], out.Map[1][0]))
	out.Orientation = determinant.Sign()
	sd, td := source.Domain(), target.Domain()
	sl := [2]*big.Rat{new(big.Rat).SetFloat64(sd.UMin), new(big.Rat).SetFloat64(sd.VMin)}
	sh := [2]*big.Rat{new(big.Rat).SetFloat64(sd.UMax), new(big.Rat).SetFloat64(sd.VMax)}
	tl := [2]*big.Rat{new(big.Rat).SetFloat64(td.UMin), new(big.Rat).SetFloat64(td.VMin)}
	th := [2]*big.Rat{new(big.Rat).SetFloat64(td.UMax), new(big.Rat).SetFloat64(td.VMax)}
	var normalized [2][3]*big.Rat
	for i := 0; i < 2; i++ {
		span := new(big.Rat).Sub(th[i], tl[i])
		constant := new(big.Rat).Sub(out.Map[i][2], tl[i])
		for j := 0; j < 2; j++ {
			constant.Add(constant, new(big.Rat).Mul(out.Map[i][j], sl[j]))
			normalized[i][j] = new(big.Rat).Quo(new(big.Rat).Mul(out.Map[i][j], new(big.Rat).Sub(sh[j], sl[j])), span)
		}
		normalized[i][2] = new(big.Rat).Quo(constant, span)
		for _, q := range normalized[i] {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	poly := [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}
	for axis := 0; axis < 2; axis++ {
		for side := 0; side < 2; side++ {
			row := normalized[axis]
			if side == 1 {
				row = [3]*big.Rat{new(big.Rat).Neg(row[0]), new(big.Rat).Neg(row[1]), new(big.Rat).Sub(big.NewRat(1, 1), row[2])}
			}
			poly, e = clipShearPolygon(poly, row, b)
			if e != nil {
				return out, e
			}
		}
	}
	area := new(big.Rat)
	for i, p := range poly {
		q := poly[(i+1)%len(poly)]
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(p[0], q[1]), new(big.Rat).Mul(p[1], q[0])))
	}
	if e := b.check(area); e != nil {
		return out, e
	}
	if len(poly) < 3 || area.Sign() == 0 {
		out.NoArea = true
		return out, nil
	}
	var powers [2][4]shearPolynomial
	for owner, p := range []*SurfacePatch{source, target} {
		for axis := 0; axis < 4; axis++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			coeff, e := tensorPowerCoefficients(p.net, p.countU, p.countV, axis, b)
			if e != nil {
				return out, e
			}
			power := shearPolynomial{}
			for i := 0; i < p.countU; i++ {
				for j := 0; j < p.countV; j++ {
					power[[2]int{i, j}] = coeff[i*p.countV+j]
				}
			}
			if owner == 1 {
				power, e = composeShearPolynomial(power, normalized, b)
				if e != nil {
					return out, e
				}
			}
			powers[owner][axis] = power
		}
	}
	for axis := 0; axis < 3; axis++ {
		left, e := multiplyShearPolynomial(powers[0][axis], powers[1][3], b)
		if e != nil {
			return out, e
		}
		right, e := multiplyShearPolynomial(powers[1][axis], powers[0][3], b)
		if e != nil {
			return out, e
		}
		for ij, q := range left {
			if v := right[ij]; (v == nil && q.Sign() != 0) || (v != nil && v.Cmp(q) != 0) {
				return out, ErrConvergence
			}
		}
		for ij, q := range right {
			if v := left[ij]; v == nil && q.Sign() != 0 {
				return out, ErrConvergence
			}
		}
	}
	for _, p := range poly {
		var pa, pb [2]*big.Rat
		for axis := 0; axis < 2; axis++ {
			pa[axis] = new(big.Rat).Add(sl[axis], new(big.Rat).Mul(new(big.Rat).Sub(sh[axis], sl[axis]), p[axis]))
		}
		for axis := 0; axis < 2; axis++ {
			pb[axis] = new(big.Rat).Set(out.Map[axis][2])
			for j := 0; j < 2; j++ {
				pb[axis].Add(pb[axis], new(big.Rat).Mul(out.Map[axis][j], pa[j]))
			}
			for _, q := range []*big.Rat{pa[axis], pb[axis]} {
				if e := b.check(q); e != nil {
					return out, e
				}
			}
		}
		out.A = append(out.A, pa)
		out.B = append(out.B, pb)
	}
	out.Coincident = true
	return out, nil
}

type shearPolynomial map[[2]int]*big.Rat

func multiplyShearPolynomial(a, c shearPolynomial, b *polynomialBudget) (shearPolynomial, error) {
	r := shearPolynomial{}
	for ij, x := range a {
		for kl, y := range c {
			key := [2]int{ij[0] + kl[0], ij[1] + kl[1]}
			if r[key] == nil {
				r[key] = new(big.Rat)
			}
			r[key].Add(r[key], new(big.Rat).Mul(x, y))
			if e := b.check(r[key]); e != nil {
				return nil, e
			}
		}
	}
	return r, nil
}
func composeShearPolynomial(p shearPolynomial, m [2][3]*big.Rat, b *polynomialBudget) (shearPolynomial, error) {
	r := shearPolynomial{}
	for ij, q := range p {
		term := shearPolynomial{{0, 0}: new(big.Rat).Set(q)}
		for axis, n := range ij {
			linear := shearPolynomial{{1, 0}: m[axis][0], {0, 1}: m[axis][1], {0, 0}: m[axis][2]}
			for k := 0; k < n; k++ {
				var e error
				term, e = multiplyShearPolynomial(term, linear, b)
				if e != nil {
					return nil, e
				}
			}
		}
		for key, v := range term {
			if r[key] == nil {
				r[key] = new(big.Rat)
			}
			r[key].Add(r[key], v)
			if e := b.check(r[key]); e != nil {
				return nil, e
			}
		}
	}
	return r, nil
}
func clipShearPolygon(p [][2]*big.Rat, row [3]*big.Rat, b *polynomialBudget) ([][2]*big.Rat, error) {
	var out [][2]*big.Rat
	if len(p) == 0 {
		return out, nil
	}
	value := func(p [2]*big.Rat) *big.Rat {
		return new(big.Rat).Add(row[2], new(big.Rat).Add(new(big.Rat).Mul(row[0], p[0]), new(big.Rat).Mul(row[1], p[1])))
	}
	for i, a := range p {
		c := p[(i+1)%len(p)]
		fa, fc := value(a), value(c)
		if e := b.check(fc); e != nil {
			return nil, e
		}
		if e := b.check(fa); e != nil {
			return nil, e
		}
		if fa.Sign() >= 0 {
			out = append(out, a)
		}
		if fa.Sign()*fc.Sign() < 0 {
			t := new(big.Rat).Quo(fa, new(big.Rat).Sub(fa, fc))
			var q [2]*big.Rat
			for j := 0; j < 2; j++ {
				q[j] = new(big.Rat).Add(a[j], new(big.Rat).Mul(t, new(big.Rat).Sub(c[j], a[j])))
				if e := b.check(q[j]); e != nil {
					return nil, e
				}
			}
			out = append(out, q)
		}
	}
	return out, nil
}
