package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffinePolygonCrossingOptions bounds both shell proofs and exhaustive
// cross-owner face clipping cumulatively. Face/edge proof stages keep their
// existing bounds. Defaults:64M operations,256 face pairs; maxima:128M,16384.
type ConvexAffinePolygonCrossingOptions struct {
	// AllowPairedEdgeJunctions accepts exact simultaneous edge/edge nodes only
	// with both source parameters and all four original incident face pairs.
	// Material pairing is independently checked during native reconstruction.
	AllowPairedEdgeJunctions    bool
	Shell                       ConvexAffinePolygonShellOptions
	MaxOperations, MaxFacePairs int
}

// ConvexAffinePolygonSectionVertex retains one normalized canonical original
// edge parameter in [0,1] (the original model Range remains on OriginalShell)
// and both incident original face/coedge occurrences at a transverse junction.
type ConvexAffinePolygonSectionVertex struct {
	Point       [3]*big.Rat
	Owner, Edge int
	Parameter   *big.Rat
	Occurrences [][2]int
	// Owners has one entry normally, or both entries in the opt-in paired mode.
	// Paired nodes set legacy Owner/Edge to -1 and Parameter to nil.
	Owners       []ConvexAffinePolygonSectionOwner
	ContactPairs [][2]int
}

// ConvexAffinePolygonCrossing authenticates complete original section incidence.
// The default produces degree-two cycles. The paired-edge opt-in admits degree
// two/four junctions with all four incident pairs; it does not choose a material
// pairing. Coplanar area and edge-carried contacts remain unsupported.
type ConvexAffinePolygonCrossing struct {
	shells      [2]*ConvexAffinePolygonShell
	source      *AffineShellPairedImprint
	points      [][3]*big.Rat
	contacts    []*intersect.AffinePolygonContact
	nodes       []ConvexAffinePolygonSectionVertex
	work        int
	allowPaired bool
}

func (r *ConvexAffinePolygonCrossing) CompleteFacePairs() int {
	if r == nil {
		return 0
	}
	return len(r.contacts)
}
func (r *ConvexAffinePolygonCrossing) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}
func (r *ConvexAffinePolygonCrossing) SectionEdges() []AffineShellImprintEdge {
	if r == nil {
		return nil
	}
	return r.source.Edges()
}
func (r *ConvexAffinePolygonCrossing) OriginalShell(owner int) (*ConvexAffinePolygonShell, error) {
	if r == nil || owner < 0 || owner > 1 || r.shells[owner] == nil {
		return nil, geom.ErrInvalid
	}
	return r.shells[owner], nil
}
func (r *ConvexAffinePolygonCrossing) SectionVertices() []ConvexAffinePolygonSectionVertex {
	if r == nil {
		return nil
	}
	out := append([]ConvexAffinePolygonSectionVertex(nil), r.nodes...)
	for i := range out {
		for k := range out[i].Point {
			out[i].Point[k] = new(big.Rat).Set(out[i].Point[k])
		}
		if out[i].Parameter != nil {
			out[i].Parameter = new(big.Rat).Set(out[i].Parameter)
		}
		out[i].Owners = append([]ConvexAffinePolygonSectionOwner(nil), out[i].Owners...)
		for j := range out[i].Owners {
			out[i].Owners[j].Parameter = new(big.Rat).Set(out[i].Owners[j].Parameter)
			out[i].Owners[j].Occurrences = append([][2]int(nil), out[i].Owners[j].Occurrences...)
		}
		out[i].ContactPairs = append([][2]int(nil), out[i].ContactPairs...)
		out[i].Occurrences = append([][2]int(nil), out[i].Occurrences...)
	}
	return out
}

// PairContact is the complete finite clipped contact of the two original faces.
func (r *ConvexAffinePolygonCrossing) PairContact(a, b int) (*intersect.AffinePolygonContact, error) {
	if r == nil || r.source == nil || a < 0 || b < 0 || a >= len(r.source.sources[0].faces) || b >= len(r.source.sources[1].faces) {
		return nil, geom.ErrInvalid
	}
	index := a*len(r.source.sources[1].faces) + b
	if index >= len(r.contacts) {
		return nil, geom.ErrInvalid
	}
	return r.contacts[index], nil
}
func CertifyConvexAffinePolygonCrossing(ctx context.Context, a, c Solid, o ConvexAffinePolygonCrossingOptions) (*ConvexAffinePolygonCrossing, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxFacePairs == 0 {
		o.MaxFacePairs = 256
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxFacePairs < 1 || o.MaxFacePairs > 16384 || o.Shell.MaxOperations < 0 || o.Shell.MaxOperations > 128000000 {
		return nil, geom.ErrInvalid
	}
	if len(c.faces) == 0 || len(a.faces) > o.MaxFacePairs/len(c.faces) {
		return nil, geom.ErrConvergence
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: 16384}
	r := &ConvexAffinePolygonCrossing{allowPaired: o.AllowPairedEdgeJunctions, source: &AffineShellPairedImprint{sources: [2]Solid{a, c}}}
	for owner, s := range r.source.sources {
		if b.work >= b.max {
			return nil, geom.ErrConvergence
		}
		so := o.Shell
		if so.MaxOperations == 0 || so.MaxOperations > b.max-b.work {
			so.MaxOperations = b.max - b.work
		}
		proof, e := CertifyConvexAffinePolygonShell(ctx, s, so)
		if e != nil {
			return nil, e
		}
		r.shells[owner] = proof
		b.work += proof.ExactOperations()
		for _, f := range s.faces {
			p, e := exactFaceSupportPatch(f, o.Shell.Face.Patch)
			if e != nil {
				return nil, e
			}
			r.source.charts[owner] = append(r.source.charts[owner], splineEmbeddingFace{patch: p})
		}
	}
	ids := map[[3]string]int{}
	degree := []int{}
	segments := map[[2]int]bool{}
	for i, fa := range a.faces {
		for j, fc := range c.faces {
			if b.work >= b.max {
				return nil, geom.ErrConvergence
			}
			pa, e := algebraicShellTracePatch(fa, r.shells[0].queries[i].chart.Domain())
			if e != nil {
				return nil, e
			}
			pc, e := algebraicShellTracePatch(fc, r.shells[1].queries[j].chart.Domain())
			if e != nil {
				return nil, e
			}
			loops := [2][][2]*big.Rat{affinePolygonFaceLoop(fa), affinePolygonFaceLoop(fc)}
			pair, e := intersect.IntersectConvexAffinePatches(ctx, pa, pc, loops, o.Shell.Face.Patch, intersect.AffinePolygonContactOptions{MaxEdges: len(loops[0]) + len(loops[1]), MaxOperations: b.max - b.work, MaxCoefficientBits: 16384})
			if e != nil {
				return nil, e
			}
			r.contacts = append(r.contacts, pair)
			contact := pair.Contact()
			b.work += contact.ExactOperations()
			kind, e := contact.Kind()
			if e != nil {
				return nil, e
			}
			if kind == intersect.AffinePatchContactEmpty {
				continue
			}
			if kind != intersect.AffinePatchContactSegment && !(r.allowPaired && kind == intersect.AffinePatchContactPoint) {
				return nil, fmt.Errorf("polygon crossing pair %d/%d is not transverse segment: %w", i, j, geom.ErrDomain)
			}
			points, e := contact.Vertices()
			if e != nil {
				return nil, e
			}
			edges, e := contact.Edges()
			if e != nil {
				return nil, e
			}
			pointOnly := kind == intersect.AffinePatchContactPoint
			if pointOnly {
				if len(points) != 1 || len(edges) != 0 {
					return nil, geom.ErrDomain
				}
			} else {
				if len(points) != 2 || len(edges) != 1 {
					return nil, geom.ErrDomain
				}
				sides := pair.EdgeTrimSides()
				if len(sides) != 1 || len(sides[0][0]) != 0 || len(sides[0][1]) != 0 {
					return nil, geom.ErrDomain
				}
			}
			vertices := pair.VertexTrimSides()
			ends := [2]int{}
			for k, p := range points {
				if pointOnly && len(vertices[k][0])+len(vertices[k][1]) != 2 {
					return nil, geom.ErrDomain
				}
				id, e := r.recordSectionPoint(b, ids, [2]int{i, j}, p, vertices[k])
				if e != nil {
					return nil, e
				}
				for len(degree) < len(r.nodes) {
					degree = append(degree, 0)
				}
				ends[k] = id
				if !pointOnly {
					degree[id]++
				}
			}
			if pointOnly {
				continue
			}
			ordered := ends
			if ends[0] > ends[1] {
				ends[0], ends[1] = ends[1], ends[0]
			}
			if ends[0] == ends[1] || segments[ends] {
				return nil, geom.ErrDomain
			}
			segments[ends] = true
			r.source.edges = append(r.source.edges, AffineShellImprintEdge{Faces: [2]int{i, j}, Vertices: ordered, UV: [2][2][2]*big.Rat{{points[0].A, points[1].A}, {points[0].B, points[1].B}}})
		}
	}
	if len(segments) < 3 {
		return nil, geom.ErrDomain
	}
	if e := r.finishSectionNodes(b, degree); e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r.work = b.work
	return r, nil
}
