//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type AlgebraicCapMeshOptions struct {
	Display                    geom.ComposedCurvePolylineOptions
	MaxVertices, MaxPredicates int
}

// CertifiedAlgebraicCapMesh triangulates exactly the represented planar ring.
// CurveVertices index the identical samples in Curves and Mesh. Endpoints are
// shared by topology identity and independently recertified against both model
// curves. SourceCurves retain the unmodified exact algebraic/knot geometry.
// BoundarySquaredDistance certifies source curve/chord correspondence only;
// no closed-half sewing, source fill-distance or source embedding is inferred.
type CertifiedAlgebraicCapMesh struct {
	Mesh                                   mesh.Mesh
	Side                                   int
	SourceEdges                            [2]int
	SourceCurves                           [2]AlgebraicPartitionCurve
	Curves                                 [2]geom.CertifiedComposedCollectionPolyline
	CurveVertices                          [2][]int
	TopologyVertices                       [2]int
	EndpointDisplays                       [2]geom.Vec3
	Boundary                               []int
	ProjectedArea, BoundarySquaredDistance *big.Rat
	ProjectionAxes                         [2]int
	Predicates, ExactOperations            int
}

// TessellateCapCertified keeps every boundary sample unchanged in the final
// cap. Only exact coplanar finite binary64 samples are accepted; a rational
// plane with no compatible selected endpoint display rejects explicitly.
// The source subdivision is never mutated. Adjacent shell meshes must consume
// EndpointDisplays/CurveVertices before this cap can be stitched to them.
func (s AlgebraicLensPlaneSplit) TessellateCapCertified(ctx context.Context, side int, o AlgebraicCapMeshOptions) (CertifiedAlgebraicCapMesh, error) {
	var empty CertifiedAlgebraicCapMesh
	if ctx == nil || o.Display.EndpointPoints != nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	half, e := s.Half(side)
	if e != nil {
		return empty, e
	}
	if len(s.source.curves) != 2 {
		return empty, geom.ErrConvergence
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 512
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.Display.MaxSegments == 0 {
		o.Display.MaxSegments = 4096
	}
	if o.Display.MaxOperations == 0 {
		o.Display.MaxOperations = 8000000
	}
	if o.MaxVertices < 3 || o.MaxVertices > 1024 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 || o.Display.MaxSegments < 1 || o.Display.MaxSegments > 1000000 || o.Display.MaxOperations < 1 || o.Display.MaxOperations > 1000000000 {
		return empty, geom.ErrInvalid
	}
	result := CertifiedAlgebraicCapMesh{Side: side, SourceEdges: half.CapEdges, BoundarySquaredDistance: new(big.Rat)}
	for i, id := range half.CapEdges {
		if id < 0 || id >= len(s.source.edges) {
			return empty, geom.ErrInvalid
		}
		edge := s.source.edges[id]
		if edge.Cut < 0 || edge.Cut >= len(s.source.curves) {
			return empty, geom.ErrConvergence
		}
		result.SourceCurves[i] = cloneAlgebraicPartitionCurve(s.source.curves[edge.Cut])
	}
	first, second := result.SourceCurves[0], result.SourceCurves[1]
	if first.Axis != second.Axis || first.Partition.Vertices != second.Partition.Vertices {
		return empty, geom.ErrConvergence
	}
	for i := 0; i < 3; i++ {
		if first.Partition.Line[i].Cmp(second.Partition.Line[i]) != 0 {
			return empty, geom.ErrConvergence
		}
	}
	axis, other := first.Axis, first.Axis^1
	line := first.Partition.Line
	result.ProjectionAxes = [2]int{axis, 2}
	result.TopologyVertices = first.Partition.Vertices
	for i, id := range result.TopologyVertices {
		if id < 0 || id >= len(s.source.subdivision.vertices) {
			return empty, geom.ErrInvalid
		}
		p := s.source.subdivision.vertices[id].Display
		q := exactBoundPoint(p)
		q[other] = new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Add(line[2], new(big.Rat).Mul(line[axis], q[axis])), line[other]))
		display, e := geom.RepresentedExactPoint(q)
		if e != nil {
			return empty, e
		}
		result.EndpointDisplays[i] = display
		result.Mesh.Vertices = append(result.Mesh.Vertices, display)
	}
	if result.EndpointDisplays[0] == result.EndpointDisplays[1] {
		return empty, geom.ErrDegenerate
	}
	segments := 0
	for i, curve := range result.SourceCurves {
		options := o.Display
		options.EndpointPoints = &result.EndpointDisplays
		options.MaxOperations -= result.ExactOperations
		options.MaxSegments -= segments
		if options.MaxOperations < 1 || options.MaxSegments < 1 {
			return empty, geom.ErrConvergence
		}
		poly, e := curve.Model.TessellateCertified(ctx, geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: options})
		if e != nil {
			return empty, e
		}
		if !poly.Complete || poly.DisplayDegenerate || len(poly.Vertices) < 2 {
			return empty, geom.ErrConvergence
		}
		path := make([]int, len(poly.Vertices))
		path[0], path[len(path)-1] = 0, 1
		for j, v := range poly.Vertices {
			q := exactBoundPoint(v.Point)
			plane := new(big.Rat).Add(line[2], new(big.Rat).Add(new(big.Rat).Mul(line[0], q[0]), new(big.Rat).Mul(line[1], q[1])))
			if plane.Sign() != 0 {
				return empty, geom.ErrUnrepresentable
			}
			if j > 0 && j < len(path)-1 {
				path[j] = len(result.Mesh.Vertices)
				result.Mesh.Vertices = append(result.Mesh.Vertices, v.Point)
			}
		}
		if poly.Vertices[0].Point != result.EndpointDisplays[0] || poly.Vertices[len(path)-1].Point != result.EndpointDisplays[1] {
			return empty, geom.ErrConvergence
		}
		result.Curves[i] = poly
		result.CurveVertices[i] = path
		result.ExactOperations += poly.ExactOperations
		segments += len(poly.Segments)
		error := ratInset(poly.MaximumError)
		error.Mul(error, error)
		if error.Cmp(result.BoundarySquaredDistance) > 0 {
			result.BoundarySquaredDistance.Set(error)
		}
	}
	if len(result.Mesh.Vertices) > o.MaxVertices {
		return empty, geom.ErrConvergence
	}
	for i, path := range result.CurveVertices {
		path = append([]int(nil), path...)
		if half.CapReversed[i] {
			for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
				path[a], path[b] = path[b], path[a]
			}
		}
		if len(result.Boundary) > 0 {
			if result.Boundary[len(result.Boundary)-1] != path[0] {
				return empty, geom.ErrConvergence
			}
			path = path[1:]
		}
		result.Boundary = append(result.Boundary, path...)
	}
	if len(result.Boundary) < 4 || result.Boundary[0] != result.Boundary[len(result.Boundary)-1] {
		return empty, geom.ErrConvergence
	}
	result.Boundary = result.Boundary[:len(result.Boundary)-1]
	proof := partitionFillProof{ctx: ctx, limit: o.MaxPredicates, points: make([][2]*big.Rat, len(result.Mesh.Vertices))}
	for i, p := range result.Mesh.Vertices {
		q := exactBoundPoint(p)
		proof.points[i] = [2]*big.Rat{q[axis], q[2]}
	}
	if e := proof.simple(result.Boundary); e != nil {
		return empty, e
	}
	signed := proof.area(result.Boundary)
	if signed.Sign() == 0 {
		return empty, geom.ErrDegenerate
	}
	ring := append([]int(nil), result.Boundary...)
	reverse := signed.Sign() < 0
	if reverse {
		for a, b := 0, len(ring)-1; a < b; a, b = a+1, b-1 {
			ring[a], ring[b] = ring[b], ring[a]
		}
	}
	tris, e := proof.triangulate(ring)
	if e != nil {
		return empty, e
	}
	sum := new(big.Rat)
	chain := map[[2]int]int{}
	for j, a := range ring {
		partitionChainAdd(chain, a, ring[(j+1)%len(ring)], -1)
	}
	for _, tri := range tris {
		sum.Add(sum, proof.area(tri[:]))
		for j, a := range tri {
			partitionChainAdd(chain, a, tri[(j+1)%3], 1)
		}
		if reverse {
			tri[1], tri[2] = tri[2], tri[1]
		}
		result.Mesh.Triangles = append(result.Mesh.Triangles, tri)
	}
	if len(chain) != 0 || sum.Cmp(new(big.Rat).Abs(signed)) != 0 {
		return empty, geom.ErrConvergence
	}
	// Exact orientation of the projected chart relative to plane (A,B,0).
	orientation := line[other].Sign()
	if axis == 0 {
		orientation = -orientation
	}
	if signed.Sign()*orientation != -side {
		return empty, geom.ErrConvergence
	}
	result.ProjectedArea = sum
	result.Predicates = proof.work
	return result, nil
}
