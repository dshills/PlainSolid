package geom

import (
	"context"
	"math/big"
)

type BorePointLocation int

const (
	BoreOutside BorePointLocation = iota
	BoreInside
	BoreBoundary
)

type BoreBoundaryMask uint8

const (
	BoreSphere BoreBoundaryMask = 1 << iota
	BoreCylinder
	BoreFloor
)

// OffAxisSphereBore is immutable material in sphere-centered coordinates:
// |p|≤R minus the open offset-cylinder pocket above the planar floor.
// It does not depend on a sampled or sewn boundary model.
type OffAxisSphereBore struct{ R, r, cx, cy, floor *big.Rat }

// NewOffAxisSphereBore proves d+r<R and floor²+(d+r)²<R² using exact
// squared rational inequalities, without evaluating d=sqrt(cx²+cy²).
// Thus every cylindrical rim is transverse and above the floor. The broader
// d>=r offset case is supported; overlapping axes are not required.
func NewOffAxisSphereBore(R, r float64, offset Vec2, floor float64) (OffAxisSphereBore, error) {
	if !finite(R) || !finite(r) || !finite(offset.X) || !finite(offset.Y) || !finite(floor) || R <= 0 || r <= 0 || r >= R {
		return OffAxisSphereBore{}, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	out := OffAxisSphereBore{q(R), q(r), q(offset.X), q(offset.Y), q(floor)}
	d2 := new(big.Rat).Add(sq(out.cx), sq(out.cy))
	gap := new(big.Rat).Sub(out.R, out.r)
	if d2.Cmp(sq(gap)) >= 0 {
		return OffAxisSphereBore{}, ErrDomain
	}
	a := new(big.Rat).Sub(new(big.Rat).Sub(new(big.Rat).Sub(sq(out.R), sq(out.floor)), d2), sq(out.r))
	rhs := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(sq(out.r), d2))
	if a.Sign() <= 0 || sq(a).Cmp(rhs) <= 0 {
		return OffAxisSphereBore{}, ErrDomain
	}
	return out, nil
}
func boreLocation(s, c, h int) (BorePointLocation, BoreBoundaryMask) {
	if s > 0 || c < 0 && h > 0 {
		return BoreOutside, 0
	}
	var mask BoreBoundaryMask
	if s == 0 {
		mask |= BoreSphere
	}
	if c == 0 && h >= 0 {
		mask |= BoreCylinder
	}
	if h == 0 && c <= 0 {
		mask |= BoreFloor
	}
	if mask != 0 {
		return BoreBoundary, mask
	}
	return BoreInside, 0
}

// Classify uses exact represented point coordinates. All masks at a shared
// sphere/cylinder rim or cylinder/floor ring are reported together.
func (s OffAxisSphereBore) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	if s.R == nil || !p.IsFinite() {
		return BoreOutside, 0, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	x, y, z := q(p.X), q(p.Y), q(p.Z)
	sphere := new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Add(sq(x), sq(y)), sq(z)), sq(s.R))
	cyl := new(big.Rat).Sub(new(big.Rat).Add(sq(new(big.Rat).Sub(x, s.cx)), sq(new(big.Rat).Sub(y, s.cy))), sq(s.r))
	h := new(big.Rat).Sub(z, s.floor)
	loc, mask := boreLocation(sphere.Sign(), cyl.Sign(), h.Sign())
	return loc, mask, nil
}

// BoreLineHit.Multiplicity is the maximum support-polynomial multiplicity
// among the merged boundary events, not a crossing-parity classification.
type BoreLineHit struct {
	Parameter    *ExactAlgebraicRoot
	Bounds       ExactParameterInterval
	Multiplicity int
	Boundary     BoreBoundaryMask
}

// IntersectLine returns every isolated boundary event on the closed original-t
// interval. Root signs filter hidden sphere/cylinder pieces exactly. Tangencies
// retain multiplicity; coincident sphere/cylinder/floor events merge by exact
// root equality. Lines lying identically on the floor or cylindrical wall are
// explicitly unresolved (their boundary contains intervals, not isolated roots).
// Isolation/sign work shares the supplied arithmetic budget; root import and
// at most ten pair comparisons each receive the same bounded options.
func (s OffAxisSphereBore) IntersectLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) ([]BoreLineHit, error) {
	if ctx == nil || s.R == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return nil, ErrInvalid
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return nil, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	rootOpts := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	squared := func(v, d float64, center *big.Rat) (rationalPolynomial, error) {
		line := rationalPolynomial{new(big.Rat).Sub(q(v), center), q(d)}
		return polynomialProduct(line, line, b)
	}
	var poly [3]rationalPolynomial
	for i := 0; i < 2; i++ {
		poly[i] = rationalPolynomial{new(big.Rat), new(big.Rat), new(big.Rat)}
	}
	ov, dv := [3]float64{origin.X, origin.Y, origin.Z}, [3]float64{direction.X, direction.Y, direction.Z}
	for i := 0; i < 3; i++ {
		term, e := squared(ov[i], dv[i], new(big.Rat))
		if e != nil {
			return nil, e
		}
		for j, x := range term {
			poly[0][j].Add(poly[0][j], x)
		}
	}
	poly[0][0].Sub(poly[0][0], new(big.Rat).Mul(s.R, s.R))
	for i, c := range []*big.Rat{s.cx, s.cy} {
		term, e := squared(ov[i], dv[i], c)
		if e != nil {
			return nil, e
		}
		for j, x := range term {
			poly[1][j].Add(poly[1][j], x)
		}
	}
	poly[1][0].Sub(poly[1][0], new(big.Rat).Mul(s.r, s.r))
	poly[2] = rationalPolynomial{new(big.Rat).Sub(q(origin.Z), s.floor), q(direction.Z)}
	for i := range poly {
		poly[i] = trimPolynomial(poly[i])
		if len(poly[i]) == 0 {
			return nil, ErrConvergence
		}
		for _, x := range poly[i] {
			if e := b.check(x); e != nil {
				return nil, e
			}
		}
	}
	var hits []BoreLineHit
	for _, p := range poly {
		if len(p) == 1 {
			continue
		}
		coeff := make([]string, len(p))
		for i, x := range p {
			coeff[i] = x.RatString()
		}
		_, e := isolatePolynomialRoots(p, q(minT), q(maxT), b, o.MaxSubdivisions, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if new(big.Rat).Sub(hi, lo).Cmp(q(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: coeff, Lower: lo.RatString(), Upper: hi.RatString()}, ExactRootCodecOptions{ExactCurvePlaneEventOptions: rootOpts})
			if e != nil {
				return false, e
			}
			var signs [3]int
			for i, test := range poly {
				signs[i], _, e = composedPolynomialRootSign(ctx, root.root, test, b, o.MaxSubdivisions)
				if e != nil {
					return false, e
				}
			}
			loc, mask := boreLocation(signs[0], signs[1], signs[2])
			if loc == BoreBoundary {
				hits = append(hits, BoreLineHit{root, root.Interval(), m, mask})
			}
			return true, nil
		})
		if e != nil {
			return nil, e
		}
	}
	// Insertion sorting merges simultaneous algebraic parameters without ever
	// treating overlapping numerical brackets as equality.
	var out []BoreLineHit
	for _, hit := range hits {
		insert := len(out)
		merged := false
		for j, old := range out {
			order, e := hit.Parameter.Compare(ctx, old.Parameter, rootOpts)
			if e != nil {
				return nil, e
			}
			if !order.Proven {
				return nil, ErrConvergence
			}
			if order.Order == 0 {
				out[j].Boundary |= hit.Boundary
				if hit.Multiplicity > out[j].Multiplicity {
					out[j].Multiplicity = hit.Multiplicity
				}
				merged = true
				break
			}
			if order.Order < 0 {
				insert = j
				break
			}
		}
		if !merged {
			out = append(out, BoreLineHit{})
			copy(out[insert+1:], out[insert:])
			out[insert] = hit
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
