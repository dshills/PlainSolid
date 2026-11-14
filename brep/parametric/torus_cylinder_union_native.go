package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// TorusCylinderUnionSet is the connected closed union in the strict transverse
// family of NewTorusCylinderMaterial. Its two original cylinder caps are clear
// of the torus. Algebraic rim identities are independent of numerical Eval.
type TorusCylinderUnionSet struct{ base TorusCylinderSet }

func (s TorusCylinderUnionSet) Solid() Solid                      { return s.base.Solid() }
func (s TorusCylinderUnionSet) Query() geom.TorusCylinderMaterial { return s.base.Query() }
func (s TorusCylinderUnionSet) RimProof() (*big.Rat, *big.Rat, [2]*geom.ExactAlgebraicRoot, error) {
	return s.base.RimProof()
}
func (s TorusCylinderUnionSet) RimEdges() ([2]int, error) { return s.base.RimEdges() }

// SourceFaces retains torus, cylinder wall, lower cap, upper cap ownership.
func (s TorusCylinderUnionSet) SourceFaces() ([4][]int, error) {
	if s.base.radius == nil {
		return [4][]int{}, geom.ErrInvalid
	}
	return [4][]int{{0}, {1, 2}, {3}, {4}}, nil
}

// NewTorusCylinderUnionSet builds the original outer torus band, two exposed
// cylinder bands, and both planar disks. All adjacent bands share the exact
// original circular rims. The meridian closes on the axis through the disks,
// giving one embedded genus-zero boundary; the periodic seams are chart seams.
// Like NewTorusCylinderSet, numerical native validation and Eval are separate
// from the exact original support and algebraic rim proof.
func NewTorusCylinderUnionSet(ctx context.Context, R, r, c, lower, upper, tolerance float64) (TorusCylinderUnionSet, error) {
	var zero TorusCylinderUnionSet
	base, e := NewTorusCylinderSet(ctx, R, r, c, lower, upper, geom.TorusCylinderDifference, tolerance)
	if e != nil {
		return zero, e
	}
	query, e := geom.NewTorusCylinderMaterial(R, r, c, lower, upper, geom.TorusCylinderUnion)
	if e != nil {
		return zero, e
	}
	low, e := newTorusCylinderOuterWall(R, r, c, lower, true)
	if e != nil {
		return zero, e
	}
	high, e := newTorusCylinderOuterWall(R, r, c, upper, false)
	if e != nil {
		return zero, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	if new(big.Rat).Sub(q(-r), q(lower)).Cmp(pad) <= 0 || new(big.Rat).Sub(q(upper), q(r)).Cmp(pad) <= 0 {
		return zero, geom.ErrConvergence
	}
	// Use the canonical +X radial frame, shared with the cylinder chart.
	bottom, e := geom.NewFrame(geom.Vec3{Z: lower}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return zero, e
	}
	top, e := geom.NewFrame(geom.Vec3{Z: upper}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return zero, e
	}
	torus := low.source
	vertices := []geom.Vec3{{X: c, Z: -torus.h}, {X: c, Z: torus.h}, {X: c, Z: lower}, {X: c, Z: upper}, {Z: lower}, {Z: upper}}
	solid, e := build(vertices, math.Max(R+r, math.Max(math.Abs(lower), math.Abs(upper))),
		patch{face: Face{Surface: torus, U: circleDomain, V: Interval{-1, 1}, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}}, corners: [4]int{0, 0, 1, 1}},
		patch{face: Face{Surface: low, U: circleDomain, V: normalized, Boundary: [4]Use{{3, false}, {4, false}, {0, true}, {4, true}}}, corners: [4]int{2, 2, 0, 0}},
		patch{face: Face{Surface: high, U: circleDomain, V: normalized, Boundary: [4]Use{{2, true}, {5, false}, {6, false}, {5, true}}}, corners: [4]int{1, 1, 3, 3}},
		patch{face: Face{Surface: disk{bottom}, U: circleDomain, V: Interval{0, c}, Boundary: [4]Use{{7, false}, {8, false}, {3, true}, {8, true}}}, corners: [4]int{4, 4, 2, 2}, collapsed: [4]bool{true, false, false, false}},
		patch{face: Face{Surface: disk{top}, U: circleDomain, V: Interval{0, c}, Boundary: [4]Use{{9, false}, {10, false}, {6, false}, {10, true}}, Reversed: true}, corners: [4]int{5, 5, 3, 3}, collapsed: [4]bool{true, false, false, false}})
	if e != nil {
		return zero, e
	}
	solid, e = NewSolid(solid.vertices, solid.edges, solid.faces, tolerance)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	base.solid, base.query = solid, query
	return TorusCylinderUnionSet{base}, nil
}

// Original radii and cap are the persistent coefficients. The exact height is
// sqrt(source.heightSquared), not the derived floating evaluation cache.
type torusCylinderOuterWall struct {
	source torusCylinderChart
	cap    float64
	lower  bool
}

func newTorusCylinderOuterWall(R, r, c, cap float64, lower bool) (torusCylinderOuterWall, error) {
	s, e := newTorusCylinderChart(R, r, c, true, false)
	if e != nil {
		return torusCylinderOuterWall{}, e
	}
	if !finite(cap) || (lower && cap >= -r) || (!lower && cap <= r) {
		return torusCylinderOuterWall{}, geom.ErrDomain
	}
	return torusCylinderOuterWall{s, cap, lower}, nil
}
func (s torusCylinderOuterWall) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s torusCylinderOuterWall) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s torusCylinderOuterWall) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !finite(u) || !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	a, b := s.source.h, s.cap
	if s.lower {
		a, b = s.cap, -s.source.h
	}
	sn, cs := math.Sincos(u)
	p := geom.Vec3{X: s.source.c * cs, Y: s.source.c * sn, Z: (1-v)*a + v*b}
	du, dv := geom.Vec3{X: -s.source.c * sn, Y: s.source.c * cs}, geom.Vec3{Z: b - a}
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}
