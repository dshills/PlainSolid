package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedFrustumBoreSection retains the closed original rim curves at one
// horizontal section. A bore floor adds an inner boundary curve there, but
// its retained disk fills that curve unless the corresponding end is open.
// Face IDs are in the receiver's native indexing. Absent owners are -1.
type WrappedFrustumBoreSection struct {
	Outer, Inner                  geom.WrappedFrustumSection
	Height                        *big.Rat
	OuterFace, InnerFace, CapFace int
	MaterialPresent, HasHole      bool
}

// SectionCanonicalWrappedFrustumBore intersects the complete native
// circular-frustum bore with a represented model Z plane. It returns exact
// original U curves, including at a nonbinary original V.
func (s Solid) SectionCanonicalWrappedFrustumBore(ctx context.Context, height float64, o geom.SurfacePatchOptions) (WrappedFrustumBoreSection, error) {
	var empty WrappedFrustumBoreSection
	if !finite(height) {
		return empty, geom.ErrInvalid
	}
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return empty, e
	}
	z := ratInset(height)
	a, e := geom.CertifyWrappedFrustumEmbedding(ctx, p.Outer, o)
	if e != nil {
		return empty, e
	}
	b, e := geom.CertifyWrappedFrustumEmbedding(ctx, p.Inner, o)
	if e != nil {
		return empty, e
	}
	r := WrappedFrustumBoreSection{Height: z, OuterFace: -1, InnerFace: -1, CapFace: -1}
	if z.Cmp(a.LowerZ) < 0 || z.Cmp(a.UpperZ) > 0 {
		return r, nil
	}
	r.Outer, e = geom.SectionWrappedFrustum(ctx, p.Outer, height, o)
	if e != nil {
		return empty, e
	}
	r.MaterialPresent, r.OuterFace = true, p.OuterFaces[0]
	if z.Cmp(a.LowerZ) == 0 {
		r.CapFace = p.OuterFaces[1]
	}
	if z.Cmp(a.UpperZ) == 0 {
		r.CapFace = p.OuterFaces[2]
	}
	lo, hi := z.Cmp(b.LowerZ), z.Cmp(b.UpperZ)
	if lo < 0 || hi > 0 {
		return r, nil
	}
	r.Inner, e = geom.SectionWrappedFrustum(ctx, p.Inner, height, o)
	if e != nil {
		return empty, e
	}
	r.InnerFace = p.InnerFaces[0]
	r.HasHole = lo > 0 && hi < 0
	if lo == 0 || hi == 0 {
		i := 1
		if hi == 0 {
			i = 2
		}
		r.CapFace = p.InnerFaces[i]
		r.HasHole = p.InnerFaces[i] == p.OuterFaces[i]
	}
	return r, nil
}

// LocateCanonicalWrappedFrustumBore locates material on an independently
// validated native circular frustum bore, using the original two axes.
func (s Solid) LocateCanonicalWrappedFrustumBore(ctx context.Context, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	return LocateWrappedEccentricCircularFrustumBore(ctx, p.Outer, p.Inner, point, o)
}

// IntersectCanonicalWrappedFrustumOpenBoreLine preserves the receiver's
// actual face IDs for every contact, even when its topology was reindexed.
func (s Solid) IntersectCanonicalWrappedFrustumOpenBoreLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return s.intersectCanonicalFrustumBoreLine(ctx, origin, direction, minT, maxT, o, true)
}

// IntersectCanonicalWrappedFrustumBoreLine returns the contacts and overlaps
// with a native blind bore, through bore, or enclosed cavity. The line uses
// the caller's original parameterization and every owner is a receiver face ID.
func (s Solid) IntersectCanonicalWrappedFrustumBoreLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return s.intersectCanonicalFrustumBoreLine(ctx, origin, direction, minT, maxT, o, false)
}

func (s Solid) intersectCanonicalFrustumBoreLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	o geom.ToroidLineOptions, oneOpening bool) (WrappedLoftLineIntersections, error) {
	var empty WrappedLoftLineIntersections
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Surface)
	if e != nil {
		return empty, e
	}
	if oneOpening && p.Kind != "blind-bore" {
		return empty, geom.ErrConvergence
	}
	r, e := IntersectWrappedEccentricCircularFrustumBoreLine(ctx, p.Outer, p.Inner, origin, direction, minT, maxT, o)
	if e != nil {
		return empty, e
	}
	ids := canonicalFrustumBoreLineFaceIDs(p)
	for i := range r.Hits {
		for j, id := range r.Hits[i].Faces {
			r.Hits[i].Faces[j] = ids[id]
		}
		sort.Ints(r.Hits[i].Faces)
	}
	for i := range r.Overlaps {
		r.Overlaps[i].Face = ids[r.Overlaps[i].Face]
	}
	return r, nil
}

func canonicalFrustumBoreLineFaceIDs(p WrappedBoreProof) []int {
	ids := []int{p.OuterFaces[0], p.OuterFaces[1], p.OuterFaces[2], p.InnerFaces[0]}
	for i := 1; i < 3; i++ {
		if p.InnerFaces[i] != p.OuterFaces[i] {
			ids = append(ids, p.InnerFaces[i])
		}
	}
	return ids
}

// TessellateCanonicalWrappedFrustumOpenBoreCertified returns the certified
// display with triangle owner IDs from this original native solid.
func (s Solid) TessellateCanonicalWrappedFrustumOpenBoreCertified(ctx context.Context, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreMesh, error) {
	var empty CertifiedWrappedFrustumOpenBoreMesh
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Embedding)
	if e != nil {
		return empty, e
	}
	if p.Kind != "blind-bore" {
		return empty, geom.ErrConvergence
	}
	m, e := TessellateWrappedEccentricFrustumOpenBoreCertified(ctx, p.Outer, p.Inner, o)
	if e != nil {
		return empty, e
	}
	ids, e := canonicalFrustumBoreFaceIDs(p)
	if e != nil {
		return empty, e
	}
	for i, id := range m.OriginalFace {
		m.OriginalFace[i] = ids[id]
	}
	return m, nil
}

func canonicalFrustumBoreFaceIDs(p WrappedBoreProof) ([5]int, error) {
	ids := [...]int{p.OuterFaces[0], p.OuterFaces[1], p.OuterFaces[2], p.InnerFaces[0], -1}
	for i := 1; i < 3; i++ {
		if p.InnerFaces[i] != p.OuterFaces[i] {
			if ids[4] >= 0 {
				return [5]int{}, geom.ErrConvergence
			}
			ids[4] = p.InnerFaces[i]
		}
	}
	if ids[4] < 0 {
		return [5]int{}, geom.ErrConvergence
	}
	return ids, nil
}
