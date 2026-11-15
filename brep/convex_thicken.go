package brep

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ConvexThickenHistory records the source cap replaced by the lip and the
// inward facets traced to the supporting faces of the original solid.
// A negative interior face index identifies a missing facet.
type ConvexThickenHistory struct {
	LipFace       int
	InteriorFaces []int
	PlaneTests    int
}

// ThickenConvexOpenFace hollows a convex planar solid from one selected
// face. Each other source support has a parallel inward offset at the
// requested positive normal distance. The selected original face becomes a
// rim with one opening bounded by the actual inner facet intersection.
//
// The source and resulting cavity must meet the exact-coordinate conditions
// of OffsetConvex, except that the selected support stays in place. The
// operation requires a positive-volume cavity with a positive-area opening.
func (s Solid) ThickenConvexOpenFace(ctx context.Context, openingFace int, thickness float64, o ConvexOffsetOptions) (Solid, ConvexThickenHistory, error) {
	if openingFace < 0 || openingFace >= len(s.faces) || thickness <= 0 || math.IsNaN(thickness) || math.IsInf(thickness, 0) {
		return Solid{}, ConvexThickenHistory{}, geom.ErrInvalid
	}
	work, e := s.prepareConvexOffset(ctx, o)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	planes, e := s.convexOffsetPlanes(ctx, new(big.Rat).SetFloat64(-thickness), openingFace, &work)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	points, incident, e := intersectOffsetHalfspaces(ctx, planes, &work)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	inner := make([][]int, len(planes))
	facets := 0
	for i, p := range planes {
		if ring := offsetFacetHull(points, incident[i], p.n); len(ring) >= 3 {
			inner[i] = ring
			facets++
		}
	}
	if facets < 4 || len(inner[openingFace]) == 0 {
		return Solid{}, ConvexThickenHistory{}, fmt.Errorf("brep: inward offsets leave no cavity opening on source face %d: %w", openingFace, geom.ErrDomain)
	}
	vertices, e := representedOffsetVertices(points)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	polygons := make([][][]int, len(s.faces), len(s.faces)+facets-1)
	for i, f := range s.faces {
		ring, e := s.faceIndices(f)
		if e != nil {
			return Solid{}, ConvexThickenHistory{}, e
		}
		polygons[i] = [][]int{ring}
	}
	h := ConvexThickenHistory{LipFace: openingFace, InteriorFaces: make([]int, len(planes)), PlaneTests: work.tests}
	for i, ring := range inner {
		h.InteriorFaces[i] = -1
		if len(ring) == 0 {
			continue
		}
		reverseIDs(ring)
		for j := range ring {
			ring[j] += len(s.vertices)
		}
		if i == openingFace {
			polygons[i] = append(polygons[i], ring)
			continue
		}
		h.InteriorFaces[i] = len(polygons)
		polygons = append(polygons, [][]int{ring})
	}
	if e = ctx.Err(); e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	out, e := fromPolygonFaces(append(s.Vertices(), vertices...), polygons)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	if e = ctx.Err(); e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	return out, h, nil
}
