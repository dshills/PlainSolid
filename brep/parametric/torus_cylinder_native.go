package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// TorusCylinderSet is a native periodic closed boundary plus its immutable exact
// original material query. This first native slice supports Common/Difference.
// Rims retain algebraic heights; numerical Eval/sewing remains tolerance-based.
type TorusCylinderSet struct {
	solid                 Solid
	query                 geom.TorusCylinderMaterial
	rims                  [2]*geom.ExactAlgebraicRoot
	radius, heightSquared *big.Rat
}

func (s TorusCylinderSet) Solid() Solid                      { return s.solid }
func (s TorusCylinderSet) Query() geom.TorusCylinderMaterial { return s.query }

// RimProof returns owned radial/height data and immutable exact lower/upper roots.
func (s TorusCylinderSet) RimProof() (*big.Rat, *big.Rat, [2]*geom.ExactAlgebraicRoot, error) {
	if s.radius == nil {
		return nil, nil, [2]*geom.ExactAlgebraicRoot{}, geom.ErrInvalid
	}
	return new(big.Rat).Set(s.radius), new(big.Rat).Set(s.heightSquared), s.rims, nil
}

// RimEdges identifies the lower and upper original common circles. The other
// two edges are distinct periodic chart seams, not additional material rims.
func (s TorusCylinderSet) RimEdges() ([2]int, error) {
	if s.radius == nil {
		return [2]int{}, geom.ErrInvalid
	}
	return [2]int{0, 2}, nil
}

// SourceFaces maps original torus and cylinder owners to native faces0 and1.
func (s TorusCylinderSet) SourceFaces() ([2][]int, error) {
	if s.radius == nil {
		return [2][]int{}, geom.ErrInvalid
	}
	return [2][]int{{0}, {1}}, nil
}

// NewTorusCylinderSet constructs a connected genus-one solid in canonical
// coordinates. Exact original inequalities and algebraic rim roots are proved
// before native topology. Two periodic annuli have opposite rim incidence and
// distinct per-face seam edges: V2/E4/F2, Euler0. The meridian is a simple
// circular arc closed by its chord, at strictly positive radius; revolution is
// embedded. No rounded angle is used to author the trim. Native numerical
// evaluators share endpoint representatives, distinct from the exact rim proof.
func NewTorusCylinderSet(ctx context.Context, R, r, c, lower, upper float64, op geom.TorusCylinderOperation, tolerance float64) (TorusCylinderSet, error) {
	var z TorusCylinderSet
	if ctx == nil || !finite(tolerance) || tolerance <= 0 || (op != geom.TorusCylinderCommon && op != geom.TorusCylinderDifference) {
		return z, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return z, e
	}
	query, e := geom.NewTorusCylinderMaterial(R, r, c, lower, upper, op)
	if e != nil {
		return z, e
	}
	torus, e := newTorusCylinderChart(R, r, c, op == geom.TorusCylinderDifference, false)
	if e != nil {
		return z, e
	}
	wall := torus
	wall.wall = true
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	if torus.heightSquared.Cmp(new(big.Rat).Mul(pad, pad)) <= 0 || new(big.Rat).Sub(q(R), q(r)).Cmp(pad) <= 0 || new(big.Rat).Sub(q(c), new(big.Rat).Sub(q(R), q(r))).Cmp(pad) <= 0 || new(big.Rat).Sub(new(big.Rat).Add(q(R), q(r)), q(c)).Cmp(pad) <= 0 {
		return z, geom.ErrConvergence
	}
	roots, e := query.ClipRationalLine(ctx, [3]*big.Rat{q(c), new(big.Rat), new(big.Rat)}, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, q(-r), q(r), geom.ToroidLineOptions{ParameterTolerance: tolerance})
	if e != nil {
		return z, e
	}
	if len(roots.Intervals) != 1 || roots.Intervals[0].Boundary != geom.TorusCylinderWall || roots.Intervals[0].StartBoundary != 3 || roots.Intervals[0].EndBoundary != 3 {
		return z, geom.ErrConvergence
	}
	lo, e := torus.Eval(0, -1)
	if e != nil {
		return z, e
	}
	hi, e := torus.Eval(0, 1)
	if e != nil {
		return z, e
	}
	if lo == hi {
		return z, geom.ErrConvergence
	}
	solid, e := build([]geom.Vec3{lo, hi}, R+r,
		patch{face: Face{Surface: torus, U: circleDomain, V: Interval{-1, 1}, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}, Reversed: !torus.outer}, corners: [4]int{0, 0, 1, 1}},
		patch{face: Face{Surface: wall, U: circleDomain, V: Interval{-1, 1}, Boundary: [4]Use{{0, false}, {3, false}, {2, false}, {3, true}}, Reversed: torus.outer}, corners: [4]int{0, 0, 1, 1}})
	if e != nil {
		return z, e
	}
	solid, e = NewSolid(solid.vertices, solid.edges, solid.faces, tolerance)
	if e != nil {
		return z, e
	}
	if e := ctx.Err(); e != nil {
		return z, e
	}
	return TorusCylinderSet{solid, query, [2]*geom.ExactAlgebraicRoot{roots.Intervals[0].Start, roots.Intervals[0].End}, q(c), new(big.Rat).Set(torus.heightSquared)}, nil
}

// Original coefficients are retained; derived floating values are evaluation
// caches only. With sigma=+1 outer/-1 inner and exact K>0,
// rho=R+sigma*r*(1-Kv²)/(1+Kv²), z=2r*sqrt(K)*v/(1+Kv²).
// K=(r-sigma*(c-R))/(r+sigma*(c-R)). This lies exactly on the
// original torus. At v=±1 the identities give rho=c,z=±sqrt(h²).
type torusCylinderChart struct {
	R, r, c       float64
	outer, wall   bool
	k, h          float64
	heightSquared *big.Rat
}

func newTorusCylinderChart(R, r, c float64, outer, wall bool) (torusCylinderChart, error) {
	var z torusCylinderChart
	if !finite(R) || !finite(r) || !finite(c) || R <= r || r <= 0 || c <= 0 {
		return z, geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	delta := new(big.Rat).Sub(q(c), q(R))
	rr := q(r)
	if new(big.Rat).Abs(delta).Cmp(rr) >= 0 {
		return z, geom.ErrDomain
	}
	h2 := new(big.Rat).Sub(new(big.Rat).Mul(rr, rr), new(big.Rat).Mul(delta, delta))
	if !outer {
		delta.Neg(delta)
	}
	K := new(big.Rat).Quo(new(big.Rat).Sub(rr, delta), new(big.Rat).Add(rr, delta))
	k, _ := K.Float64()
	hf, _ := h2.Float64()
	h := math.Sqrt(hf)
	if !finite(k) || k <= 0 || !finite(h) || h <= 0 {
		return z, geom.ErrConvergence
	}
	return torusCylinderChart{R, r, c, outer, wall, k, h, h2}, nil
}
func (s torusCylinderChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s torusCylinderChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, du, dv, e := s.evaluate(u, v)
	return du, dv, e
}
func (s torusCylinderChart) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !finite(u) || !finite(v) || v < -1 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	rho, z, dr, dz := s.c, s.h*v, 0.0, s.h
	if !s.wall {
		sigma := 1.0
		if !s.outer {
			sigma = -1
		}
		kv2 := s.k * v * v
		den := 1 + kv2
		root := math.Sqrt(s.k)
		rho = s.R + sigma*s.r*(1-kv2)/den
		z = 2 * s.r * root * v / den
		dr = -sigma * 4 * s.r * s.k * v / (den * den)
		dz = 2 * s.r * root * (1 - kv2) / (den * den)
		if v == -1 || v == 1 {
			rho = s.c
			z = s.h * v
		}
	}
	sn, cs := math.Sincos(u)
	p := geom.Vec3{X: rho * cs, Y: rho * sn, Z: z}
	du := geom.Vec3{X: -rho * sn, Y: rho * cs}
	dv := geom.Vec3{X: dr * cs, Y: dr * sn, Z: dz}
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}
func (s torusCylinderChart) secondBounds() (float64, float64, float64, bool) {
	if s.wall {
		return s.c, 0, 0, true
	}
	k := math.Nextafter(s.k, math.Inf(1))
	root := math.Nextafter(math.Sqrt(k), math.Inf(1))
	uu := upAdd(s.R, s.r)
	uv := upMul(2, upMul(s.r, root))
	vv := upMul(4, upMul(s.r, upAdd(k, upMul(k, root))))
	return uu, uv, vv, finite(uu) && finite(uv) && finite(vv)
}
