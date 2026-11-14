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
	Outer, Inner geom.WrappedFrustumSection
	Height       *big.Rat
	// AreaPi is the exact area of retained planar material divided by π.
	// A retained cavity floor fills its disk on the plane itself.
	AreaPi                        *big.Rat
	OuterFace, InnerFace, CapFace int
	MaterialPresent, HasHole      bool
}

// SectionCanonicalWrappedFrustumBore intersects the complete native
// circular-frustum bore with a represented model Z plane. It returns exact
// original U curves, including at a nonbinary original V.
func (s Solid) SectionCanonicalWrappedFrustumBore(ctx context.Context, height float64, o geom.SurfacePatchOptions) (WrappedFrustumBoreSection, error) {
	if !finite(height) {
		return WrappedFrustumBoreSection{}, geom.ErrInvalid
	}
	return s.SectionCanonicalWrappedFrustumBoreRational(ctx, ratInset(height), o)
}

// SectionCanonicalWrappedFrustumBoreRational retains exact chart and material
// section data at any bounded rational height. No modeled height or radius is
// rounded to produce a witness.
func (s Solid) SectionCanonicalWrappedFrustumBoreRational(ctx context.Context, height *big.Rat, o geom.SurfacePatchOptions) (WrappedFrustumBoreSection, error) {
	var empty WrappedFrustumBoreSection
	if height == nil || o.MaxCoefficientBits < 0 {
		return empty, geom.ErrInvalid
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if height.Num().BitLen() > bits || height.Denom().BitLen() > bits {
		return empty, geom.ErrConvergence
	}
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return empty, e
	}
	z := new(big.Rat).Set(height)
	a, e := geom.CertifyWrappedFrustumEmbedding(ctx, p.Outer, o)
	if e != nil {
		return empty, e
	}
	b, e := geom.CertifyWrappedFrustumEmbedding(ctx, p.Inner, o)
	if e != nil {
		return empty, e
	}
	r := WrappedFrustumBoreSection{Height: z, AreaPi: new(big.Rat), OuterFace: -1, InnerFace: -1, CapFace: -1}
	if z.Cmp(a.LowerZ) < 0 || z.Cmp(a.UpperZ) > 0 {
		return r, nil
	}
	r.Outer, e = geom.SectionWrappedFrustumRational(ctx, p.Outer, z, o)
	if e != nil {
		return empty, e
	}
	r.MaterialPresent, r.OuterFace = true, p.OuterFaces[0]
	r.AreaPi.Mul(r.Outer.Radius, r.Outer.Radius)
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
	r.Inner, e = geom.SectionWrappedFrustumRational(ctx, p.Inner, z, o)
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
	if r.HasHole {
		r.AreaPi.Sub(r.AreaPi, new(big.Rat).Mul(r.Inner.Radius, r.Inner.Radius))
	}
	return r, nil
}

// LocateCanonicalWrappedFrustumBore locates material on an independently
// validated native circular frustum bore, using the original two axes.
func (s Solid) LocateCanonicalWrappedFrustumBore(ctx context.Context, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	return s.LocateCanonicalWrappedFrustumBoreRational(ctx, [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)}, o)
}

// LocateCanonicalWrappedFrustumBoreRational classifies a nonrounded model
// point against the independently reconstructed native material boundary.
func (s Solid) LocateCanonicalWrappedFrustumBoreRational(ctx context.Context, point [3]*big.Rat,
	o geom.SurfacePatchOptions) (parametric.Location, error) {
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	return LocateWrappedEccentricCircularFrustumBoreRational(ctx, p.Outer, p.Inner, point, o)
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
	if !origin.IsFinite() || !direction.IsFinite() || !finite(minT) || !finite(maxT) {
		return WrappedLoftLineIntersections{}, geom.ErrInvalid
	}
	return s.intersectCanonicalFrustumBoreLineRational(ctx,
		[3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)},
		[3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}, ratInset(minT), ratInset(maxT), o, oneOpening)
}

// IntersectCanonicalWrappedFrustumBoreLineRational preserves exact line
// coefficients, parameter bounds, and native face ownership.
func (s Solid) IntersectCanonicalWrappedFrustumBoreLineRational(ctx context.Context, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return s.intersectCanonicalFrustumBoreLineRational(ctx, origin, direction, minT, maxT, o, false)
}

func (s Solid) intersectCanonicalFrustumBoreLineRational(ctx context.Context, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions, oneOpening bool) (WrappedLoftLineIntersections, error) {
	var empty WrappedLoftLineIntersections
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Surface)
	if e != nil {
		return empty, e
	}
	if oneOpening && p.Kind != "blind-bore" {
		return empty, geom.ErrConvergence
	}
	r, e := IntersectWrappedEccentricCircularFrustumBoreLineRational(ctx, p.Outer, p.Inner, origin, direction, minT, maxT, o)
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
