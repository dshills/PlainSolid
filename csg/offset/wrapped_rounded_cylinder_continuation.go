package offset

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RoundedCylinderSharpRim identifies the new analytic cap/lateral contact.
// SourceBlend is an extinct face, not an ancestor of either surviving face.
// Radius and Z are owned exact coordinates in the canonical cylinder frame.
// Edges follows the result cap traversal; End is 0 for lower or 1 for upper.
type RoundedCylinderSharpRim struct {
	End                                   int
	Edges                                 []int
	SourceLateral, SourceCap, SourceBlend int
	Radius, Z                             *big.Rat
}

// RoundedCylinderContinuationHistory preserves original face ancestry and the
// sharp rim created after the smaller original blend disappears.
type RoundedCylinderContinuationHistory struct {
	Faces    WrappedRoundedCylinderHistory
	SharpRim RoundedCylinderSharpRim
}

// ErodeWrappedRoundedCylinderPastBlend accepts a canonical convex cylinder with
// two unequal positive rim blends and min(radii) < depth < max(radii). It
// continues inward beyond the first blend's extinction, retaining the opposite
// toroidal support. It does not extrapolate a negative blend radius.
//
// The first stage is exact blend extinction. The second erodes that convex
// sharp/rounded core. Erosion by balls satisfies (K⊖B_a)⊖B_b=K⊖B_(a+b).
// At the vanished corner, shifted cap/lateral constraints imply all intermediate
// normal constraints because cos(theta)+sin(theta)>=1 on [0,pi/2]. Thus the
// sharp continuation is the true Euclidean erosion, not a rounded approximation.
// Both stages must be exactly representable; failures return no partial solid
// or history. SurfacePatchOptions limits apply independently to source proof
// and each of the two bounded offset stages; there is no iterative search.
func ErodeWrappedRoundedCylinderPastBlend(ctx context.Context, solid trimmed.Solid, depth float64, o geom.SurfacePatchOptions) (trimmed.Solid, RoundedCylinderContinuationHistory, error) {
	fail := func(e error) (trimmed.Solid, RoundedCylinderContinuationHistory, error) {
		return trimmed.Solid{}, RoundedCylinderContinuationHistory{}, e
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
	end := 0
	if p.Fillets[1].Cmp(p.Fillets[0]) < 0 {
		end = 1
	}
	small, large := p.Fillets[end], p.Fillets[1-end]
	if small.Sign() <= 0 || t.Cmp(small) <= 0 || t.Cmp(large) >= 0 {
		return fail(geom.ErrDomain)
	}
	first, e := geom.RepresentedExactScalar(new(big.Rat).Neg(small))
	if e != nil {
		return fail(e)
	}
	remaining, e := geom.RepresentedExactScalar(new(big.Rat).Sub(small, t))
	if e != nil {
		return fail(e)
	}
	core, h1, e := WrappedRoundedCylinder(ctx, solid, first, o)
	if e != nil {
		return fail(fmt.Errorf("extinction: %w", e))
	}
	out, h2, e := WrappedRoundedCylinder(ctx, core, remaining, o)
	if e != nil {
		return fail(fmt.Errorf("continuation: %w", e))
	}
	// Each retained support has exactly one original support in this family.
	for i, owners := range h2.SourceFaces {
		if len(owners) != 1 || owners[0] < 0 || owners[0] >= len(h1.SourceFaces) || len(h1.SourceFaces[owners[0]]) != 1 {
			return fail(fmt.Errorf("ancestry: %w", geom.ErrConvergence))
		}
		h2.SourceFaces[i] = []int{h1.SourceFaces[owners[0]][0]}
	}
	faces := out.Faces()
	edges := []int{}
	seen := make(map[int]bool)
	for _, a := range faces[h2.ResultCaps[end]].Outer {
		for _, b := range faces[h2.ResultLateral].Outer {
			if a.Edge == b.Edge {
				if seen[a.Edge] || (a.Reversed != faces[h2.ResultCaps[end]].Reversed) == (b.Reversed != faces[h2.ResultLateral].Reversed) {
					return fail(fmt.Errorf("rim duplicate/orientation edge=%d a=%v b=%v: %w", a.Edge, a.Reversed, b.Reversed, geom.ErrConvergence))
				}
				edges = append(edges, a.Edge)
				seen[a.Edge] = true
			}
		}
	}
	if len(edges) == 0 || h2.ResultBlends[end] != -1 || h2.ResultBlends[1-end] < 0 {
		return fail(fmt.Errorf("rim absent edge=%d blends=%v: %w", len(edges), h2.ResultBlends, geom.ErrConvergence))
	}
	z := new(big.Rat).Add(p.LowerZ, t)
	if end == 1 {
		z.Sub(p.UpperZ, t)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return out, RoundedCylinderContinuationHistory{Faces: h2, SharpRim: RoundedCylinderSharpRim{
		End: end, Edges: edges, SourceLateral: p.Lateral, SourceCap: p.Caps[end], SourceBlend: p.Blends[end],
		Radius: new(big.Rat).Sub(p.Radius, t), Z: z,
	}}, nil
}
