package plaincadbridge

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

var errToFaceCoverage = errors.New("brep: termination face does not cover the projected profile")

func (b *Bridge) toAnalyticFace(target stored, q request) (Shape, error) {
	if target.analytic == nil {
		return Shape{}, &failureError{"UNSUPPORTED", "to-face termination requires a qualified bounded target cap"}
	}
	native := target.analytic.solid
	face, err := target.analytic.planarFace(q.Origin.vec(), q.Normal.vec())
	if err != nil {
		return Shape{}, fmt.Errorf("to-face reference was lost or is ambiguous: %w", err)
	}
	frame, outer, holes, err := native.PlanarStraightFaceWithCircularHoles(face.Face)
	if err != nil {
		return Shape{}, &failureError{"UNSUPPORTED", "to-face termination requires a straight cap boundary with complete circular holes"}
	}
	direction := q.Direction.vec()
	unit, err := direction.Unit()
	axis := frame.ZAxis()
	if err != nil || unit != axis && unit != axis.Scale(-1) {
		return Shape{}, &failureError{"UNSUPPORTED", "analytic target cap requires a perpendicular sweep"}
	}
	// Prove the exact outer boundary with the existing planar algorithm.
	// The planar extrusion is a proof carrier for that one finite support.
	carrier, err := brep.Prism(outer, axis)
	if err != nil {
		return Shape{}, err
	}
	ctx := context.Background()
	if q.Op == "circleToFace" {
		if q.Center == nil || len(q.Holes) != 0 {
			return Shape{}, errors.New("circular to-face extrusion requires one circle")
		}
		center, radius := q.Center.vec(), float64(q.Radius)
		_, proof, err := trimmed.CylinderToFace(ctx, center, direction, radius, carrier, 0, geom.DiskPolygonOptions{})
		if err != nil {
			return Shape{}, fmt.Errorf("extrude to face failed: %w", err)
		}
		p := frame.ToLocal(center)
		for _, hole := range holes {
			r := radius + hole.Radius
			if math.Hypot(p.X-hole.Center.X, p.Y-hole.Center.Y) <= r+native.Tolerance() {
				return Shape{}, errToFaceCoverage
			}
		}
		return b.circle(center, direction, radius, proof.Height)
	}
	source, sourceHoles := loop(q.Outer), make([][]geom.Vec3, len(q.Holes))
	count := len(source)
	for i, h := range q.Holes {
		count += len(h)
		sourceHoles[i] = loop(h)
	}
	if count > MaxProfileVertices {
		return Shape{}, &failureError{"LIMIT", "profile exceeds 128 vertices"}
	}
	result, err := brep.PrismToFace(ctx, source, sourceHoles, direction, carrier, 0)
	if err != nil {
		return Shape{}, fmt.Errorf("extrude to face failed: %w", err)
	}
	project := func(points []geom.Vec3) []geom.Vec3 {
		out := make([]geom.Vec3, len(points))
		for i, p := range points {
			local := frame.ToLocal(p)
			out[i] = frame.ToWorld(geom.Vec3{X: local.X, Y: local.Y})
		}
		return out
	}
	projected := project(source)
	projectedHoles := make([][]geom.Vec3, len(sourceHoles))
	for i, h := range sourceHoles {
		projectedHoles[i] = project(h)
	}
	for _, h := range holes {
		center := frame.ToWorld(geom.Vec3{X: h.Center.X, Y: h.Center.Y})
		disk, err := geom.NewPlanarCircularProfile(center, axis, h.Radius)
		if err != nil {
			return Shape{}, err
		}
		coverage, err := geom.ClassifyDiskPlanarPolygon(ctx, disk, projected, projectedHoles, geom.DiskPolygonOptions{})
		if err != nil {
			return Shape{}, err
		}
		if !coverage.Complete || coverage.Relation != geom.DiskPolygonOutside {
			return Shape{}, errToFaceCoverage
		}
	}
	return b.put(result)
}
