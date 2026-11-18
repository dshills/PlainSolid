//go:build research

package trimmed

import (
	"context"
	"math"

	"plainsolid/geom"
)

// AlgebraicGraphStockBoundaryOptions bounds the shared exact-curve
// approximation and independent display edge contact test.
type AlgebraicGraphStockBoundaryOptions struct {
	Section           RationalGraphShellSectionOptions
	Exact             geom.ExactCurvePlaneEventOptions
	Display           geom.ComposedCollectionPolylineOptions
	EndpointTolerance float64
	MaxPairs          int
}

// CertifiedAlgebraicGraphStockBoundary provides one represented network for
// all incidences of the exact shell. Edges follow the canonical model curve
// order of Indexed. The certificate concerns the shared edge network, not
// triangulation or source fill distances on the stock faces.
type CertifiedAlgebraicGraphStockBoundary struct {
	Indexed              AlgebraicGraphShellBoundary
	Vertices             []geom.Vec3
	Edges                [][]int
	Curves               []geom.CertifiedComposedCollectionPolyline
	MaximumBoundaryError float64
	ExactOperations      int
	PairChecks           int
}

func (c AlgebraicGraphCapPair) TessellateStockBoundaryCertified(ctx context.Context, o AlgebraicGraphStockBoundaryOptions) (CertifiedAlgebraicGraphStockBoundary, error) {
	var zero CertifiedAlgebraicGraphStockBoundary
	if ctx == nil || o.Display.EndpointPoints != nil || o.Display.JunctionPoints != nil {
		return zero, geom.ErrInvalid
	}
	d := &o.Display
	if d.Deflection == 0 {
		d.Deflection = 1e-4
	}
	if d.MaxOperations == 0 {
		d.MaxOperations = 8000000
	}
	if d.MaxSegments == 0 {
		d.MaxSegments = 4096
	}
	if d.MaxEndpointSubdivisions == 0 {
		d.MaxEndpointSubdivisions = 8192
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 2000000
	}
	if d.Deflection <= 0 || math.IsInf(d.Deflection, 0) || math.IsNaN(d.Deflection) ||
		d.MaxOperations < 1 || d.MaxSegments < 1 || d.MaxEndpointSubdivisions < 1 || o.MaxPairs < 1 || o.MaxPairs > 16000000 {
		return zero, geom.ErrInvalid
	}
	tolerance := d.Deflection / 64
	if o.EndpointTolerance != 0 {
		if !finite(o.EndpointTolerance) || o.EndpointTolerance < 0 {
			return zero, geom.ErrInvalid
		}
		tolerance = min(tolerance, o.EndpointTolerance)
	}
	if tolerance <= 0 {
		return zero, geom.ErrUnrepresentable
	}
	topology, curves, e := c.ExactModelBoundary(ctx, o.Section, o.Exact)
	if e != nil {
		return zero, e
	}
	out := CertifiedAlgebraicGraphStockBoundary{Indexed: topology, Vertices: make([]geom.Vec3, topology.Vertices)}
	endpoints := make([]bool, topology.Vertices)
	subdivisions, segments := 0, 0
	for i, curve := range curves {
		edge := topology.Edges[i]
		vs := [2]int{edge.Start, edge.End}
		if curve.Reversed {
			vs[0], vs[1] = vs[1], vs[0]
		}
		for k, v := range vs {
			if endpoints[v] {
				continue
			}
			work, remaining := d.MaxOperations-out.ExactOperations, d.MaxEndpointSubdivisions-subdivisions
			if work < 1 || remaining < 1 {
				return zero, geom.ErrConvergence
			}
			cell := curve.Cells.Cells[k*(len(curve.Cells.Cells)-1)]
			b, e := cell.EncloseEndpoint(ctx, k, geom.ComposedCurveEndpointOptions{
				ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: work, MaxSubdivisions: remaining, MaxCoefficientBits: d.MaxCoefficientBits},
				Tolerance:                   d.Deflection / 64,
			})
			if e != nil {
				return zero, e
			}
			if !b.Converged || !b.HasRepresentative {
				return zero, geom.ErrUnrepresentable
			}
			out.ExactOperations += b.ExactOperations
			subdivisions += b.Subdivisions
			out.Vertices[v], endpoints[v] = b.Point, true
		}
		display := *d
		display.EndpointPoints = &[2]geom.Vec3{out.Vertices[vs[0]], out.Vertices[vs[1]]}
		display.MaxOperations -= out.ExactOperations
		display.MaxSegments -= segments
		display.MaxEndpointSubdivisions -= subdivisions
		if display.MaxOperations < 1 || display.MaxSegments < 1 || display.MaxEndpointSubdivisions < 1 {
			return zero, geom.ErrConvergence
		}
		path, e := curve.Cells.TessellateCertified(ctx, display)
		if e != nil {
			return zero, e
		}
		if !path.Complete || path.DisplayDegenerate || len(path.Vertices) < 2 ||
			path.Vertices[0].Point != out.Vertices[vs[0]] || path.Vertices[len(path.Vertices)-1].Point != out.Vertices[vs[1]] {
			return zero, geom.ErrConvergence
		}
		row := make([]int, len(path.Vertices))
		row[0], row[len(row)-1] = vs[0], vs[1]
		for j := 1; j < len(row)-1; j++ {
			row[j] = len(out.Vertices)
			out.Vertices = append(out.Vertices, path.Vertices[j].Point)
		}
		if curve.Reversed {
			for j, k := 0, len(row)-1; j < k; j, k = j+1, k-1 {
				row[j], row[k] = row[k], row[j]
			}
		}
		out.Edges = append(out.Edges, row)
		out.Curves = append(out.Curves, path)
		out.MaximumBoundaryError = max(out.MaximumBoundaryError, path.MaximumError)
		out.ExactOperations += path.ExactOperations
		subdivisions += path.EndpointSubdivisions
		segments += len(path.Segments)
		if subdivisions > d.MaxEndpointSubdivisions || segments > d.MaxSegments {
			return zero, geom.ErrConvergence
		}
	}
	out.PairChecks, e = checkBoundaryPolylineEmbedding(ctx, out.Vertices, out.Edges, o.MaxPairs)
	if e != nil {
		return zero, e
	}
	return out, nil
}
