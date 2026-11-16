package fillet

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ChamferWrappedFrustumTubeRims independently bevels four rims of a native
// coaxial circular annular frustum. Outer and inner original wall slopes can
// differ. Authored radial setbacks vary affinely to zero at their axial runouts;
// the unchanged original wall equations supply the underlying radius at Z.
// Exact positive outer-minus-inner clearance is checked at the union of caps
// and all runout heights, proving every intervening affine interval is nested.
// Each wall's opposite runouts must be strictly disjoint. Original circular
// bases and cap planes survive; ancestry, native seam reproof and per-call
// limits match ChamferWrappedCylinderTubeRims. Eccentric axes, apex/contact,
// nonrepresentable geometry and unresolved proofs fail atomically. No normal
// length square root is required for these radial/axial authored bevels.
func ChamferWrappedFrustumTubeRims(ctx context.Context, source trimmed.Solid, selection CircularTubeChamfers, o geom.SurfacePatchOptions) (WrappedTubeChamfer, error) {
	return chamferWrappedFrustumTubeRims(ctx, source, selection, o, true)
}

// ChamferWrappedEccentricFrustumTubeRims applies independent outer and inner
// radial/axial rim bevels to a canonical circular through-bore with parallel
// axes. Each center and original cap plane is retained. A positive radial
// gap whose square exceeds the squared axis separation is proved at every
// cap and runout, with affine interpolation proving strict nesting between
// them. Invalid runouts, tangent walls, or nonrepresentable geometry fail
// without a partial result. Ancestry and options match the coaxial operation.
func ChamferWrappedEccentricFrustumTubeRims(ctx context.Context, source trimmed.Solid, selection CircularTubeChamfers, o geom.SurfacePatchOptions) (WrappedTubeChamfer, error) {
	return chamferWrappedFrustumTubeRims(ctx, source, selection, o, false)
}

func chamferWrappedFrustumTubeRims(ctx context.Context, source trimmed.Solid, selection CircularTubeChamfers, o geom.SurfacePatchOptions, coaxial bool) (WrappedTubeChamfer, error) {
	fail := func(e error) (WrappedTubeChamfer, error) { return WrappedTubeChamfer{}, e }
	if ctx == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	p, e := source.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return fail(e)
	}
	if !p.Complete || p.Kind != "tube" {
		return fail(geom.ErrConvergence)
	}
	supports := [2]*geom.WrappedSurfacePatch{p.Outer, p.Inner}
	proofs := [2]geom.WrappedFrustumEmbedding{}
	for i, w := range supports {
		proofs[i], e = geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
		if e != nil {
			return fail(e)
		}
		if !proofs[i].Circular {
			return fail(geom.ErrConvergence)
		}
	}
	if coaxial && (proofs[0].CenterX.Cmp(proofs[1].CenterX) != 0 || proofs[0].CenterY.Cmp(proofs[1].CenterY) != 0) ||
		proofs[0].LowerZ.Cmp(proofs[1].LowerZ) != 0 || proofs[0].UpperZ.Cmp(proofs[1].UpperZ) != 0 {
		return fail(geom.ErrConvergence)
	}
	slopes := [2]*big.Rat{}
	for i, q := range proofs {
		slopes[i] = new(big.Rat).Quo(new(big.Rat).Sub(q.UpperRadius, q.LowerRadius), new(big.Rat).Sub(q.UpperZ, q.LowerZ))
	}
	lo, hi := proofs[0].LowerZ, proofs[0].UpperZ
	height := new(big.Rat).Sub(hi, lo)
	settings := [2][2]CircularRimChamfer{selection.Outer, selection.Inner}
	radial, axial := [2][2]*big.Rat{}, [2][2]*big.Rat{}
	levels := []*big.Rat{lo, hi}
	selected := false
	for side := 0; side < 2; side++ {
		for end, c := range settings[side] {
			radial[side][end], axial[side][end] = new(big.Rat).SetFloat64(c.Radial), new(big.Rat).SetFloat64(c.Axial)
			a, b := radial[side][end], axial[side][end]
			if a == nil || b == nil || a.Sign() < 0 || b.Sign() < 0 || (a.Sign() == 0) != (b.Sign() == 0) {
				return fail(geom.ErrInvalid)
			}
			if side == 0 && a.Cmp([2]*big.Rat{proofs[side].LowerRadius, proofs[side].UpperRadius}[end]) >= 0 {
				return fail(geom.ErrDomain)
			}
			if b.Sign() > 0 {
				selected = true
				z := new(big.Rat).Add(lo, b)
				if end == 1 {
					z.Sub(hi, b)
				}
				levels = append(levels, z)
			}
		}
		if new(big.Rat).Add(axial[side][0], axial[side][1]).Cmp(height) >= 0 {
			return fail(geom.ErrDomain)
		}
	}
	if !selected {
		return fail(geom.ErrInvalid)
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i].Cmp(levels[j]) < 0 })
	unique := levels[:0]
	for _, z := range levels {
		if len(unique) == 0 || z.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, z)
		}
	}
	levels = unique
	radiusAt := func(side int, z *big.Rat) (*big.Rat, int) {
		r := new(big.Rat).Add(proofs[side].LowerRadius, new(big.Rat).Mul(slopes[side], new(big.Rat).Sub(z, lo)))
		for end := 0; end < 2; end++ {
			distance := new(big.Rat).Sub(z, lo)
			if end == 1 {
				distance.Sub(hi, z)
			}
			if axial[side][end].Sign() > 0 && distance.Cmp(axial[side][end]) < 0 {
				delta := new(big.Rat).Mul(radial[side][end], new(big.Rat).Quo(new(big.Rat).Sub(axial[side][end], distance), axial[side][end]))
				if side == 0 {
					r.Sub(r, delta)
				} else {
					r.Add(r, delta)
				}
				return r, end
			}
		}
		return r, -1
	}
	for _, z := range levels {
		a, _ := radiusAt(0, z)
		b, _ := radiusAt(1, z)
		if a.Sign() <= 0 || b.Sign() <= 0 || a.Cmp(b) <= 0 {
			return fail(geom.ErrDomain)
		}
	}
	var solids [2]trimmed.Solid
	roles := [2]map[int]int{make(map[int]int), make(map[int]int)}
	for side, w := range supports {
		base, _, e := geom.DraftWrappedFrustumExact(ctx, w, new(big.Rat), new(big.Rat).Sub(proofs[side].LowerRadius, proofs[side].UpperRadius), o)
		if e != nil {
			return fail(e)
		}
		bands := make([]*geom.WrappedSurfacePatch, len(levels)-1)
		ends := make([]int, len(bands))
		for i := range bands {
			r0, _ := radiusAt(side, levels[i])
			r1, _ := radiusAt(side, levels[i+1])
			mid := new(big.Rat).Quo(new(big.Rat).Add(levels[i], levels[i+1]), big.NewRat(2, 1))
			_, ends[i] = radiusAt(side, mid)
			band, e := geom.ReheightWrappedCylinder(ctx, base, levels[i], levels[i+1], o)
			if e != nil {
				return fail(e)
			}
			bands[i], _, e = geom.DraftWrappedFrustumExact(ctx, band, new(big.Rat).Sub(r0, proofs[side].LowerRadius), new(big.Rat).Sub(r1, proofs[side].LowerRadius), o)
			if e != nil {
				return fail(e)
			}
		}
		var history trimmed.WrappedLoftHistory
		solids[side], history, e = trimmed.NewWrappedCircularStationLoft(ctx, bands, source.Tolerance(), o)
		if e != nil {
			return fail(e)
		}
		for i, face := range history.WallFaces {
			roles[side][face] = ends[i]
		}
	}
	var out trimmed.Solid
	var h trimmed.WrappedStationBooleanHistory
	if coaxial {
		out, h, e = trimmed.CombineCoaxialWrappedStationLofts(ctx, solids[0], solids[1], parametric.Difference, o)
	} else {
		out, h, e = trimmed.CutWrappedEccentricCircularStationLofts(ctx, solids[0], solids[1], o)
	}
	if e != nil {
		return fail(e)
	}
	result := WrappedTubeChamfer{Solid: out, SourceFaces: make([][]int, len(out.Faces())), Caps: h.ResultCaps}
	for end, cap := range h.ResultCaps {
		result.SourceFaces[cap] = []int{p.OuterFaces[end+1]}
	}
	for side, bands := range [][]trimmed.WrappedStationBooleanBand{h.Bands, h.InnerBands} {
		oldWall := p.OuterFaces[0]
		if side == 1 {
			oldWall = p.InnerFaces[0]
		}
		for _, band := range bands {
			end, ok := roles[side][band.SourceWalls[side]]
			if !ok || !band.BasisPreserved {
				return fail(geom.ErrConvergence)
			}
			id := band.ResultWall
			result.Walls[side] = append(result.Walls[side], id)
			result.SourceFaces[id] = []int{oldWall}
			if end >= 0 {
				result.SourceFaces[id] = append(result.SourceFaces[id], p.OuterFaces[end+1])
				result.Bevels[side][end] = append(result.Bevels[side][end], id)
			}
		}
	}
	for _, owners := range result.SourceFaces {
		if len(owners) == 0 {
			return fail(geom.ErrConvergence)
		}
	}
	return result, nil
}
