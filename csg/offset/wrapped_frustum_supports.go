package offset

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// FrustumSupportInsets independently moves the lateral support by Euclidean
// normal distance and the two caps by axial distance, all inward/nonnegative.
type FrustumSupportInsets struct{ Lateral, LowerCap, UpperCap float64 }

// FrustumSupportInsetHistory records original support ownership and the exact
// intersections of moved caps with the moved lateral support. Arrays use lower,
// upper order. Returned rationals and slices are owned by the result.
type FrustumSupportInsetHistory struct {
	SourceFaces         [][]int
	Lateral             int
	Caps                [2]int
	RimEdges            [2][]int
	Heights, Radii      [2]*big.Rat
	Slope, NormalLength *big.Rat
}

// InsetWrappedCircularFrustumSupports intersects independently inset supporting
// halfspaces of an existing canonical circular frustum. The complete source
// solid is revalidated, including cap/lateral incidence. Unequal distances are
// selective support edits, not a constant-distance erosion of the whole body.
//
// Exact acceptance requires rational sqrt(1+slope²), positive remaining height
// and end radii, and representable native vertices. Original circular weights
// and U basis are retained by exact draft/reheight operations. Source proof,
// each of three support edits, construction and result proof each have their
// own SurfacePatchOptions budget; there is no unbounded search or partial result.
func InsetWrappedCircularFrustumSupports(ctx context.Context, source trimmed.Solid, in FrustumSupportInsets, o geom.SurfacePatchOptions) (trimmed.Solid, FrustumSupportInsetHistory, error) {
	fail := func(e error) (trimmed.Solid, FrustumSupportInsetHistory, error) {
		return trimmed.Solid{}, FrustumSupportInsetHistory{}, e
	}
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	d := [3]*big.Rat{new(big.Rat).SetFloat64(in.Lateral), new(big.Rat).SetFloat64(in.LowerCap), new(big.Rat).SetFloat64(in.UpperCap)}
	for _, x := range d {
		if x == nil || x.Sign() < 0 {
			return fail(geom.ErrInvalid)
		}
	}
	w, p, e := source.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return fail(e)
	}
	if !p.Circular {
		return fail(geom.ErrConvergence)
	}
	slope := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	squared := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(slope, slope))
	n, den := new(big.Int).Sqrt(squared.Num()), new(big.Int).Sqrt(squared.Denom())
	if new(big.Int).Mul(n, n).Cmp(squared.Num()) != 0 || new(big.Int).Mul(den, den).Cmp(squared.Denom()) != 0 {
		return fail(geom.ErrUnrepresentable)
	}
	normal := new(big.Rat).SetFrac(n, den)
	heights := [2]*big.Rat{new(big.Rat).Add(p.LowerZ, d[1]), new(big.Rat).Sub(p.UpperZ, d[2])}
	if heights[0].Cmp(heights[1]) >= 0 {
		return fail(geom.ErrDomain)
	}
	move := new(big.Rat).Mul(d[0], normal)
	radii := [2]*big.Rat{}
	for i, z := range heights {
		radii[i] = new(big.Rat).Sub(new(big.Rat).Add(p.LowerRadius, new(big.Rat).Mul(slope, new(big.Rat).Sub(z, p.LowerZ))), move)
		if radii[i].Sign() <= 0 {
			return fail(geom.ErrDomain)
		}
	}
	// Preserve the exact circumferential basis while authoring new endpoint rows.
	cylinder, _, e := geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat), new(big.Rat).Sub(p.LowerRadius, p.UpperRadius), o)
	if e != nil {
		return fail(e)
	}
	cylinder, e = geom.ReheightWrappedCylinder(ctx, cylinder, heights[0], heights[1], o)
	if e != nil {
		return fail(e)
	}
	result, _, e := geom.DraftWrappedFrustumExact(ctx, cylinder, new(big.Rat).Sub(radii[0], p.LowerRadius), new(big.Rat).Sub(radii[1], p.LowerRadius), o)
	if e != nil {
		return fail(e)
	}
	out, _, e := trimmed.NewWrappedFrustum(ctx, result, source.Tolerance(), o)
	if e != nil {
		return fail(e)
	}
	rw, rp, e := out.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return fail(e)
	}
	if rp.LowerRadius.Cmp(radii[0]) != 0 || rp.UpperRadius.Cmp(radii[1]) != 0 || rp.LowerZ.Cmp(heights[0]) != 0 || rp.UpperZ.Cmp(heights[1]) != 0 {
		return fail(geom.ErrConvergence)
	}
	roles := func(s trimmed.Solid, wall *geom.WrappedSurfacePatch, z [2]*big.Rat) (int, [2]int) {
		lateral, caps := -1, [2]int{-1, -1}
		for i, f := range s.Faces() {
			switch chart := f.Surface.(type) {
			case *geom.WrappedSurfacePatch:
				if chart == wall {
					lateral = i
				}
			case geom.PlaneSurface:
				height := new(big.Rat).SetFloat64(chart.Frame().Origin().Z)
				for j := range z {
					if height.Cmp(z[j]) == 0 {
						caps[j] = i
					}
				}
			}
		}
		return lateral, caps
	}
	oldWall, oldCaps := roles(source, w, [2]*big.Rat{p.LowerZ, p.UpperZ})
	wall, caps := roles(out, rw, heights)
	if oldWall < 0 || oldCaps[0] < 0 || oldCaps[1] < 0 || wall < 0 || caps[0] < 0 || caps[1] < 0 {
		return fail(geom.ErrConvergence)
	}
	h := FrustumSupportInsetHistory{SourceFaces: make([][]int, len(out.Faces())), Lateral: wall, Caps: caps, Heights: heights, Radii: radii, Slope: slope, NormalLength: normal}
	h.SourceFaces[wall] = []int{oldWall}
	faces := out.Faces()
	for i, cap := range caps {
		h.SourceFaces[cap] = []int{oldCaps[i]}
		seen := map[int]bool{}
		for _, a := range faces[cap].Outer {
			for _, b := range faces[wall].Outer {
				if a.Edge == b.Edge {
					if seen[a.Edge] || (a.Reversed != faces[cap].Reversed) == (b.Reversed != faces[wall].Reversed) {
						return fail(geom.ErrConvergence)
					}
					seen[a.Edge] = true
					h.RimEdges[i] = append(h.RimEdges[i], a.Edge)
				}
			}
		}
		if len(h.RimEdges[i]) == 0 {
			return fail(geom.ErrConvergence)
		}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return out, h, nil
}
