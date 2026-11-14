package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// FrustumRollingCircle is owned exact meridian evidence for one original rim.
type FrustumRollingCircle struct {
	Center, Normal, Tangency [2]*big.Rat
	Radius                   *big.Rat
}
type WrappedFrustumDoubleFilletHistory struct {
	Lateral      int
	Caps, Blends [2]int
	Rims         [2][2][4]int // end, cap/side, quadrant; final result edge IDs
	SourceFaces  [][]int
	Circles      [2]FrustumRollingCircle
}

// FilletWrappedFrustumRims rolls both original conical cap rims independently.
// Both radii must be positive. Each endpoint uses the existing exact acute or
// obtuse proof appropriate to the ORIGINAL slope. Their exact axial tangencies
// must be strictly ordered, leaving a nonempty trim of the original cone.
// Original cap planes, circular bases and source face ownership are retained.
//
// Each independently proved donor contributes only its selected cap and blend.
// Its side rim is mapped to the new central cone by canonical quadrant identity,
// exact represented endpoint/range equality, and final ZERO model-edge error
// over the complete curves. No geometric tolerance merges different cone trims.
// Every endpoint/construction proof has its own supplied budget; final edge
// proof uses those limits. Touching runouts and unsupported representations fail
// atomically, without returning either independently constructed end.
func FilletWrappedFrustumRims(ctx context.Context, s Solid, lower, upper float64, o geom.SurfacePatchOptions) (Solid, WrappedFrustumDoubleFilletHistory, error) {
	fail := func(e error) (Solid, WrappedFrustumDoubleFilletHistory, error) {
		return Solid{}, WrappedFrustumDoubleFilletHistory{}, e
	}
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	w, p, e := s.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return fail(e)
	}
	direction := p.UpperRadius.Cmp(p.LowerRadius)
	if direction == 0 {
		return fail(geom.ErrDomain)
	}
	var donors [2]Solid
	var proofs [2]WrappedFrustumLowerFilletHistory
	for end, r := range [2]float64{lower, upper} {
		obtuse := (end == 0 && direction < 0) || (end == 1 && direction > 0)
		donors[end], proofs[end], e = filletWrappedFrustumEndRim(ctx, s, r, end == 1, obtuse, o)
		if e != nil {
			return fail(e)
		}
	}
	lo, hi := proofs[0].Tangency[1], proofs[1].Tangency[1]
	if lo.Cmp(hi) >= 0 {
		return fail(geom.ErrDomain)
	}
	slope := new(big.Rat).Quo(new(big.Rat).Sub(p.UpperRadius, p.LowerRadius), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
	for _, proof := range proofs {
		want := new(big.Rat).Add(p.LowerRadius, new(big.Rat).Mul(slope, new(big.Rat).Sub(proof.Tangency[1], p.LowerZ)))
		if want.Cmp(proof.Tangency[0]) != 0 {
			return fail(geom.ErrConvergence)
		}
	}
	w, _, e = geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat), new(big.Rat).Sub(p.LowerRadius, p.UpperRadius), o)
	if e != nil {
		return fail(e)
	}
	w, e = geom.ReheightWrappedCylinder(ctx, w, lo, hi, o)
	if e != nil {
		return fail(e)
	}
	w, _, e = geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat).Sub(proofs[0].Tangency[0], p.LowerRadius), new(big.Rat).Sub(proofs[1].Tangency[0], p.LowerRadius), o)
	if e != nil {
		return fail(e)
	}
	core, _, e := NewWrappedFrustum(ctx, w, s.tolerance, o)
	if e != nil {
		return fail(e)
	}
	vertices, edges := core.Vertices(), core.Edges()
	faces := []Face{cloneFace(core.faces[0])}
	h := WrappedFrustumDoubleFilletHistory{Lateral: 0, SourceFaces: [][]int{append([]int(nil), proofs[0].SourceFaces[proofs[0].Lateral]...)}}
	for end, donor := range donors {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
		proof := proofs[end]
		used := make([]bool, len(de))
		for _, id := range []int{proof.Blends[end], proof.Caps[end]} {
			for _, u := range df[id].Outer {
				used[u.Edge] = true
			}
		}
		vm, em := make([]int, len(dv)), make([]int, len(de))
		for i := range vm {
			vm[i] = -1
		}
		for i := range em {
			em[i] = -1
		}
		bind := func(a, b int) bool {
			if dv[a] != vertices[b] || vm[a] >= 0 && vm[a] != b {
				return false
			}
			vm[a] = b
			return true
		}
		for q, old := range proof.BlendRims[end][1] {
			id := 4*end + q
			if de[old].Range != edges[id].Range || !bind(de[old].Start, edges[id].Start) || !bind(de[old].End, edges[id].End) {
				return fail(geom.ErrConvergence)
			}
			em[old] = id
		}
		for old, yes := range used {
			if !yes {
				continue
			}
			for _, v := range []int{de[old].Start, de[old].End} {
				if vm[v] < 0 {
					vm[v] = len(vertices)
					vertices = append(vertices, dv[v])
				}
			}
		}
		for old, yes := range used {
			if !yes || em[old] >= 0 {
				continue
			}
			edge := de[old]
			edge.Start, edge.End = vm[edge.Start], vm[edge.End]
			em[old] = len(edges)
			edges = append(edges, edge)
		}
		for _, old := range []int{proof.Blends[end], proof.Caps[end]} {
			face := cloneFace(df[old])
			for i := range face.Outer {
				if em[face.Outer[i].Edge] < 0 {
					return fail(geom.ErrConvergence)
				}
				face.Outer[i].Edge = em[face.Outer[i].Edge]
			}
			id := len(faces)
			faces = append(faces, face)
			h.SourceFaces = append(h.SourceFaces, append([]int(nil), proof.SourceFaces[old]...))
			if old == proof.Blends[end] {
				h.Blends[end] = id
			} else {
				h.Caps[end] = id
			}
		}
		for role, row := range proof.BlendRims[end] {
			for q, id := range row {
				if id < 0 || em[id] < 0 {
					return fail(geom.ErrConvergence)
				}
				h.Rims[end][role][q] = em[id]
			}
		}
		h.Circles[end] = FrustumRollingCircle{Center: proof.MeridianCenter, Normal: proof.ConeNormal, Tangency: proof.Tangency, Radius: proof.Radius}
	}
	out, e := NewSolidContext(ctx, vertices, edges, faces, s.tolerance)
	if e != nil {
		return fail(e)
	}
	if out.Topology().EulerCharacteristic != 2 || len(out.Topology().Shells) != 1 {
		return fail(geom.ErrDegenerate)
	}
	agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return fail(e)
	}
	if !agreement.Complete {
		return fail(geom.ErrConvergence)
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return fail(geom.ErrConvergence)
		}
	}
	return out, h, nil
}
