package brep

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"plainsolid/geom"
)

// ConvexThickenHistory records the source faces retaining a lip and the
// inward facets traced to the supporting faces of the original solid.
// LipFace identifies a single opening, or is -1 for other cases. LipFaces
// lists all openings in source face order. A negative interior face index
// identifies a missing facet.
type ConvexThickenHistory struct {
	LipFace       int
	LipFaces      []int
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
	return s.ThickenConvex(ctx, []int{openingFace}, thickness, o)
}

// ThickenConvex hollows a convex planar source to the requested positive
// normal distance. Any selected source faces stay in their original
// planes and retain a lip around each opening. With an empty face list,
// the exact inward offset becomes a closed cavity shell.
//
// Every opening must have positive area and lie strictly inside its
// original face. Openings that meet each other on an original edge or
// vertex are outside this construction's domain.
func (s Solid) ThickenConvex(ctx context.Context, openingFaces []int, thickness float64, o ConvexOffsetOptions) (Solid, ConvexThickenHistory, error) {
	if len(s.faces) == 0 || len(openingFaces) > len(s.faces) || thickness <= 0 || math.IsNaN(thickness) || math.IsInf(thickness, 0) {
		return Solid{}, ConvexThickenHistory{}, geom.ErrInvalid
	}
	work, e := s.prepareConvexOffset(ctx, o)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	opened := make(map[int]bool, len(openingFaces))
	for _, i := range openingFaces {
		if i < 0 || i >= len(s.faces) || opened[i] {
			return Solid{}, ConvexThickenHistory{}, geom.ErrInvalid
		}
		opened[i] = true
	}
	planes, e := s.convexOffsetPlanes(ctx, new(big.Rat).SetFloat64(-thickness), opened, &work)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	points, incident, e := intersectOffsetHalfspaces(ctx, planes, &work)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	inner := make([][]int, len(planes))
	facets := 0
	openingVertices := make(map[int]bool)
	for i, p := range planes {
		ring := offsetFacetHull(points, incident[i], p.n)
		if len(ring) >= 3 {
			inner[i] = ring
			facets++
		}
		if !opened[i] {
			continue
		}
		if len(inner[i]) == 0 {
			return Solid{}, ConvexThickenHistory{}, fmt.Errorf("brep: inward offsets leave no cavity opening on source face %d: %w", i, geom.ErrDomain)
		}
		// Shifted planes already have positive clearance. A cavity
		// vertex on two unshifted planes would touch the original rim.
		for _, id := range incident[i] {
			if openingVertices[id] {
				return Solid{}, ConvexThickenHistory{}, fmt.Errorf("brep: cavity openings meet on the original boundary: %w", geom.ErrDomain)
			}
			openingVertices[id] = true
		}
	}
	if facets < 4 {
		return Solid{}, ConvexThickenHistory{}, fmt.Errorf("brep: inward offsets leave no positive-volume cavity: %w", geom.ErrDomain)
	}
	vertices, e := representedOffsetVertices(points)
	if e != nil {
		return Solid{}, ConvexThickenHistory{}, e
	}
	polygons := make([][][]int, len(s.faces), len(s.faces)+facets-len(opened))
	for i, f := range s.faces {
		ring, e := s.faceIndices(f)
		if e != nil {
			return Solid{}, ConvexThickenHistory{}, e
		}
		polygons[i] = [][]int{ring}
	}
	h := ConvexThickenHistory{LipFace: -1, InteriorFaces: make([]int, len(planes)), PlaneTests: work.tests}
	if len(openingFaces) == 1 {
		h.LipFace = openingFaces[0]
	}
	for i, ring := range inner {
		h.InteriorFaces[i] = -1
		if len(ring) == 0 {
			continue
		}
		reverseIDs(ring)
		for j := range ring {
			ring[j] += len(s.vertices)
		}
		if opened[i] {
			h.LipFaces = append(h.LipFaces, i)
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
