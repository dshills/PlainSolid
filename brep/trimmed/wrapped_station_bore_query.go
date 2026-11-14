package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedStationBoreProof recovers both original band families and two
// annular caps of a strictly nested equal-height station loft bore.
// The station proofs refer to reconstructed cap-complete shells; their wall
// IDs are remapped in OuterWalls and InnerWalls to the original solid.
type WrappedStationBoreProof struct {
	Outer, Inner           WrappedStationLoftProof
	OuterWalls, InnerWalls []int
	Caps                   [2]int
	Complete               bool
}

// ValidateWrappedCircularStationBore independently reconstructs the two
// capped lofts and proves their nested radii and the original shared
// annular topology. This includes exact validation after native reload.
func (s Solid) ValidateWrappedCircularStationBore(ctx context.Context, o geom.SurfacePatchOptions) (WrappedStationBoreProof, error) {
	return s.validateWrappedProfileStationBore(ctx, o, true)
}

// ValidateWrappedRationalStationBore independently authenticates both
// original outward/inward rational profile families and their annular caps.
func (s Solid) ValidateWrappedRationalStationBore(ctx context.Context, o geom.SurfacePatchOptions) (WrappedStationBoreProof, error) {
	return s.validateWrappedProfileStationBore(ctx, o, false)
}

func (s Solid) validateWrappedProfileStationBore(ctx context.Context, o geom.SurfacePatchOptions, circular bool) (WrappedStationBoreProof, error) {
	if ctx == nil {
		return WrappedStationBoreProof{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return WrappedStationBoreProof{}, e
	}
	if len(s.faces) < 4 || len(s.faces) > 256 || !circular && len(s.faces) > 34 {
		return WrappedStationBoreProof{}, geom.ErrConvergence
	}
	type item struct {
		z *big.Rat
		w *geom.WrappedSurfacePatch
	}
	var sides [2][]item
	caps := 0
	for _, f := range s.faces {
		switch w := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o)
			if e != nil {
				return WrappedStationBoreProof{}, e
			}
			if circular && !p.Circular || len(f.Holes) != 0 {
				return WrappedStationBoreProof{}, geom.ErrConvergence
			}
			i := 0
			if f.Reversed {
				i = 1
			}
			sides[i] = append(sides[i], item{p.LowerZ, w})
		case geom.PlaneSurface:
			if len(f.Holes) != 1 {
				return WrappedStationBoreProof{}, geom.ErrConvergence
			}
			caps++
		default:
			return WrappedStationBoreProof{}, geom.ErrConvergence
		}
	}
	if caps != 2 || len(sides[0]) == 0 || len(sides[1]) == 0 {
		return WrappedStationBoreProof{}, geom.ErrConvergence
	}
	var solids [2]Solid
	var proofs [2]WrappedStationLoftProof
	build := NewWrappedRationalStationLoft
	if circular {
		build = NewWrappedCircularStationLoft
	}
	for i, items := range sides {
		sort.Slice(items, func(i, j int) bool { return items[i].z.Cmp(items[j].z) < 0 })
		w := make([]*geom.WrappedSurfacePatch, len(items))
		for j, x := range items {
			w[j] = x.w
		}
		q, _, e := build(ctx, w, s.tolerance, o)
		if e != nil {
			return WrappedStationBoreProof{}, e
		}
		p, _, e := q.validateWrappedProfileStationLoft(ctx, o, circular)
		if e != nil {
			return WrappedStationBoreProof{}, e
		}
		solids[i], proofs[i] = q, p
	}
	ref, h, e := combineCoaxialWrappedStationLofts(ctx, solids[0], solids[1], parametric.Difference, o, circular)
	if e != nil {
		return WrappedStationBoreProof{}, e
	}
	if len(h.InnerBands) == 0 {
		return WrappedStationBoreProof{}, geom.ErrConvergence
	}
	faces, e := matchCanonicalWrappedMaterial(ctx, s, ref, o)
	if e != nil {
		return WrappedStationBoreProof{}, e
	}
	r := WrappedStationBoreProof{Outer: proofs[0], Inner: proofs[1], Caps: [2]int{faces[h.ResultCaps[0]], faces[h.ResultCaps[1]]}, Complete: true}
	for _, b := range h.Bands {
		r.OuterWalls = append(r.OuterWalls, faces[b.ResultWall])
	}
	for _, b := range h.InnerBands {
		r.InnerWalls = append(r.InnerWalls, faces[b.ResultWall])
	}
	return r, nil
}

// LocateWrappedCircularStationBore uses exact radius interpolation for both
// walls. A point strictly within an end-cap opening is outside material;
// the cap annulus and its two rims are boundary.
func (s Solid) LocateWrappedCircularStationBore(ctx context.Context, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCircularStationBore(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	z := ratInset(point.Z)
	r, valid := wrappedStationRadiusAt(p.Outer, z)
	if !valid {
		return parametric.Outside, nil
	}
	q, valid := wrappedStationRadiusAt(p.Inner, z)
	if !valid {
		return parametric.Outside, geom.ErrConvergence
	}
	x, y := new(big.Rat).Sub(ratInset(point.X), p.Outer.Center[0]), new(big.Rat).Sub(ratInset(point.Y), p.Outer.Center[1])
	d := new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
	a, b := d.Cmp(r.Mul(r, r)), d.Cmp(q.Mul(q, q))
	if a > 0 || b < 0 {
		return parametric.Outside, nil
	}
	if a == 0 || b == 0 || z.Cmp(p.Outer.Heights[0]) == 0 || z.Cmp(p.Outer.Heights[len(p.Outer.Heights)-1]) == 0 {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

// IntersectWrappedCircularStationBoreLine proves all contacts with a finite
// represented line, including intervals coincident with the conical walls
// or annular caps. Exact cap rim roots retain cap and wall ancestry.
func (s Solid) IntersectWrappedCircularStationBoreLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	var empty WrappedLoftLineIntersections
	if ctx == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (geom.Vec3{}) || !finite(minT) || !finite(maxT) ||
		minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCircularStationBore(ctx, o.Surface)
	if e != nil {
		return empty, e
	}
	if direction.Z == 0 {
		z := ratInset(origin.Z)
		for i, j := range []int{0, len(p.Outer.Heights) - 1} {
			if z.Cmp(p.Outer.Heights[j]) != 0 {
				continue
			}
			k := 0
			if i == 1 {
				k = len(p.Inner.Heights) - 1
			}
			q, e := geom.IntersectCircularAnnulusLine(ctx, p.Outer.Center, p.Inner.Radii[k], p.Outer.Radii[j],
				geom.Vec2{X: origin.X, Y: origin.Y}, geom.Vec2{X: direction.X, Y: direction.Y}, minT, maxT, o)
			if e != nil {
				return empty, e
			}
			var out WrappedLoftLineIntersections
			cap := [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap}[i]
			for _, h := range q.Rims {
				walls := p.OuterWalls
				if h.Inner {
					walls = p.InnerWalls
				}
				wall := walls[i*(len(walls)-1)]
				ids := []int{p.Caps[i], wall}
				sort.Ints(ids)
				out.Hits = append(out.Hits, WrappedLoftLineHit{h.Parameter, ids, cap | geom.CylinderLateral})
			}
			if q.Point != nil {
				out.Hits = append(out.Hits, WrappedLoftLineHit{*q.Point, []int{p.Caps[i]}, cap})
			}
			for _, span := range q.Spans {
				out.Overlaps = append(out.Overlaps, WrappedLoftLineOverlap{span[0], span[1], p.Caps[i], cap})
			}
			return out, nil
		}
	}
	type group struct {
		p    WrappedStationLoftProof
		wall []int
	}
	var r WrappedLoftLineIntersections
	for _, g := range []group{{p.Outer, p.OuterWalls}, {p.Inner, p.InnerWalls}} {
		bands := make([]*geom.WrappedSurfacePatch, len(g.wall))
		for i, id := range g.wall {
			bands[i] = s.faces[id].Surface.(*geom.WrappedSurfacePatch)
		}
		q, e := intersectWrappedStationLoftBands(ctx, g.p, bands, origin, direction, minT, maxT, o)
		if e != nil {
			return empty, e
		}
		for _, h := range q.Hits {
			if h.Parts&geom.CylinderLateral == 0 {
				continue
			}
			ids := make([]int, 0, len(h.Faces))
			for _, id := range h.Faces {
				i := sort.SearchInts(g.p.History.WallFaces, id)
				if i < len(g.p.History.WallFaces) && g.p.History.WallFaces[i] == id {
					ids = append(ids, g.wall[i])
				}
			}
			for i, cap := range [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap} {
				if h.Parts&cap != 0 {
					ids = append(ids, p.Caps[i])
				}
			}
			sort.Ints(ids)
			r.Hits = append(r.Hits, WrappedLoftLineHit{Parameter: h.Parameter, Faces: ids, Parts: h.Parts})
		}
		for _, h := range q.Overlaps {
			if h.Part != geom.CylinderLateral {
				continue
			}
			i := sort.SearchInts(g.p.History.WallFaces, h.Face)
			if i == len(g.wall) || g.p.History.WallFaces[i] != h.Face {
				return empty, geom.ErrConvergence
			}
			h.Face = g.wall[i]
			r.Overlaps = append(r.Overlaps, h)
		}
	}
	if direction.Z != 0 {
		for i, part := range [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap} {
			j, k := 0, 0
			if i == 1 {
				j, k = len(p.Outer.Heights)-1, len(p.Inner.Heights)-1
			}
			t := new(big.Rat).Quo(new(big.Rat).Sub(p.Outer.Heights[j], ratInset(origin.Z)), ratInset(direction.Z))
			if t.Cmp(ratInset(minT)) < 0 || t.Cmp(ratInset(maxT)) > 0 {
				continue
			}
			coordinate := func(axis, speed float64, center *big.Rat) *big.Rat {
				return new(big.Rat).Add(new(big.Rat).Sub(ratInset(axis), center), new(big.Rat).Mul(t, ratInset(speed)))
			}
			x, y := coordinate(origin.X, direction.X, p.Outer.Center[0]), coordinate(origin.Y, direction.Y, p.Outer.Center[1])
			d := new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
			a := d.Cmp(new(big.Rat).Mul(p.Outer.Radii[j], p.Outer.Radii[j]))
			b := d.Cmp(new(big.Rat).Mul(p.Inner.Radii[k], p.Inner.Radii[k]))
			if a < 0 && b > 0 {
				q := geom.ToroidLineRoot{Min: t, Max: new(big.Rat).Set(t), Multiplicity: 1}
				r.Hits = append(r.Hits, WrappedLoftLineHit{Parameter: q, Faces: []int{p.Caps[i]}, Parts: part})
			}
		}
	}
	sort.SliceStable(r.Hits, func(i, j int) bool { return r.Hits[i].Parameter.Min.Cmp(r.Hits[j].Parameter.Min) < 0 })
	for i := 1; i < len(r.Hits); i++ {
		if r.Hits[i-1].Parameter.Max.Cmp(r.Hits[i].Parameter.Min) >= 0 {
			return empty, geom.ErrConvergence
		}
	}
	sort.SliceStable(r.Overlaps, func(i, j int) bool { return r.Overlaps[i].Start.Min.Cmp(r.Overlaps[j].Start.Min) < 0 })
	return r, nil
}
