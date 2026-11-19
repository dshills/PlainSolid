//go:build research

package trimmed

import (
	"context"

	"plainsolid/geom"
)

// SplineGraphSectionPolyline is a closed display approximation of an
// exact four-arc section. Edges use shared represented vertices and
// retain their originating conic arc and spline knot cell.
type SplineGraphSectionPolyline struct {
	Vertices     []geom.Vec3
	Segments     []SplineGraphSectionDisplayEdge
	MaximumError float64
}

type SplineGraphSectionDisplayEdge struct {
	Vertices   [2]int
	Arc, Cell  int
	ErrorUpper float64
}

// TessellateCertified bounds the model-space chord error on each original
// conic arc. It joins adjacent algebraic knot cells and all four conic
// junctions at one represented point, rechecking each incident cell against it.
func (s SplineGraphEllipseSection) TessellateCertified(ctx context.Context, o geom.ComposedCurvePolylineOptions) (SplineGraphSectionPolyline, error) {
	var empty SplineGraphSectionPolyline
	if ctx == nil || len(s.displayCells) != 4 || o.EndpointPoints != nil {
		return empty, geom.ErrInvalid
	}
	if o.Deflection == 0 {
		o.Deflection = 1e-6
	}
	if !finite(o.Deflection) || o.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxEndpointSubdivisions == 0 {
		o.MaxEndpointSubdivisions = 8192
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	type piece struct {
		arc, cell int
		curve     *geom.ExactComposedCurveCell
	}
	var all []piece
	for i, arc := range s.displayCells {
		for j, c := range arc {
			all = append(all, piece{i, j, c})
		}
	}
	ends := make([]geom.Vec3, len(all))
	for i, p := range all {
		if o.MaxOperations < 1 || o.MaxEndpointSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		q, e := p.curve.EncloseEndpoint(ctx, 0, geom.ComposedCurveEndpointOptions{
			Tolerance: o.Deflection / 8,
			ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{
				MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxEndpointSubdivisions,
			},
		})
		if e != nil {
			return empty, e
		}
		if !q.Converged || !q.HasRepresentative {
			return empty, geom.ErrConvergence
		}
		ends[i] = q.Point
		o.MaxOperations -= q.ExactOperations
		o.MaxEndpointSubdivisions -= q.Subdivisions
	}
	// Preassigned junction IDs make every knot and conic corner shared.
	out := SplineGraphSectionPolyline{Vertices: append([]geom.Vec3(nil), ends...)}
	for i, p := range all {
		if o.MaxOperations < 1 || o.MaxEndpointSubdivisions < 1 || o.MaxSegments < 1 {
			return empty, geom.ErrConvergence
		}
		last := (i + 1) % len(all)
		options := o
		options.EndpointPoints = &[2]geom.Vec3{ends[i], ends[last]}
		q, e := p.curve.TessellateCertified(ctx, options)
		if e != nil {
			return empty, e
		}
		if !q.Complete || len(q.Vertices) < 2 || q.Vertices[0].Point != ends[i] || q.Vertices[len(q.Vertices)-1].Point != ends[last] {
			return empty, geom.ErrConvergence
		}
		o.MaxOperations -= q.ExactOperations
		o.MaxEndpointSubdivisions -= q.EndpointSubdivisions
		o.MaxSegments -= len(q.Segments)
		ids := make([]int, len(q.Vertices))
		ids[0], ids[len(ids)-1] = i, last
		for j := 1; j+1 < len(ids); j++ {
			ids[j] = len(out.Vertices)
			out.Vertices = append(out.Vertices, q.Vertices[j].Point)
		}
		for _, x := range q.Segments {
			a, b := ids[x.Vertices[0]], ids[x.Vertices[1]]
			if a == b || out.Vertices[a] == out.Vertices[b] {
				return empty, geom.ErrConvergence
			}
			out.Segments = append(out.Segments, SplineGraphSectionDisplayEdge{Vertices: [2]int{a, b}, Arc: p.arc, Cell: p.cell, ErrorUpper: x.ErrorUpper})
			out.MaximumError = max(out.MaximumError, x.ErrorUpper)
		}
	}
	return out, nil
}
