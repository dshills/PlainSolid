//go:build research

package trimmed

import (
	"context"

	"plainsolid/geom"
)

// CertifiedExactQuadraticDisplay contains a single continuous trim display
// polyline. Segments retain the originating knot cell and its exact
// pointwise chord error bound, including display rounding.
type CertifiedExactQuadraticDisplay struct {
	Vertices     []geom.ComposedPolylineVertex
	Segments     []ExactQuadraticDisplaySegment
	MaximumError float64
}

type ExactQuadraticDisplaySegment struct {
	geom.ComposedPolylineSegment
	Cell int
}

// TessellateModelCertified approximates an exact trim on all its composed
// knot cells, including irrational crossings. Adjacent cells share a
// represented junction vertex whose error is independently bounded for each.
func (q ExactQuadraticUV) TessellateModelCertified(ctx context.Context, p *geom.SurfacePatch, c geom.SurfaceCurveAgreementOptions,
	o geom.ComposedCurvePolylineOptions) (CertifiedExactQuadraticDisplay, error) {
	var empty CertifiedExactQuadraticDisplay
	if ctx == nil || o.EndpointPoints != nil {
		return empty, geom.ErrInvalid
	}
	if o.Deflection == 0 {
		o.Deflection = 1e-6
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || o.MaxOperations < 0 || o.MaxEndpointSubdivisions < 0 || o.MaxSegments < 0 {
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
	r, e := q.ModelCurveCells(ctx, p, c)
	if e != nil {
		return empty, e
	}
	if !r.Complete || len(r.Cells) == 0 {
		return empty, geom.ErrConvergence
	}
	if len(r.Cells) > o.MaxSegments {
		return empty, geom.ErrConvergence
	}
	junctions := make([]geom.Vec3, len(r.Cells)+1)
	for i := range junctions {
		cell, side := min(i, len(r.Cells)-1), 0
		if i == len(r.Cells) {
			side = 1
		}
		if o.MaxOperations < 1 || o.MaxEndpointSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		limit := o.Deflection / 8
		if limit == 0 {
			return empty, geom.ErrConvergence
		}
		x, e := r.Cells[cell].EncloseEndpoint(ctx, side, geom.ComposedCurveEndpointOptions{Tolerance: limit,
			ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{
				MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxEndpointSubdivisions,
			}})
		if e != nil {
			return empty, e
		}
		if !x.Converged || !x.HasRepresentative {
			return empty, geom.ErrConvergence
		}
		junctions[i] = x.Point
		o.MaxOperations -= x.ExactOperations
		o.MaxEndpointSubdivisions -= x.Subdivisions
	}
	var out CertifiedExactQuadraticDisplay
	for i, cell := range r.Cells {
		if o.MaxOperations < 1 || o.MaxEndpointSubdivisions < 1 || o.MaxSegments < 1 {
			return empty, geom.ErrConvergence
		}
		options := o
		options.EndpointPoints = &[2]geom.Vec3{junctions[i], junctions[i+1]}
		t, e := cell.TessellateCertified(ctx, options)
		if e != nil {
			return empty, e
		}
		if !t.Complete || len(t.Vertices) < 2 {
			return empty, geom.ErrConvergence
		}
		offset := len(out.Vertices)
		start := 0
		if i > 0 {
			offset--
			start = 1
			old, joined := &out.Vertices[offset], t.Vertices[0]
			if old.Point != joined.Point {
				return empty, geom.ErrConvergence
			}
			old.ErrorUpper = min(old.ErrorUpper, joined.ErrorUpper)
			old.Parameter.Lower = maxRat(old.Parameter.Lower, joined.Parameter.Lower)
			old.Parameter.Upper = minRat(old.Parameter.Upper, joined.Parameter.Upper)
			if old.Parameter.Lower.Cmp(old.Parameter.Upper) > 0 {
				return empty, geom.ErrConvergence
			}
		}
		out.Vertices = append(out.Vertices, t.Vertices[start:]...)
		for _, s := range t.Segments {
			s.Vertices[0] += offset
			s.Vertices[1] += offset
			if out.Vertices[s.Vertices[0]].Point == out.Vertices[s.Vertices[1]].Point {
				return empty, geom.ErrConvergence
			}
			out.Segments = append(out.Segments, ExactQuadraticDisplaySegment{s, i})
			out.MaximumError = max(out.MaximumError, s.ErrorUpper)
		}
		o.MaxOperations -= t.ExactOperations
		o.MaxEndpointSubdivisions -= t.EndpointSubdivisions
		o.MaxSegments -= len(t.Segments)
	}
	return out, nil
}
