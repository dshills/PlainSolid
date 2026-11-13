package geom

import (
	"context"
	"math"
)

// ComposedCollectionPolylineOptions uses a global MaxSegments/MaxOperations
// budget. Optional JunctionPoints supplies one represented point per original
// cell boundary, including both ends. Every point is independently certified;
// this permits a deliberate separated display only within Deflection. Do not
// combine JunctionPoints with the embedded overall EndpointPoints override.
type ComposedCollectionPolylineOptions struct {
	ComposedCurvePolylineOptions
	JunctionPoints []Vec3
}

// ComposedPolylineJunction preserves exact parameter identity independently of
// its displayed coordinate. Different roots may map to identical float points.
type ComposedPolylineJunction struct {
	Vertex     int
	Root       *ExactAlgebraicRoot
	ErrorUpper float64
}

// CertifiedComposedCollectionPolyline never welds distinct parameter events by
// coordinate equality. CollapsedSegments and CoincidentJunctionGroups identify
// display degeneracy only, not exact geometric equality or topological merging.
type CertifiedComposedCollectionPolyline struct {
	CertifiedComposedPolyline
	Junctions                []ComposedPolylineJunction
	DisplayDegenerate        bool
	CollapsedSegments        []int
	CoincidentJunctionGroups [][]int
}

// TessellateCertified proves the ordered collection's joins, selects shared
// represented junctions, then applies the per-cell chord/rounding certificate.
// Only exact shared root/model identities permit reuse of a vertex index.
// Complete proves coverage of the full supplied (possibly restricted) domain.
func (collection SurfaceCurveCompositionCells) TessellateCertified(ctx context.Context, o ComposedCollectionPolylineOptions) (r CertifiedComposedCollectionPolyline, err error) {
	if ctx == nil || len(collection.Cells) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if len(collection.Cells) > 4096 {
		return r, ErrConvergence
	}
	if o.Deflection == 0 {
		o.Deflection = 1e-6
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxEndpointSubdivisions == 0 {
		o.MaxEndpointSubdivisions = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || o.MaxSegments < 1 || o.MaxDepth < 1 || o.MaxDepth > 64 || o.MaxEndpointSubdivisions < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	if o.JunctionPoints != nil && (len(o.JunctionPoints) != len(collection.Cells)+1 || o.EndpointPoints != nil) {
		return r, ErrInvalid
	}
	first, last := collection.Cells[0], collection.Cells[len(collection.Cells)-1]
	if first == nil || last == nil {
		return r, ErrInvalid
	}
	lower, e := first.EndpointRoot(0)
	if e != nil {
		return r, e
	}
	upper, e := last.EndpointRoot(1)
	if e != nil {
		return r, e
	}
	validated, e := collection.RestrictAlgebraic(ctx, lower, upper, ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxEndpointSubdivisions})
	r.ExactOperations = validated.ExactOperations
	r.EndpointSubdivisions = validated.Subdivisions
	if e != nil {
		return r, e
	}
	if !validated.Complete || len(validated.Cells) != len(collection.Cells) {
		return r, ErrConvergence
	}
	points := make([]Vec3, len(validated.Cells)+1)
	r.Junctions = make([]ComposedPolylineJunction, len(points))
	tolerance := o.Deflection / 32
	if tolerance == 0 {
		tolerance = o.Deflection
	}
	for i := range points {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		cell, endpoint := validated.Cells[0], 0
		if i > 0 {
			cell, endpoint = validated.Cells[i-1], 1
		}
		root, e := cell.EndpointRoot(endpoint)
		if e != nil {
			return r, e
		}
		r.Junctions[i].Root = root
		r.Junctions[i].Vertex = -1
		if o.JunctionPoints != nil {
			points[i] = o.JunctionPoints[i]
			if !points[i].IsFinite() {
				return r, ErrInvalid
			}
			continue
		}
		remaining := o.MaxOperations - r.ExactOperations
		if remaining < 1 {
			return r, ErrConvergence
		}
		bounds, e := cell.EncloseEndpoint(ctx, endpoint, ComposedCurveEndpointOptions{ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxEndpointSubdivisions}, Tolerance: tolerance})
		r.ExactOperations += bounds.ExactOperations
		r.EndpointSubdivisions += bounds.Subdivisions
		if e != nil {
			return r, e
		}
		if !bounds.Converged || !bounds.HasRepresentative {
			return r, ErrConvergence
		}
		points[i] = bounds.Point
	}
	if o.EndpointPoints != nil {
		if !o.EndpointPoints[0].IsFinite() || !o.EndpointPoints[1].IsFinite() {
			return r, ErrInvalid
		}
		points[0] = o.EndpointPoints[0]
		points[len(points)-1] = o.EndpointPoints[1]
	}
	defer func() {
		groups := map[Vec3][]int{}
		for i, junction := range r.Junctions {
			if junction.Vertex >= 0 && junction.Vertex < len(r.Vertices) {
				point := r.Vertices[junction.Vertex].Point
				groups[point] = append(groups[point], i)
			}
		}
		// Preserve deterministic output order, independent of map iteration.
		for i, junction := range r.Junctions {
			if junction.Vertex < 0 || junction.Vertex >= len(r.Vertices) {
				continue
			}
			group := groups[r.Vertices[junction.Vertex].Point]
			if len(group) > 1 && group[0] == i {
				r.CoincidentJunctionGroups = append(r.CoincidentJunctionGroups, append([]int(nil), group...))
			}
		}
		for i, segment := range r.Segments {
			if r.Vertices[segment.Vertices[0]].Point == r.Vertices[segment.Vertices[1]].Point {
				r.CollapsedSegments = append(r.CollapsedSegments, i)
			}
		}
		r.DisplayDegenerate = len(r.CollapsedSegments) > 0 || len(r.CoincidentJunctionGroups) > 0
	}()
	for i, cell := range validated.Cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		child := o.ComposedCurvePolylineOptions
		child.MaxOperations -= r.ExactOperations
		child.MaxSegments -= len(r.Segments)
		if child.MaxOperations < 1 || child.MaxSegments < 1 {
			return r, ErrConvergence
		}
		ends := [2]Vec3{points[i], points[i+1]}
		child.EndpointPoints = &ends
		polyline, e := cell.TessellateCertified(ctx, child)
		r.ExactOperations += polyline.ExactOperations
		r.EndpointSubdivisions += polyline.EndpointSubdivisions
		if len(polyline.Vertices) > 0 {
			offset := len(r.Vertices) - 1
			if len(r.Vertices) == 0 {
				offset = 0
				r.Vertices = append(r.Vertices, polyline.Vertices[0])
			} else {
				if r.Vertices[offset].Point != polyline.Vertices[0].Point {
					return r, ErrConvergence
				}
				r.Vertices[offset].ErrorUpper = math.Max(r.Vertices[offset].ErrorUpper, polyline.Vertices[0].ErrorUpper)
			}
			r.Junctions[i].Vertex = offset
			r.Junctions[i].ErrorUpper = math.Max(r.Junctions[i].ErrorUpper, polyline.Vertices[0].ErrorUpper)
			r.Vertices = append(r.Vertices, polyline.Vertices[1:]...)
			for _, segment := range polyline.Segments {
				segment.Vertices[0] += offset
				segment.Vertices[1] += offset
				r.Segments = append(r.Segments, segment)
			}
			r.MaximumError = math.Max(r.MaximumError, polyline.MaximumError)
			if polyline.Complete {
				index := len(r.Vertices) - 1
				r.Junctions[i+1].Vertex = index
				r.Junctions[i+1].ErrorUpper = math.Max(r.Junctions[i+1].ErrorUpper, r.Vertices[index].ErrorUpper)
			}
		}
		if e != nil {
			return r, e
		}
		if !polyline.Complete {
			return r, ErrConvergence
		}
	}
	r.Complete = true
	return r, nil
}
