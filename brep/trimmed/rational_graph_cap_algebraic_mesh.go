package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// AlgebraicGraphCapMeshOptions bounds independent chord, polygon and
// triangulation proofs. Display.EndpointPoints must be nil because adjacent
// uses derive one shared represented point from their exact vertex.
type AlgebraicGraphCapMeshOptions struct {
	Display       geom.ComposedCurvePolylineOptions
	Polygon       trimvalidate.Options
	MaxPredicates int
}

// AlgebraicGraphCapBoundaryMesh holds oriented, shared display paths in
// original clip wire/use order. Segment bounds belong to the authoritative
// model curves, including exact algebraic endpoints and rounding.
type AlgebraicGraphCapBoundaryMesh struct {
	Vertices             []geom.Vec3
	Paths                [][][]int
	Chords               [][]geom.CertifiedComposedPolyline
	MaximumBoundaryError float64
	Operations, Splits   int
}

// CertifiedAlgebraicGraphCapMesh triangulates the exact represented polygon
// and holes on the original cap plane. The boundary correspondence provides
// a two-sided closed-fill distance bound by planar winding. This is one open
// planar cap, with no claim about a companion shell mesh.
type CertifiedAlgebraicGraphCapMesh struct {
	Mesh             mesh.Mesh
	Boundary         AlgebraicGraphCapBoundaryMesh
	Polygon          trimvalidate.Report
	Regions          []RationalGraphShellRegion
	Predicates       int
	MaximumFillError float64
}

func (c AlgebraicGraphCapClip) TessellateCertified(ctx context.Context, o AlgebraicGraphCapMeshOptions) (CertifiedAlgebraicGraphCapMesh, error) {
	var zero CertifiedAlgebraicGraphCapMesh
	if ctx == nil || c.regions == nil || o.Display.EndpointPoints != nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 2000000
	}
	if o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return zero, geom.ErrInvalid
	}
	poly, e := o.Polygon.WithDefaults()
	if e != nil {
		return zero, e
	}
	boundary, e := c.sampleCapBoundary(ctx, o.Display)
	if e != nil {
		return zero, e
	}
	out := CertifiedAlgebraicGraphCapMesh{Boundary: boundary, MaximumFillError: boundary.MaximumBoundaryError}
	out.Mesh.Vertices = append([]geom.Vec3(nil), boundary.Vertices...)
	axes := orientedRationalGraphProjection(c.regions.plane)
	points := make([][2]*big.Rat, len(boundary.Vertices))
	for i, model := range boundary.Vertices {
		p := exactBoundPoint(model)
		points[i] = [2]*big.Rat{p[axes[0]], p[axes[1]]}
	}
	if len(points) > poly.MaxEdges {
		return zero, geom.ErrConvergence
	}
	rings := make([]trimvalidate.Loop, len(boundary.Paths))
	ids := make([][]int, len(rings))
	for i, wire := range boundary.Paths {
		for _, path := range wire {
			ids[i] = append(ids[i], path[:len(path)-1]...)
		}
		if len(ids[i]) < 3 {
			return zero, geom.ErrDegenerate
		}
		for j, a := range ids[i] {
			b := ids[i][(j+1)%len(ids[i])]
			x, _ := points[a][0].Float64()
			y, _ := points[a][1].Float64()
			u, _ := points[b][0].Float64()
			v, _ := points[b][1].Float64()
			rings[i] = append(rings[i], trimvalidate.Segment{Start: geom.Vec2{X: x, Y: y}, End: geom.Vec2{X: u, Y: v}})
		}
	}
	_, placement, proof, e := trimvalidate.CertifyRingForest(ctx, rings, poly)
	if e != nil {
		return zero, e
	}
	if proof.Status != trimvalidate.Certified || len(placement) != len(rings) {
		return zero, geom.ErrConvergence
	}
	out.Polygon = proof
	index := make(map[int]int)
	for i, ring := range placement {
		want := 1
		if ring.Depth%2 != 0 {
			want = -1
		}
		if want != ring.Orientation {
			return zero, geom.ErrConvergence
		}
		if want > 0 {
			index[i] = len(out.Regions)
			out.Regions = append(out.Regions, RationalGraphShellRegion{Outer: i})
		}
	}
	for i, ring := range placement {
		if ring.Depth%2 != 0 {
			parent, ok := index[ring.Parent]
			if !ok {
				return zero, geom.ErrConvergence
			}
			out.Regions[parent].Holes = append(out.Regions[parent].Holes, i)
		}
	}
	for _, region := range out.Regions {
		loops := [][]int{ids[region.Outer]}
		for _, i := range region.Holes {
			loops = append(loops, ids[i])
		}
		remaining := o.MaxPredicates - out.Predicates
		if remaining < 1 {
			return zero, geom.ErrConvergence
		}
		triangles, predicates, e := exactHoledFootprintTriangulation(ctx, points, loops, remaining)
		if e != nil {
			return zero, e
		}
		out.Predicates += predicates
		out.Mesh.Triangles = append(out.Mesh.Triangles, triangles...)
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

func (c AlgebraicGraphCapClip) sampleCapBoundary(ctx context.Context, o geom.ComposedCurvePolylineOptions) (AlgebraicGraphCapBoundaryMesh, error) {
	var zero AlgebraicGraphCapBoundaryMesh
	if o.Deflection == 0 {
		o.Deflection = 1e-4
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxEndpointSubdivisions == 0 {
		o.MaxEndpointSubdivisions = 8192
	}
	if o.Deflection <= 0 || math.IsNaN(o.Deflection) || math.IsInf(o.Deflection, 0) ||
		o.MaxOperations < 1 || o.MaxSegments < 1 || o.MaxEndpointSubdivisions < 1 {
		return zero, geom.ErrInvalid
	}
	out := AlgebraicGraphCapBoundaryMesh{Paths: make([][][]int, len(c.wires)), Chords: make([][]geom.CertifiedComposedPolyline, len(c.wires))}
	segments := 0
	plane := c.regions.plane
	inline := func(x geom.Vec3) bool {
		return x.IsFinite() && plane.dot(exactBoundPoint(x)).Cmp(plane.offset) == 0
	}
	for i, wire := range c.wires {
		junctions := make([]int, len(wire))
		for j, use := range wire {
			work, splits := o.MaxOperations-out.Operations, o.MaxEndpointSubdivisions-out.Splits
			if work < 1 || splits < 1 {
				return zero, geom.ErrConvergence
			}
			side := 0
			if use.Reversed {
				side = 1
			}
			p, e := use.Cell.EncloseEndpoint(ctx, side, geom.ComposedCurveEndpointOptions{
				ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{
					MaxOperations: work, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: splits,
				}, Tolerance: o.Deflection / 64,
			})
			if e != nil {
				return zero, e
			}
			if !p.Converged || !p.HasRepresentative || !inline(p.Point) {
				return zero, geom.ErrUnrepresentable
			}
			out.Operations += p.ExactOperations
			out.Splits += p.Subdivisions
			junctions[j] = len(out.Vertices)
			out.Vertices = append(out.Vertices, p.Point)
		}
		for j, use := range wire {
			a, b := junctions[j], junctions[(j+1)%len(wire)]
			if use.Reversed {
				a, b = b, a
			}
			f := o
			f.EndpointPoints = &[2]geom.Vec3{out.Vertices[a], out.Vertices[b]}
			f.MaxOperations -= out.Operations
			f.MaxEndpointSubdivisions -= out.Splits
			f.MaxSegments -= segments
			if f.MaxOperations < 1 || f.MaxEndpointSubdivisions < 1 || f.MaxSegments < 1 {
				return zero, geom.ErrConvergence
			}
			q, e := use.Cell.TessellateCertified(ctx, f)
			if e != nil {
				return zero, e
			}
			if !q.Complete || len(q.Vertices) < 2 ||
				q.Vertices[0].Point != out.Vertices[a] || q.Vertices[len(q.Vertices)-1].Point != out.Vertices[b] {
				return zero, geom.ErrConvergence
			}
			path := make([]int, len(q.Vertices))
			path[0], path[len(path)-1] = a, b
			for k := 1; k < len(path)-1; k++ {
				p := q.Vertices[k].Point
				if !inline(p) {
					return zero, geom.ErrUnrepresentable
				}
				path[k] = len(out.Vertices)
				out.Vertices = append(out.Vertices, p)
			}
			out.Operations += q.ExactOperations
			out.Splits += q.EndpointSubdivisions
			segments += len(q.Segments)
			if out.Splits > o.MaxEndpointSubdivisions || segments > o.MaxSegments || out.Operations > o.MaxOperations {
				return zero, geom.ErrConvergence
			}
			out.MaximumBoundaryError = max(out.MaximumBoundaryError, q.MaximumError)
			if use.Reversed {
				for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
					path[l], path[r] = path[r], path[l]
				}
			}
			out.Paths[i] = append(out.Paths[i], path)
			out.Chords[i] = append(out.Chords[i], q)
		}
	}
	return out, nil
}
