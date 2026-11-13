package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CertifiedEdgeChord carries the original edge ID and a pointwise deviation
// bound. Its domain reports the normalized parameter interval on the stored edge,
// with ordinary float rounding. ErrorUpper refers to the source parameter map.
type CertifiedEdgeChord struct {
	Edge int
	geom.CertifiedCurveSegment
}

// CertifiedEdgeChords builds a model-space drawing of all supported spline
// model edges. Unsupported curves return an error.
// It does not substitute samples for an unavailable deflection certificate.
func (s Solid) CertifiedEdgeChords(ctx context.Context, o geom.CertifiedTessellationOptions, maxSegments int) ([]CertifiedEdgeChord, error) {
	if ctx == nil || maxSegments < 1 || !finite(o.Deflection) || o.Deflection < 0 {
		return nil, geom.ErrInvalid
	}
	if o.Deflection == 0 {
		o.Deflection = 1e-4
	}
	var out []CertifiedEdgeChord
	for i, edge := range s.edges {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if edge.Curve == nil {
			continue
		}
		budget := maxSegments - len(out)
		if budget <= 0 {
			return nil, parametric.ErrBudget
		}
		c, reversed, e := exactEdgeSpline(edge)
		if e != nil {
			return nil, e
		}
		if c == nil {
			return nil, fmt.Errorf("trimmed: edge %d has no exact spline descriptor", i)
		}
		local := o
		if local.MaxSegments == 0 || local.MaxSegments > budget {
			local.MaxSegments = budget
		}
		t, e := c.TessellateCertifiedContext(ctx, local)
		if e != nil {
			return nil, e
		}
		if !t.Complete {
			return nil, geom.ErrConvergence
		}
		a, b := c.Domain()
		for j := range t.Segments {
			k := j
			if reversed {
				k = len(t.Segments) - 1 - j
			}
			x := t.Segments[k]
			u, v := (x.Domain.Start-a)/(b-a), (x.Domain.End-a)/(b-a)
			if reversed {
				u, v = 1-v, 1-u
				x.Start, x.End = x.End, x.Start
			}
			x.Domain = geom.ParameterSpan{Start: u, End: v}
			out = append(out, CertifiedEdgeChord{i, x})
		}
	}
	return out, nil
}
