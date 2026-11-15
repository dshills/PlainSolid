package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AffineShellCapDiscoveryOptions bounds exhaustive face-pair discovery and final
// cap/occupancy reproof cumulatively (64M operations, 256 pairs by default;
// maxima 128M and 16384). Embedding/patch recovery keep their existing bounds.
type AffineShellCapDiscoveryOptions struct {
	Contact                     AffineShellCapContactOptions
	MaxOperations, MaxFacePairs int
}
type AffineShellCapDiscovery struct {
	contact     *AffineShellCapContact
	pairs, work int
}

func (r *AffineShellCapDiscovery) Contact() *AffineShellCapContact {
	if r == nil {
		return nil
	}
	return r.contact
}
func (r *AffineShellCapDiscovery) CompleteFacePairs() int {
	if r == nil {
		return 0
	}
	return r.pairs
}
func (r *AffineShellCapDiscovery) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}

// DiscoverAffineShellCapContact authenticates every original finite face pair,
// requires exactly one positive-area coplanar pair and proves that all remaining
// contacts lie in that interface. Same-side solids, ambiguity and unsupported
// geometry fail without a speculative fallback or caller-selected face IDs.
func DiscoverAffineShellCapContact(ctx context.Context, a, c Solid, o AffineShellCapDiscoveryOptions) (*AffineShellCapDiscovery, error) {
	if ctx == nil || len(a.faces) == 0 || len(c.faces) == 0 {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxFacePairs == 0 {
		o.MaxFacePairs = 256
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxFacePairs < 1 || o.MaxFacePairs > 16384 || o.Contact.MaxOperations < 0 || o.Contact.MaxOperations > 128000000 {
		return nil, geom.ErrInvalid
	}
	if len(a.faces) > o.MaxFacePairs/len(c.faces) {
		return nil, geom.ErrConvergence
	}
	var charts [2][]splineEmbeddingFace
	for owner, s := range [2]Solid{a, c} {
		if _, e := s.validateSplineRectangleEmbedding(ctx, o.Contact.Embedding, &charts[owner]); e != nil {
			return nil, e
		}
	}
	result := &AffineShellCapDiscovery{}
	candidate := [2]int{-1, -1}
	points := [][3]*big.Rat{}
	for i, x := range charts[0] {
		for j, y := range charts[1] {
			if result.work >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			pa, e := algebraicShellTracePatch(a.faces[i], x.patch.Domain())
			if e != nil {
				return nil, e
			}
			pc, e := algebraicShellTracePatch(c.faces[j], y.patch.Domain())
			if e != nil {
				return nil, e
			}
			limit := o.MaxOperations - result.work
			if limit > 8000000 {
				limit = 8000000
			}
			contact, e := intersect.IntersectAffinePatches(ctx, pa, pc, o.Contact.Patches, geom.AffineChartIntersectionOptions{MaxOperations: limit, MaxCoefficientBits: 16384})
			if e != nil {
				return nil, e
			}
			result.work += contact.ExactOperations()
			result.pairs++
			kind, e := contact.Kind()
			if e != nil {
				return nil, e
			}
			if kind == intersect.AffinePatchContactArea {
				if candidate[0] >= 0 {
					return nil, fmt.Errorf("affine cap: multiple positive-area face pairs: %w", geom.ErrDomain)
				}
				candidate = [2]int{i, j}
			}
			if kind != intersect.AffinePatchContactEmpty {
				vs, e := contact.Vertices()
				if e != nil {
					return nil, e
				}
				for _, v := range vs {
					points = append(points, v.Point)
				}
			}
		}
	}
	if candidate[0] < 0 {
		return nil, geom.ErrDomain
	}
	if result.work >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	final := o.Contact
	if final.MaxOperations == 0 {
		final.MaxOperations = 64000000
	}
	if remaining := o.MaxOperations - result.work; final.MaxOperations > remaining {
		final.MaxOperations = remaining
	}
	var e error
	result.contact, e = CertifyAffineShellCapContact(ctx, a, c, candidate, final)
	if e != nil {
		return nil, e
	}
	result.work += result.contact.ExactOperations()
	// Positive-area uniqueness alone is not coverage. Every other exact segment
	// or point must belong to both original cap rectangles. Affineness and convex
	// rectangles then cover the entire finite contact between its endpoints.
	for _, p := range points {
		for owner := 0; owner < 2; owner++ {
			remaining := o.MaxOperations - result.work
			if remaining < 1 {
				return nil, geom.ErrConvergence
			}
			q, e := result.contact.affine[owner][candidate[owner]].InvertExactContext(ctx, p, geom.AffineChartInverseOptions{MaxOperations: remaining, MaxCoefficientBits: 16384})
			result.work += q.ExactOperations
			if e != nil {
				return nil, e
			}
			if !q.OnPlane || !q.Inside {
				return nil, geom.ErrDomain
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return result, nil
}

type AffineShellCapNativeOptions struct {
	Discovery AffineShellCapDiscoveryOptions
	Fuse      AffineShellCapFuseOptions
}

func combineSplineShellsAffineCap(ctx context.Context, a, c Solid, op parametric.BooleanOp, o AffineShellCapNativeOptions) (Solid, SplineShellCombineHistory, error) {
	discovered, e := DiscoverAffineShellCapContact(ctx, a, c, o.Discovery)
	if e != nil {
		return Solid{}, SplineShellCombineHistory{}, e
	}
	h := SplineShellCombineHistory{Method: "exact-affine-cap-abutment", AffineCap: discovered, FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(c.faces)), EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(c.edges))}
	if op == parametric.Intersection {
		if e = ctx.Err(); e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		return Solid{}, h, nil
	}
	if op == parametric.Difference {
		for i := range h.FacesA {
			h.FacesA[i] = []int{i}
		}
		for i := range h.EdgesA {
			h.EdgesA[i] = []int{i}
		}
		if e = ctx.Err(); e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		return a, h, nil
	}
	if op != parametric.Union {
		return Solid{}, SplineShellCombineHistory{}, geom.ErrInvalid
	}
	fused, e := discovered.contact.Fuse(ctx, o.Fuse)
	if e != nil {
		return Solid{}, SplineShellCombineHistory{}, e
	}
	h.AffineCapUnion = fused
	appendUnique := func(ids []int, id int) []int {
		for _, old := range ids {
			if old == id {
				return ids
			}
		}
		return append(ids, id)
	}
	for fi, f := range fused.FaceSources() {
		if e = ctx.Err(); e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		if f.Owner == 0 {
			h.FacesA[f.Face] = append(h.FacesA[f.Face], fi)
		} else {
			h.FacesB[f.Face] = append(h.FacesB[f.Face], fi)
		}
		proof := fused.SourceFaceFragments(f.Owner, f.Face)
		for j, u := range proof.cycles[f.Fragment].Uses {
			edge := fused.solid.faces[fi].Outer[j].Edge
			if u.OriginalEdge >= 0 {
				if f.Owner == 0 {
					h.EdgesA[u.OriginalEdge] = appendUnique(h.EdgesA[u.OriginalEdge], edge)
				} else {
					h.EdgesB[u.OriginalEdge] = appendUnique(h.EdgesB[u.OriginalEdge], edge)
				}
			}
			if u.SectionEdge >= 0 {
				h.SectionEdges = appendUnique(h.SectionEdges, edge)
			}
			if u.AuxiliaryEdge >= 0 {
				h.GeneratedEdges = appendUnique(h.GeneratedEdges, edge)
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return Solid{}, SplineShellCombineHistory{}, e
	}
	return fused.solid, h, nil
}
