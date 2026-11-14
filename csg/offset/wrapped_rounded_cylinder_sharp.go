package offset

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RoundedCylinderSharpHistory records the three retained original support
// faces and both vanished blends as contact history, never as surviving faces.
type RoundedCylinderSharpHistory struct {
	Faces WrappedRoundedCylinderHistory
	Rims  [2]RoundedCylinderSharpRim
}

// ErodeWrappedRoundedCylinderToSharp returns the exact Euclidean erosion of a
// canonical cylinder with two positive rounded rims when depth is at least
// both original blend radii. Equality at the larger radius is included. The
// result must have strictly positive radius and height, with exactly represented
// coordinates. Shallower depths, originally sharp rims, and collapsed bodies
// are outside this entry point; other offset APIs keep their existing contracts.
//
// For each vanished rounded corner, write its outward meridian normal as
// (cos(theta),sin(theta)), theta in [0,pi/2]. With t>=r, the shifted cap/wall
// intersection satisfies the shifted circular support inequality because
// (t-r)*(cos(theta)+sin(theta)-1)>=0. Thus every curved constraint is redundant
// while the shifted axial and radial constraints remain necessary. Their
// intersection is precisely the returned sharp cylinder. No sampled geometry
// or tolerance is used to decide extinction. Work uses one source proof and
// construction followed by a fixed number of canonical reproofs; supplied
// limits apply to each call separately, without iterative search.
func ErodeWrappedRoundedCylinderToSharp(ctx context.Context, solid trimmed.Solid, depth float64, o geom.SurfacePatchOptions) (trimmed.Solid, RoundedCylinderSharpHistory, error) {
	fail := func(e error) (trimmed.Solid, RoundedCylinderSharpHistory, error) {
		return trimmed.Solid{}, RoundedCylinderSharpHistory{}, e
	}
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	t := new(big.Rat).SetFloat64(depth)
	if t == nil || t.Sign() <= 0 {
		return fail(geom.ErrInvalid)
	}
	p, e := solid.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil {
		return fail(e)
	}
	for _, r := range p.Fillets {
		if r.Sign() <= 0 || t.Cmp(r) < 0 {
			return fail(geom.ErrDomain)
		}
	}
	radius := new(big.Rat).Sub(p.Radius, t)
	lower, upper := new(big.Rat).Add(p.LowerZ, t), new(big.Rat).Sub(p.UpperZ, t)
	if radius.Sign() <= 0 || lower.Cmp(upper) >= 0 {
		return fail(geom.ErrDomain)
	}
	w, ok := solid.Faces()[p.Lateral].Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return fail(geom.ErrConvergence)
	}
	d := new(big.Rat).Neg(t)
	w, _, e = geom.DraftWrappedFrustumExact(ctx, w, d, d, o)
	if e != nil {
		return fail(e)
	}
	w, e = geom.ReheightWrappedCylinder(ctx, w, lower, upper, o)
	if e != nil {
		return fail(e)
	}
	out, _, e := trimmed.NewWrappedCylinder(ctx, w, solid.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	// Reprove builder ownership rather than assuming stored face positions.
	_, proof, e := out.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return fail(e)
	}
	if proof.Radius.Cmp(radius) != 0 || proof.LowerZ.Cmp(lower) != 0 || proof.UpperZ.Cmp(upper) != 0 {
		return fail(geom.ErrConvergence)
	}
	// The existing offset path already discovers canonical cap/wall identities.
	out, h, e := WrappedRoundedCylinder(ctx, out, 0, o)
	if e != nil {
		return fail(e)
	}
	faces := out.Faces()
	// Zero offset preserves IDs but does not populate canonical role fields via
	// fixed positions; WrappedRoundedCylinder discovers them from actual charts.
	h.SourceFaces = make([][]int, len(faces))
	h.SourceFaces[h.ResultLateral] = []int{p.Lateral}
	result := RoundedCylinderSharpHistory{Faces: h}
	for end, cap := range h.ResultCaps {
		h.SourceFaces[cap] = []int{p.Caps[end]}
		edges := []int{}
		seen := map[int]bool{}
		for _, a := range faces[cap].Outer {
			for _, b := range faces[h.ResultLateral].Outer {
				if a.Edge == b.Edge {
					if seen[a.Edge] || (a.Reversed != faces[cap].Reversed) == (b.Reversed != faces[h.ResultLateral].Reversed) {
						return fail(geom.ErrConvergence)
					}
					edges = append(edges, a.Edge)
					seen[a.Edge] = true
				}
			}
		}
		if len(edges) == 0 {
			return fail(geom.ErrConvergence)
		}
		z := lower
		if end == 1 {
			z = upper
		}
		result.Rims[end] = RoundedCylinderSharpRim{End: end, Edges: edges, SourceLateral: p.Lateral, SourceCap: p.Caps[end], SourceBlend: p.Blends[end], Radius: new(big.Rat).Set(radius), Z: new(big.Rat).Set(z)}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return out, result, nil
}
