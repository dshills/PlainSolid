package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// CutWrappedAnisotropicStationLofts removes a strictly nested complete inner
// loft with the same center/end planes and original normalized weighted profile.
// Both independent inner scales must be strictly smaller at every merged source
// station. This sufficient domain permits independent station partitions and
// aspect ratios; no intersecting/tangent walls or stepped caps are accepted.
// Actual walls, annular cap loops and original source history are preserved.
func CutWrappedAnisotropicStationLofts(ctx context.Context, outer, inner Solid, o geom.SurfacePatchOptions) (Solid, WrappedStationBooleanHistory, error) {
	x, xb, e := outer.validateWrappedAnisotropicStationLoft(ctx, o)
	if e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	y, yb, e := inner.validateWrappedAnisotropicStationLoft(ctx, o)
	if e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	if _, e = proveAnisotropicStationNesting(ctx, x, y, xb[0], yb[0], o); e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	return joinProvedWrappedStationLoftBore(ctx, outer, inner, x.History, y.History, x.Heights, y.Heights, o)
}

// Positive weights on canonical quarter controls prove x'<0,y'>0 in its
// interior (with only endpoint zeros). Thus each quadrant region is downward
// closed in its two positive coordinate magnitudes. Independent positive
// diagonal contraction strictly retains every boundary point in the outer
// region. Affine scale gaps need only be checked at the merged station ends.
func proveAnisotropicStationNesting(ctx context.Context, x, y WrappedAnisotropicStationProof, outer, inner *geom.WrappedSurfacePatch, o geom.SurfacePatchOptions) ([2]*big.Rat, error) {
	empty := [2]*big.Rat{}
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	limit, bits := o.MaxOperations, o.MaxCoefficientBits
	if limit == 0 {
		limit = 4000000
	}
	if bits == 0 {
		bits = 16384
	}
	if limit < 1 || bits < 64 {
		return empty, geom.ErrInvalid
	}
	b := &stationMassBudget{ctx: ctx, limit: limit, bits: bits}
	if x.Center[0].Cmp(y.Center[0]) != 0 || x.Center[1].Cmp(y.Center[1]) != 0 || x.Heights[0].Cmp(y.Heights[0]) != 0 || x.Heights[len(x.Heights)-1].Cmp(y.Heights[len(y.Heights)-1]) != 0 {
		return empty, geom.ErrDomain
	}
	_, a, e := stationProfileSegments(outer, b)
	if e != nil {
		return empty, e
	}
	_, c, e := stationProfileSegments(inner, b)
	if e != nil {
		return empty, e
	}
	for i := range a {
		left, right := b.mul(a[i], c[0]), b.mul(c[i], a[0])
		if b.err != nil {
			return empty, b.err
		}
		if left.Cmp(right) != 0 {
			return empty, geom.ErrDomain
		}
	}
	levels := append(append([]*big.Rat(nil), x.Heights...), y.Heights...)
	sort.Slice(levels, func(i, j int) bool { return levels[i].Cmp(levels[j]) < 0 })
	evaluate := func(p WrappedAnisotropicStationProof, z *big.Rat, axis int) *big.Rat {
		band := len(p.Heights) - 2
		for i := 0; i < len(p.Heights)-1; i++ {
			if z.Cmp(p.Heights[i+1]) <= 0 {
				band = i
				break
			}
		}
		t := b.div(b.sub(z, p.Heights[band]), b.sub(p.Heights[band+1], p.Heights[band]))
		return b.add(p.Scales[band][axis], b.mul(t, b.sub(p.Scales[band+1][axis], p.Scales[band][axis])))
	}
	gaps := [2]*big.Rat{}
	for _, z := range levels {
		for axis := 0; axis < 2; axis++ {
			gap := b.sub(evaluate(x, z, axis), evaluate(y, z, axis))
			if b.err != nil {
				return empty, b.err
			}
			if gap.Sign() <= 0 {
				return empty, geom.ErrDomain
			}
			if gaps[axis] == nil || gap.Cmp(gaps[axis]) < 0 {
				gaps[axis] = new(big.Rat).Set(gap)
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return gaps, nil
}

type WrappedAnisotropicStationBoreProof struct {
	Outer, Inner           WrappedAnisotropicStationProof // capped reconstructions
	OuterWalls, InnerWalls []int                          // actual original bore solid face IDs
	Caps                   [2]int
	MinimumScaleGap        [2]*big.Rat // not a Euclidean distance certificate
	Complete               bool
}

// ValidateWrappedAnisotropicStationBore independently rebuilds the two capped
// sources from actual outward/inward stored walls, reproves exact nesting and
// matches all original curves, coedges, annular loops and orientations.
func (s Solid) ValidateWrappedAnisotropicStationBore(ctx context.Context, o geom.SurfacePatchOptions) (WrappedAnisotropicStationBoreProof, error) {
	p, _, e := s.validateAnisotropicStationBore(ctx, o)
	return p, e
}
func (s Solid) validateAnisotropicStationBore(ctx context.Context, o geom.SurfacePatchOptions) (WrappedAnisotropicStationBoreProof, [2]Solid, error) {
	empty := WrappedAnisotropicStationBoreProof{}
	none := [2]Solid{}
	if ctx == nil {
		return empty, none, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, none, e
	}
	if len(s.faces) < 4 || len(s.faces) > 34 {
		return empty, none, geom.ErrDomain
	}
	type side struct {
		z *big.Rat
		w *geom.WrappedSurfacePatch
	}
	var sides [2][]side
	caps := 0
	for _, f := range s.faces {
		switch w := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			p, e := geom.CertifyWrappedAnisotropicEmbedding(ctx, w, o)
			if e != nil {
				return empty, none, e
			}
			if len(f.Holes) != 0 {
				return empty, none, geom.ErrDomain
			}
			i := 0
			if f.Reversed {
				i = 1
			}
			sides[i] = append(sides[i], side{p.LowerZ, w})
		case geom.PlaneSurface:
			if len(f.Holes) != 1 {
				return empty, none, geom.ErrDomain
			}
			caps++
		default:
			return empty, none, geom.ErrDomain
		}
	}
	if caps != 2 || len(sides[0]) == 0 || len(sides[1]) == 0 {
		return empty, none, geom.ErrDomain
	}
	var sources [2]Solid
	var proofs [2]WrappedAnisotropicStationProof
	var first [2]*geom.WrappedSurfacePatch
	for i, items := range sides {
		sort.Slice(items, func(a, b int) bool { return items[a].z.Cmp(items[b].z) < 0 })
		walls := make([]*geom.WrappedSurfacePatch, len(items))
		for j, item := range items {
			walls[j] = item.w
		}
		first[i] = walls[0]
		q, _, e := NewWrappedAnisotropicStationLoft(ctx, walls, s.tolerance, o)
		if e != nil {
			return empty, none, e
		}
		p, _, e := q.validateWrappedAnisotropicStationLoft(ctx, o)
		if e != nil {
			return empty, none, e
		}
		sources[i], proofs[i] = q, p
	}
	gap, e := proveAnisotropicStationNesting(ctx, proofs[0], proofs[1], first[0], first[1], o)
	if e != nil {
		return empty, none, e
	}
	ref, h, e := joinProvedWrappedStationLoftBore(ctx, sources[0], sources[1], proofs[0].History, proofs[1].History, proofs[0].Heights, proofs[1].Heights, o)
	if e != nil {
		return empty, none, e
	}
	ids, e := matchCanonicalWrappedMaterial(ctx, s, ref, o)
	if e != nil {
		return empty, none, e
	}
	p := WrappedAnisotropicStationBoreProof{Outer: proofs[0], Inner: proofs[1], Caps: [2]int{ids[h.ResultCaps[0]], ids[h.ResultCaps[1]]}, MinimumScaleGap: gap, Complete: true}
	for _, band := range h.Bands {
		p.OuterWalls = append(p.OuterWalls, ids[band.ResultWall])
	}
	for _, band := range h.InnerBands {
		p.InnerWalls = append(p.InnerWalls, ids[band.ResultWall])
	}
	return p, sources, nil
}

// LocateWrappedAnisotropicStationBoreRational classifies the authenticated
// through material. Inner cap-disk interiors are openings, not boundary faces.
// On a terminal plane the inner rim remains boundary but its disk is outside.
func (s Solid) LocateWrappedAnisotropicStationBoreRational(ctx context.Context, point [3]*big.Rat, o WrappedRationalStationPointOptions) (parametric.Location, error) {
	p, src, e := s.validateAnisotropicStationBore(ctx, o.Source)
	if e != nil {
		return parametric.Outside, e
	}
	remaining := o.MaxPointOperations
	if remaining == 0 {
		remaining = 4096
	}
	if remaining < 1 || remaining > 1000000 {
		return parametric.Outside, geom.ErrInvalid
	}
	var points [2]WrappedAnisotropicStationPoint
	for i, proof := range []WrappedAnisotropicStationProof{p.Outer, p.Inner} {
		walls := make([]*geom.WrappedSurfacePatch, len(proof.History.WallFaces))
		for j, id := range proof.History.WallFaces {
			walls[j] = src[i].faces[id].Surface.(*geom.WrappedSurfacePatch)
		}
		if remaining < 1 {
			return parametric.Outside, geom.ErrConvergence
		}
		opts := o
		opts.MaxPointOperations = remaining
		points[i], e = src[i].locatePreparedAnisotropicStationRational(ctx, point, opts, &proof, walls)
		if e != nil {
			return parametric.Outside, e
		}
		remaining -= points[i].ExactOperations
	}
	a, c := points[0], points[1]
	if a.Location == parametric.Outside || c.Location == parametric.Inside {
		return parametric.Outside, nil
	}
	if c.Location == parametric.Boundary {
		wall := false
		for _, face := range c.Faces {
			for _, id := range p.Inner.History.WallFaces {
				wall = wall || face == id
			}
		}
		if !wall {
			return parametric.Outside, nil
		}
		return parametric.Boundary, nil
	}
	return a.Location, nil
}
