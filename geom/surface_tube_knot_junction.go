package geom

import (
	"context"
	"math/big"
)

// SurfaceKnotJunctionOptions supplies two one-sided rectangles around an
// internal knot in the chart owning FreeAxis. The left free interval ends at
// the knot and the right begins there. Anchor uses original A_U,A_V,B_U,B_V.
type SurfaceKnotJunctionOptions struct {
	SurfacePairTubeOptions
	Left, Right SurfacePairParameterBox
	Anchor      [4]*big.Rat
}

// SurfaceKnotJunction retains independently proved branches meeting at one exact
// source parameter point. It permits derivative jumps: it asserts continuity
// and two regular one-sided germs, not a smooth tangent across the knot.
type SurfaceKnotJunction struct {
	left, right VerifiedSurfaceTube
	anchor      [4]*big.Rat
	point       [3]*big.Rat
	operations  int
}

func (j *SurfaceKnotJunction) Tubes() (VerifiedSurfaceTube, VerifiedSurfaceTube, error) {
	if j == nil || j.left.a == nil {
		return VerifiedSurfaceTube{}, VerifiedSurfaceTube{}, ErrInvalid
	}
	return j.left, j.right, nil
}
func (j *SurfaceKnotJunction) Anchor() ([4]*big.Rat, [3]*big.Rat, error) {
	var q [4]*big.Rat
	var p [3]*big.Rat
	if j == nil || j.left.a == nil {
		return q, p, ErrInvalid
	}
	for i := range q {
		q[i] = new(big.Rat).Set(j.anchor[i])
	}
	for i := range p {
		p[i] = new(big.Rat).Set(j.point[i])
	}
	return q, p, nil
}
func (j *SurfaceKnotJunction) ExactOperations() int {
	if j == nil {
		return 0
	}
	return j.operations
}

// CertifySurfaceKnotJunction constructs both branches from original immutable
// supports and verifies that the free split is an actual continuous internal
// spline knot. Each local rectangle must extract to one Bezier cell. No rounded
// endpoint matching or cross-knot derivative bound is used. Other branches
// elsewhere in the source pair are outside this local certificate.
func CertifySurfaceKnotJunction(ctx context.Context, a, b *SurfacePatch, o SurfaceKnotJunctionOptions) (*SurfaceKnotJunction, error) {
	if ctx == nil || a == nil || b == nil || o.FreeAxis < 0 || o.FreeAxis > 3 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, ErrInvalid
	}
	left, right := pairParameterRanges(o.Left), pairParameterRanges(o.Right)
	knot := left[o.FreeAxis][1]
	if !finite(knot) || knot != right[o.FreeAxis][0] {
		return nil, ErrDomain
	}
	owner := a
	if o.FreeAxis >= 2 {
		owner = b
	}
	knots, degree := owner.knotsU, owner.degreeU
	if o.FreeAxis%2 == 1 {
		knots, degree = owner.knotsV, owner.degreeV
	}
	if knot <= knots[degree] || knot >= knots[len(knots)-degree-1] {
		return nil, ErrDomain
	}
	multiplicity := 0
	for _, k := range knots {
		if k == knot {
			multiplicity++
		}
	}
	if multiplicity < 1 || multiplicity > degree {
		return nil, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	result := &SurfaceKnotJunction{}
	for side, box := range []SurfacePairParameterBox{o.Left, o.Right} {
		pa, e := trimPatchBudget(a, box.A, budget)
		if e != nil {
			return nil, e
		}
		pb, e := trimPatchBudget(b, box.B, budget)
		if e != nil {
			return nil, e
		}
		child := SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: o.SurfacePairTubeOptions, AtUpper: side == 0, Anchor: o.Anchor}
		child.MaxOperations -= budget.operations
		if child.MaxOperations < 1 {
			return nil, ErrConvergence
		}
		used := 0
		tube, e := newBoundaryAnchoredSurfaceTube(ctx, pa, pb, child, &used)
		budget.operations += used
		if e != nil {
			return nil, e
		}
		if side == 0 {
			result.left = tube
		} else {
			result.right = tube
		}
	}
	for i := range result.anchor {
		if result.left.anchor.parameters[i].Cmp(result.right.anchor.parameters[i]) != 0 {
			return nil, ErrConvergence
		}
		result.anchor[i] = new(big.Rat).Set(result.left.anchor.parameters[i])
	}
	for i := range result.point {
		if result.left.anchor.point[i].Cmp(result.right.anchor.point[i]) != 0 {
			return nil, ErrConvergence
		}
		result.point[i] = new(big.Rat).Set(result.left.anchor.point[i])
	}
	result.operations = budget.operations
	return result, nil
}

// ExactBoundaryAnchor returns owned exact coordinates of an accepted anchor.
// HasAnchor=false is a uniform interior tube, not an unresolved endpoint. The
// returned rational data is evidence derived from the immutable proof; edits
// to these copies cannot change later membership decisions.
func (t VerifiedSurfaceTube) ExactBoundaryAnchor() (parameters [4]*big.Rat, point [3]*big.Rat, freeAxis int, atUpper, hasAnchor bool) {
	if t.anchor == nil {
		return parameters, point, 0, false, false
	}
	for i := range parameters {
		parameters[i] = new(big.Rat).Set(t.anchor.parameters[i])
	}
	for i := range point {
		point[i] = new(big.Rat).Set(t.anchor.point[i])
	}
	ranges := pairParameterRanges(SurfacePairParameterBox{t.a.Domain(), t.b.Domain()})
	return parameters, point, t.axis, t.anchor.free == ranges[t.axis][1], true
}
