package parametric

import (
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
)

type SphereFiniteOffsetUnionKind uint8

const (
	SphereOffsetUnionDisjoint SphereFiniteOffsetUnionKind = iota
	SphereOffsetUnionSphere
	SphereOffsetUnionCylinder
	SphereOffsetUnionNorth
	SphereOffsetUnionSouth
	SphereOffsetUnionBoth
)

type SphereFiniteOffsetUnion struct {
	Solids []Solid
	Kind   SphereFiniteOffsetUnionKind
	// Face ancestry is per body, in the same order as Solids.
	SphericalFaces, CylindricalFaces, LowerCapFaces, UpperCapFaces [][]int
	// Nil for transverse cases without a rational-times-pi ideal volume.
	VolumePiCoefficients []*big.Rat
}

// UnionFiniteOffsetCylinder dispatches strictly separated or contained bodies
// and one/two-ended protrusions. Tangent containment/partial cap cuts fail
// closed. All coordinates use the translated global axis-aligned model frame.
func UnionFiniteOffsetCylinder(ctx context.Context, center geom.Vec3, R, r, offset, lower, upper, tolerance float64) (SphereFiniteOffsetUnion, error) {
	fail := func(e error) (SphereFiniteOffsetUnion, error) { return SphereFiniteOffsetUnion{}, e }
	if ctx == nil || !center.IsFinite() || !finite(offset) || offset < 0 || !finite(tolerance) || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if _, e := geom.NewOffAxisSphereFiniteCylinderCommon(R, r, geom.Vec2{X: offset}, lower, upper); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	pad := mul(big.NewRat(8, 1), q(tolerance))
	guard := new(big.Rat).Add(q(R), pad)
	single := func(s Solid, k SphereFiniteOffsetUnionKind, a, b, c, d []int, v *big.Rat) SphereFiniteOffsetUnion {
		out := SphereFiniteOffsetUnion{Solids: []Solid{s}, Kind: k, SphericalFaces: [][]int{a}, CylindricalFaces: [][]int{b}, LowerCapFaces: [][]int{c}, UpperCapFaces: [][]int{d}}
		if v != nil {
			out.VolumePiCoefficients = []*big.Rat{v}
		}
		return out
	}
	// Exact sufficient full containment with explicit positive radial/axial gap.
	if q(r).Cmp(new(big.Rat).Add(guard, q(offset))) > 0 && q(lower).Cmp(new(big.Rat).Neg(guard)) < 0 && q(upper).Cmp(guard) > 0 {
		hrat := new(big.Rat).Sub(q(upper), q(lower))
		h, e := geom.RepresentedExactScalar(hrat)
		if e != nil {
			return fail(e)
		}
		x, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(center.X), q(offset)))
		if e != nil {
			return fail(e)
		}
		z, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(center.Z), q(lower)))
		if e != nil {
			return fail(e)
		}
		frame, e := geom.NewFrame(geom.Vec3{X: x, Y: center.Y, Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return fail(e)
		}
		s, e := Cylinder(frame, r, h)
		if e != nil {
			return fail(e)
		}
		s, e = NewSolid(s.Vertices(), s.Edges(), s.Faces(), tolerance)
		if e != nil {
			return fail(e)
		}
		return single(s, SphereOffsetUnionCylinder, nil, []int{0}, []int{1}, []int{2}, mul(mul(q(r), q(r)), hrat)), nil
	}
	// Reuse the independent strict worst-cap containment proof; return only the
	// original sphere, never the cavity topology used by its proof constructor.
	_, e := SphereEnclosedCylinderCavity(ctx, center, R, r, geom.Vec2{X: offset}, lower, upper, tolerance)
	if e == nil {
		s, e := Sphere(center, R)
		if e != nil {
			return fail(e)
		}
		return single(s, SphereOffsetUnionSphere, []int{0}, nil, nil, nil, mul(big.NewRat(4, 3), mul(mul(q(R), q(R)), q(R)))), nil
	}
	if !errors.Is(e, geom.ErrDomain) {
		return fail(e)
	}
	disjoint, e := SphereDisjointFiniteCylinderUnion(ctx, center, R, r, geom.Vec2{X: offset}, lower, upper, tolerance)
	if e == nil {
		return SphereFiniteOffsetUnion{Solids: []Solid{disjoint.Solids[0], disjoint.Solids[1]}, Kind: SphereOffsetUnionDisjoint, SphericalFaces: [][]int{{0}, nil}, CylindricalFaces: [][]int{nil, {0}}, LowerCapFaces: [][]int{nil, {1}}, UpperCapFaces: [][]int{nil, {2}}, VolumePiCoefficients: []*big.Rat{disjoint.VolumePiCoefficients[0], disjoint.VolumePiCoefficients[1]}}, nil
	}
	if !errors.Is(e, geom.ErrDomain) {
		return fail(e)
	}
	north := q(upper).Cmp(guard) > 0
	south := q(lower).Cmp(new(big.Rat).Neg(guard)) < 0
	frame, e := geom.NewFrame(center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	reverse, e := geom.NewFrame(center, geom.Vec3{X: 1}, geom.Vec3{Z: -1})
	if e != nil {
		return fail(e)
	}
	if north && south {
		// Outward spherical annuli come directly from the established through
		// construction. Only the inward cylindrical wall is omitted.
		base, e := SphereOffAxisThroughBore(ctx, frame, R, r, offset, tolerance)
		if e != nil {
			return fail(e)
		}
		var faces []Face
		var sphereIDs, wallIDs, lowerIDs, upperIDs []int
		for _, id := range append(base.LowerSphericalFaces, base.UpperSphericalFaces...) {
			sphereIDs = append(sphereIDs, len(faces))
			faces = append(faces, base.Solid.faces[id])
		}
		for side, f := range []geom.Frame{frame, reverse} {
			end := upper
			if side == 1 {
				end = -lower
			}
			chart, e := newToolBoreChart(ctx, offsetBoreChart{frame: f, radius: R, bore: r, offset: offset, floor: end, kind: toolBoreUnionWall}, tolerance)
			if e != nil {
				return fail(e)
			}
			for i := 0; i < 4; i++ {
				u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
				chart.kind = toolBoreUnionWall
				wallIDs = append(wallIDs, len(faces))
				faces = append(faces, Face{Surface: chart, U: u, V: Interval{0, 1}})
				chart.kind = toolBoreUnionCap
				if side == 0 {
					upperIDs = append(upperIDs, len(faces))
				} else {
					lowerIDs = append(lowerIDs, len(faces))
				}
				faces = append(faces, Face{Surface: chart, U: u, V: Interval{0, 1}, Reversed: true})
			}
		}
		s, e := joinPrimitivePatches(faces, tolerance)
		if e != nil {
			return fail(e)
		}
		if e = ctx.Err(); e != nil {
			return fail(e)
		}
		return single(s, SphereOffsetUnionBoth, sphereIDs, wallIDs, lowerIDs, upperIDs, nil), nil
	}
	if north {
		out, e := SphereOffAxisCylinderUnion(ctx, frame, R, r, offset, lower, upper, tolerance)
		if e != nil {
			return fail(e)
		}
		return single(out.Solid, SphereOffsetUnionNorth, out.SphericalFaces, out.CylindricalFaces, nil, out.CapFaces, nil), nil
	}
	if south {
		out, e := SphereOffAxisCylinderUnion(ctx, reverse, R, r, offset, -upper, -lower, tolerance)
		if e != nil {
			return fail(e)
		}
		return single(out.Solid, SphereOffsetUnionSouth, out.SphericalFaces, out.CylindricalFaces, out.CapFaces, nil, nil), nil
	}
	return fail(geom.ErrDomain)
}
