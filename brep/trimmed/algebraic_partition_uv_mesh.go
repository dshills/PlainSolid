package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// CertifiedAlgebraicUVBoundary retains one exact-source certificate per face
// coedge plus a straight-cut endpoint certificate. UV contains represented
// chart points, not model-space mesh vertices. Fragments have no repeated end.
// No polygon embeddedness, fill distance or source/model mesh claim is made.
type CertifiedAlgebraicUVBoundary struct {
	Partition                 AlgebraicFacePartition
	UV                        []geom.Vec2
	TopologyVertices          map[int]int
	Edges                     map[int][]int // Increasing original model-edge parameter.
	Charts                    map[int]AlgebraicSubdivisionChart
	Certificates              map[int]geom.CertifiedComposedPolyline
	Cut                       [2]int
	Fragments                 [2][]int
	BoundarySquaredDistance   *big.Rat
	ExactOperations, Segments int
}

// TessellatePartitionUVBoundaryCertified uses the display deflection in UV
// units. Each incident coedge independently certifies shared endpoint displays;
// algebraic topology is never recovered from rounded coordinates. Exact source
// restrictions and normalized parameter intervals remain in Charts/Certificates.
// The straight cut has two-sided pointwise error bounded by its endpoint boxes.
func (s AlgebraicSubdivision) TessellatePartitionUVBoundaryCertified(ctx context.Context, face int, line [3]*big.Rat, o AlgebraicBoundaryMeshOptions) (CertifiedAlgebraicUVBoundary, error) {
	var empty CertifiedAlgebraicUVBoundary
	if ctx == nil || o.Display.EndpointPoints != nil {
		return empty, geom.ErrInvalid
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
	if o.Display.Deflection == 0 {
		o.Display.Deflection = 1e-6
	}
	if o.MaxEdges < 1 || o.MaxEdges > 4096 || o.MaxCharts < 1 || o.MaxCharts > 8192 || o.Display.MaxSegments < 1 || o.Display.MaxSegments > 1000000 || o.Display.MaxOperations < 1 || o.Display.MaxOperations > 1000000000 || o.Display.Deflection <= 0 || math.IsNaN(o.Display.Deflection) || math.IsInf(o.Display.Deflection, 0) {
		return empty, geom.ErrInvalid
	}
	partition, e := s.PartitionFaceByAlgebraicUVLine(ctx, face, line, o.Proof)
	if e != nil {
		return empty, e
	}
	ring := s.faces[face].Outer
	if len(ring) > o.MaxEdges || len(ring) > o.MaxCharts {
		return empty, geom.ErrConvergence
	}
	result := CertifiedAlgebraicUVBoundary{Partition: partition, TopologyVertices: map[int]int{}, Edges: map[int][]int{}, Charts: map[int]AlgebraicSubdivisionChart{}, Certificates: map[int]geom.CertifiedComposedPolyline{}, BoundarySquaredDistance: new(big.Rat)}
	seen := map[geom.Vec2]bool{}
	add := func(p geom.Vec3) (int, error) {
		if !p.IsFinite() || p.Z != 0 {
			return 0, geom.ErrConvergence
		}
		uv := geom.Vec2{X: p.X, Y: p.Y}
		if seen[uv] {
			return 0, geom.ErrDegenerate
		}
		seen[uv] = true
		id := len(result.UV)
		result.UV = append(result.UV, uv)
		return id, nil
	}
	for _, use := range ring {
		if _, exists := result.Edges[use.Edge]; exists {
			return empty, geom.ErrConvergence
		}
		chart, e := s.CoedgeChart(ctx, use.Edge, face, o.Proof)
		if e != nil {
			return empty, e
		}
		if chart.UV == nil {
			return empty, geom.ErrConvergence
		}
		edge := s.edges[use.Edge]
		var endpoints [2]geom.Vec3
		for side, vertex := range [2]int{edge.Start, edge.End} {
			id, exists := result.TopologyVertices[vertex]
			if !exists {
				remaining := o.Display.MaxOperations - result.ExactOperations
				if remaining < 1 {
					return empty, geom.ErrConvergence
				}
				tolerance := o.Display.Deflection / 32
				if tolerance == 0 {
					tolerance = o.Display.Deflection
				}
				b, e := chart.UV.EncloseEndpoint(ctx, side, geom.ComposedCurveEndpointOptions{Tolerance: tolerance, ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: o.Display.MaxCoefficientBits, MaxSubdivisions: o.Display.MaxEndpointSubdivisions}})
				result.ExactOperations += b.ExactOperations
				if e != nil {
					return empty, e
				}
				if !b.Converged || !b.HasRepresentative {
					return empty, geom.ErrConvergence
				}
				id, e = add(b.Point)
				if e != nil {
					return empty, e
				}
				result.TopologyVertices[vertex] = id
			}
			p := result.UV[id]
			endpoints[side] = geom.Vec3{X: p.X, Y: p.Y}
		}
		options := o.Display
		options.EndpointPoints = &endpoints
		options.MaxOperations -= result.ExactOperations
		options.MaxSegments -= result.Segments
		if options.MaxOperations < 1 || options.MaxSegments < 1 {
			return empty, geom.ErrConvergence
		}
		poly, e := chart.UV.TessellateCertified(ctx, options)
		if e != nil {
			return empty, e
		}
		if !poly.Complete || len(poly.Vertices) < 2 {
			return empty, geom.ErrConvergence
		}
		path := make([]int, len(poly.Vertices))
		path[0] = result.TopologyVertices[edge.Start]
		path[len(path)-1] = result.TopologyVertices[edge.End]
		if poly.Vertices[0].Point != endpoints[0] || poly.Vertices[len(path)-1].Point != endpoints[1] {
			return empty, geom.ErrConvergence
		}
		for j := 1; j < len(path)-1; j++ {
			path[j], e = add(poly.Vertices[j].Point)
			if e != nil {
				return empty, e
			}
		}
		result.Edges[use.Edge] = path
		result.Charts[use.Edge] = chart
		result.Certificates[use.Edge] = poly
		result.ExactOperations += poly.ExactOperations
		result.Segments += len(poly.Segments)
		errBound := ratInset(poly.MaximumError)
		errBound.Mul(errBound, errBound)
		if errBound.Cmp(result.BoundarySquaredDistance) > 0 {
			result.BoundarySquaredDistance.Set(errBound)
		}
	}
	if result.Segments >= o.Display.MaxSegments {
		return empty, geom.ErrConvergence
	}
	result.Segments++
	for side, vertex := range partition.Vertices {
		id, exists := result.TopologyVertices[vertex]
		if !exists {
			return empty, geom.ErrConvergence
		}
		result.Cut[side] = id
		p := result.UV[id]
		q := [2]*big.Rat{ratInset(p.X), ratInset(p.Y)}
		bounds := partition.CutUV[side]
		if !bounds.HasBounds {
			return empty, geom.ErrConvergence
		}
		sum := new(big.Rat)
		for axis := 0; axis < 2; axis++ {
			a := new(big.Rat).Sub(bounds.Coordinates[axis].Lower, q[axis])
			a.Mul(a, a)
			b := new(big.Rat).Sub(bounds.Coordinates[axis].Upper, q[axis])
			b.Mul(b, b)
			if b.Cmp(a) > 0 {
				a = b
			}
			sum.Add(sum, a)
		}
		if sum.Cmp(result.BoundarySquaredDistance) > 0 {
			result.BoundarySquaredDistance.Set(sum)
		}
	}
	bound := ratInset(o.Display.Deflection)
	bound.Mul(bound, bound)
	if result.BoundarySquaredDistance.Cmp(bound) > 0 {
		return empty, geom.ErrConvergence
	}
	for fi, fragment := range partition.Fragments {
		var path []int
		for _, use := range fragment {
			edge := []int{result.Cut[0], result.Cut[1]}
			if use.BoundaryEdge >= 0 {
				edge = result.Edges[use.BoundaryEdge]
			}
			edge = append([]int(nil), edge...)
			if use.Reversed {
				for a, b := 0, len(edge)-1; a < b; a, b = a+1, b-1 {
					edge[a], edge[b] = edge[b], edge[a]
				}
			}
			if len(path) > 0 {
				if path[len(path)-1] != edge[0] {
					return empty, geom.ErrConvergence
				}
				edge = edge[1:]
			}
			path = append(path, edge...)
		}
		if len(path) < 4 || path[0] != path[len(path)-1] {
			return empty, geom.ErrConvergence
		}
		result.Fragments[fi] = path[:len(path)-1]
	}
	return result, nil
}
