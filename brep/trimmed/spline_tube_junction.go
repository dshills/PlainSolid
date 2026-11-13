package trimmed

import (
	"context"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineTubeEdgeEnd describes one endpoint of one surface-pair branch
// meeting the selected shell face's original edge. End is 0 or 1 in branch
// traversal order; Chart is 0 or 1 in the ordered pair.
type SplineTubeEdgeEnd struct {
	Face, End, Chart int
	Segment          intersect.TubeSegment
	Endpoint         SplineTubeEndpoint
}

// SplineTubeEdgeJunction proves two branch endpoints on adjacent shell faces
// meet at one exact model edge parameter. It makes no assertion about other
// possible branch continuations away from this edge.
type SplineTubeEdgeJunction struct {
	Anchor      EdgeAnchor
	First, Last geom.TubeEndpointEdgeCertificate
}

// CertifySplineTubeEdgeJunction verifies both exact endpoint proposals on the
// original shared edge. The two branches may have different surface pairs or
// free coordinates. An equal model point alone cannot substitute for shared
// edge identity. This routine does not mutate the shell.
func (s Solid) CertifySplineTubeEdgeJunction(ctx context.Context, first, last SplineTubeEdgeEnd, o geom.TubeEndpointEdgeOptions) (SplineTubeEdgeJunction, error) {
	var r SplineTubeEdgeJunction
	if ctx == nil || first.Face == last.Face {
		return r, geom.ErrInvalid
	}
	a, e := s.CertifySplineTubeAnchor(ctx, first.Face, first.Segment, first.End, first.Chart, first.Endpoint.Side, first.Endpoint.Hint, o)
	if e != nil {
		return r, e
	}
	b, e := s.CertifySplineTubeAnchor(ctx, last.Face, last.Segment, last.End, last.Chart, last.Endpoint.Side, last.Endpoint.Hint, o)
	if e != nil {
		return r, e
	}
	if a.Anchor != b.Anchor {
		return r, geom.ErrConvergence
	}
	for k, x := range a.Proof.Point {
		if x.Cmp(b.Proof.Point[k]) != 0 {
			return r, geom.ErrConvergence
		}
	}
	return SplineTubeEdgeJunction{Anchor: a.Anchor, First: a.Proof, Last: b.Proof}, nil
}
