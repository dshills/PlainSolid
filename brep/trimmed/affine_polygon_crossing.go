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
	Shell                       ConvexAffinePolygonShellOptions
	MaxOperations, MaxFacePairs int
}

// ConvexAffinePolygonCrossing authenticates complete transverse closed section
// cycles on two original convex polygon shells. Coplanar area, point-only,
// edge-carried and simultaneous original boundary contacts remain unsupported.
// ConvexAffinePolygonSectionVertex retains one normalized canonical original
// edge parameter in [0,1] (the original model Range remains on OriginalShell)
// and both incident original face/coedge occurrences at a transverse junction.
type ConvexAffinePolygonSectionVertex struct {
	Point       [3]*big.Rat
	Owner, Edge int
	Parameter   *big.Rat
	Occurrences [][2]int
}
type ConvexAffinePolygonCrossing struct {
	shells   [2]*ConvexAffinePolygonShell
	source   *AffineShellPairedImprint
	points   [][3]*big.Rat
	contacts []*intersect.AffinePolygonContact
	nodes    []ConvexAffinePolygonSectionVertex
	work     int
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
		out[i].Parameter = new(big.Rat).Set(out[i].Parameter)
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
	r := &ConvexAffinePolygonCrossing{source: &AffineShellPairedImprint{sources: [2]Solid{a, c}}}
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
	key := func(p [3]*big.Rat) [3]string { return [3]string{p[0].RatString(), p[1].RatString(), p[2].RatString()} }
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
			if kind != intersect.AffinePatchContactSegment {
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
			if len(points) != 2 || len(edges) != 1 {
				return nil, geom.ErrDomain
			}
			sides := pair.EdgeTrimSides()
			if len(sides) != 1 || len(sides[0][0]) != 0 || len(sides[0][1]) != 0 {
				return nil, geom.ErrDomain
			}
			vertices := pair.VertexTrimSides()
			ends := [2]int{}
			for k, p := range points {
				if len(vertices[k][0])+len(vertices[k][1]) != 1 {
					return nil, fmt.Errorf("polygon crossing simultaneous/vertex endpoint: %w", geom.ErrDomain)
				}

				owner, face, uv := 0, i, p.A
				if len(vertices[k][1]) == 1 {
					owner, face, uv = 1, j, p.B
				}
				local := vertices[k][owner][0]
				use := r.source.sources[owner].faces[face].Outer[local]
				start, end := [2]*big.Rat{ratInset(use.UVStart.X), ratInset(use.UVStart.Y)}, [2]*big.Rat{ratInset(use.UVEnd.X), ratInset(use.UVEnd.Y)}
				axis := 0
				if start[0].Cmp(end[0]) == 0 {
					axis = 1
				}
				parameter := b.op('/', b.op('-', uv[axis], start[axis]), b.op('-', end[axis], start[axis]))
				if use.Reversed {
					parameter = b.op('-', big.NewRat(1, 1), parameter)
				}
				if b.err != nil {
					return nil, b.err
				}
				if parameter.Sign() <= 0 || parameter.Cmp(big.NewRat(1, 1)) >= 0 {
					return nil, geom.ErrDomain
				}
				token := key(p.Point)
				id, ok := ids[token]
				if !ok {
					id = len(r.points)
					ids[token] = id
					r.points = append(r.points, p.Point)
					degree = append(degree, 0)
					r.nodes = append(r.nodes, ConvexAffinePolygonSectionVertex{Point: p.Point, Owner: owner, Edge: use.Edge, Parameter: parameter})
				} else {
					node := r.nodes[id]
					if node.Owner != owner || node.Edge != use.Edge || node.Parameter.Cmp(parameter) != 0 {
						return nil, geom.ErrDomain
					}
				}
				r.nodes[id].Occurrences = append(r.nodes[id].Occurrences, [2]int{face, local})
				ends[k] = id
				degree[id]++
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
	for vi, d := range degree {
		occurrences := r.nodes[vi].Occurrences
		if d != 2 || len(occurrences) != 2 || occurrences[0][0] == occurrences[1][0] {
			return nil, geom.ErrDomain
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r.work = b.work
	return r, nil
}
