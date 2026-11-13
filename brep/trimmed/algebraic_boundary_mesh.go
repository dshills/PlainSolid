package trimmed

import (
	"context"
	"fmt"
	"plainsolid/geom"
)

// AlgebraicBoundaryMeshOptions bounds the network size and total tessellation
// work. Proof limits apply separately to each of at most MaxCharts coedges.
// Endpoint overrides are deliberately unavailable: subdivision identities own
// the common displays, and every chord certificate rechecks their errors.
type AlgebraicBoundaryMeshOptions struct {
	Proof               AlgebraicSeamOptions
	Display             geom.ComposedCurvePolylineOptions
	MaxEdges, MaxCharts int
}

type AlgebraicBoundaryCoedge struct {
	Face, Loop, Use, OriginalUse, Edge int
	Reverse                            bool // Physical traversal includes the face orientation.
	Vertices                           []int
	Chart                              AlgebraicSubdivisionChart
}

type CertifiedAlgebraicBoundaryMesh struct {
	Vertices                  []geom.Vec3
	TopologyVertices          []int
	TopologyEdges             []AlgebraicSubdivisionEdge
	Edges                     [][]int // Increasing original model parameter; indexed by subdivision edge.
	Certificates              []geom.CertifiedComposedCollectionPolyline
	Coedges                   []AlgebraicBoundaryCoedge
	ExactOperations, Segments int
}

// TessellateBoundaryCertified builds one shared polyline per subdivided edge.
// CoedgeChart independently proves every owner agrees continuously with that
// model curve, so the same parameterized chord certificate applies to all
// owners. Exact root identities, chart restrictions and original-use ancestry
// are retained. Distinct displayed vertices must remain distinct; no tolerance
// welding or partial network is returned. This is a boundary approximation,
// not a face-fill, embedding, or solid tessellation certificate.
func (s AlgebraicSubdivision) TessellateBoundaryCertified(ctx context.Context, o AlgebraicBoundaryMeshOptions) (CertifiedAlgebraicBoundaryMesh, error) {
	var empty CertifiedAlgebraicBoundaryMesh
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 256
	}
	if o.MaxCharts == 0 {
		o.MaxCharts = 512
	}
	if o.Display.MaxSegments == 0 {
		o.Display.MaxSegments = 4096
	}
	if o.Display.MaxOperations == 0 {
		o.Display.MaxOperations = 8000000
	}
	if o.MaxEdges < 1 || o.MaxEdges > 4096 || o.MaxCharts < 1 || o.MaxCharts > 8192 || o.Display.MaxSegments < 1 || o.Display.MaxSegments > 1000000 || o.Display.MaxOperations < 1 || o.Display.MaxOperations > 1000000000 || o.Display.EndpointPoints != nil {
		return empty, geom.ErrInvalid
	}
	if len(s.edges) == 0 || len(s.edges) > o.MaxEdges {
		return empty, geom.ErrConvergence
	}
	if err := s.checkIncidence(ctx); err != nil {
		return empty, err
	}
	r := CertifiedAlgebraicBoundaryMesh{TopologyVertices: make([]int, len(s.vertices)), TopologyEdges: s.Edges(), Edges: make([][]int, len(s.edges)), Certificates: make([]geom.CertifiedComposedCollectionPolyline, len(s.edges))}
	seen := map[geom.Vec3]int{}
	add := func(p geom.Vec3) (int, error) {
		if !p.IsFinite() {
			return 0, geom.ErrInvalid
		}
		if _, ok := seen[p]; ok {
			return 0, geom.ErrDegenerate
		}
		id := len(r.Vertices)
		seen[p] = id
		r.Vertices = append(r.Vertices, p)
		return id, nil
	}
	for i, v := range s.vertices {
		id, e := add(v.Display)
		if e != nil {
			return empty, e
		}
		r.TopologyVertices[i] = id
	}
	for ei, edge := range s.edges {
		options := o.Display
		options.MaxSegments -= r.Segments
		options.MaxOperations -= r.ExactOperations
		if options.MaxSegments < 1 || options.MaxOperations < 1 {
			return empty, geom.ErrConvergence
		}
		line, e := s.TessellateModelEdgeCertified(ctx, ei, o.Proof, geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: options})
		if e != nil {
			return empty, fmt.Errorf("boundary edge %d (%d prior segments, %d prior operations): %w", ei, r.Segments, r.ExactOperations, e)
		}
		if !line.Complete || line.DisplayDegenerate || len(line.Vertices) < 2 {
			return empty, geom.ErrConvergence
		}
		path := make([]int, len(line.Vertices))
		path[0] = r.TopologyVertices[edge.Start]
		path[len(path)-1] = r.TopologyVertices[edge.End]
		if line.Vertices[0].Point != r.Vertices[path[0]] || line.Vertices[len(path)-1].Point != r.Vertices[path[len(path)-1]] {
			return empty, geom.ErrConvergence
		}
		for j := 1; j < len(path)-1; j++ {
			id, e := add(line.Vertices[j].Point)
			if e != nil {
				return empty, e
			}
			path[j] = id
		}
		r.Edges[ei] = path
		r.Certificates[ei] = line
		r.ExactOperations += line.ExactOperations
		r.Segments += len(line.Segments)
	}
	counts := make([]int, len(s.edges))
	orientation := make([]int, len(s.edges))
	for fi, face := range s.faces {
		for li, loop := range append([][]AlgebraicSubdivisionUse{face.Outer}, face.Holes...) {
			for ui, use := range loop {
				if len(r.Coedges) >= o.MaxCharts {
					return empty, geom.ErrConvergence
				}
				chart, e := s.CoedgeChart(ctx, use.Edge, fi, o.Proof)
				if e != nil {
					return empty, fmt.Errorf("boundary edge %d face %d: %w", use.Edge, fi, e)
				}
				if chart.UV == nil || !chart.Model.Complete || chart.Reverse != use.Reversed {
					return empty, geom.ErrConvergence
				}
				reverse := use.Reversed != face.Reversed
				path := append([]int(nil), r.Edges[use.Edge]...)
				if reverse {
					for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
						path[a], path[b] = path[b], path[a]
					}
				}
				counts[use.Edge]++
				if reverse {
					orientation[use.Edge]--
				} else {
					orientation[use.Edge]++
				}
				r.Coedges = append(r.Coedges, AlgebraicBoundaryCoedge{fi, li, ui, use.OriginalUse, use.Edge, reverse, path, chart})
			}
		}
	}
	for i, n := range counts {
		if n != 2 || orientation[i] != 0 {
			return empty, geom.ErrConvergence
		}
	}
	return r, nil
}
