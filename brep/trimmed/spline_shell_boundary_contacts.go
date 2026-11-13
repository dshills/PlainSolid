package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
)

// SplineShellBoundaryContactOptions bounds both operand passes. MaxPairs
// defaults to 512, MaxWork to 64M exact isolation operations. Candidate
// pairs without a provably disjoint box are tested against original charts.
type SplineShellBoundaryContactOptions struct {
	Contact           SplineSeamFaceOptions
	MaxPairs, MaxWork int
}

// SplineShellBoundaryPair holds evidence for one original edge-to-face pair.
// Source is 0 for A edges versus B faces, and 1 for the opposite direction.
type SplineShellBoundaryPair struct {
	Source  int
	Contact SplineSeamFaceReport
}

type SplineShellBoundaryContacts struct {
	Pairs                        []SplineShellBoundaryPair
	TestedPairs, ExactOperations int
	Complete                     bool
}

// DiscoverSplineShellBoundaryContacts searches both directions for regular
// boundary-to-face events. Complete means all candidate edge/face pairs were
// exhausted; it does not exclude closed intersection components in face
// interiors, or assemble intersection wires. Boundary ambiguity is retained.
func DiscoverSplineShellBoundaryContacts(ctx context.Context, a, b Solid, o SplineShellBoundaryContactOptions) (SplineShellBoundaryContacts, error) {
	var out SplineShellBoundaryContacts
	if ctx == nil || o.MaxPairs < 0 || o.MaxPairs > 65536 || o.MaxWork < 0 {
		return out, geom.ErrInvalid
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 512
	}
	if o.MaxWork == 0 {
		o.MaxWork = 64000000
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	complete := true
	for source, pair := range [2][2]Solid{{a, b}, {b, a}} {
		x, y := pair[0], pair[1]
		if len(x.edges) == 0 || len(y.faces) == 0 {
			continue
		}
		if len(x.edges) > o.MaxPairs || len(y.faces) > o.MaxPairs || len(x.edges) > o.MaxPairs/len(y.faces) {
			return out, geom.ErrConvergence
		}
		boxes := make([]geom.Box, len(y.faces))
		known := make([]bool, len(y.faces))
		for i, f := range y.faces {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			p, e := exactFaceSupportPatch(f, o.Contact.Composition.SurfacePatchOptions)
			if e != nil {
				continue
			}
			boxes[i], e = p.Bounds()
			known[i] = e == nil
		}
		for edge, m := range x.edges {
			var edgeBox geom.Box
			boxKnown := false
			if c, _, e := exactEdgeSplineWithOptions(m, o.Contact.Composition.SurfacePatchOptions); e == nil && c != nil {
				edgeBox, e = c.ProjectionBounds()
				boxKnown = e == nil
			}
			for face := range y.faces {
				if e := ctx.Err(); e != nil {
					return out, e
				}
				if boxKnown && known[face] {
					if _, overlap := edgeBox.Intersect(boxes[face]); !overlap {
						continue
					}
				}
				if out.TestedPairs >= o.MaxPairs || out.ExactOperations >= o.MaxWork {
					return out, geom.ErrConvergence
				}
				options := o.Contact
				left := o.MaxWork - out.ExactOperations
				if options.Intersection.MaxOperations == 0 {
					options.Intersection.MaxOperations = 8000000
				}
				options.Intersection.MaxOperations = min(options.Intersection.MaxOperations, left)
				r, e := IntersectSplineSeamFace(ctx, x, edge, y, face, options)
				out.TestedPairs++
				out.ExactOperations += r.ExactOperations
				if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDomain) {
					return out, e
				}
				if e != nil || !r.Complete || len(r.Hits) > 0 || r.Graph != nil && len(r.Graph.Hits) > 0 {
					out.Pairs = append(out.Pairs, SplineShellBoundaryPair{Source: source, Contact: r})
				}
				complete = complete && e == nil && r.Complete
			}
		}
	}
	out.Complete = complete
	if !complete {
		return out, geom.ErrConvergence
	}
	return out, nil
}
