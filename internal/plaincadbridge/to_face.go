package plaincadbridge

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// toFace resolves the actual retained finite target face first. The authored
// plane is only a reference locator; it is never substituted for face coverage.
func (b *Bridge) toFace(q request) (Shape, error) {
	if q.Origin == nil || q.Normal == nil || q.Direction == nil {
		return Shape{}, errors.New("extrude to face requires a target plane and sweep direction")
	}
	target, err := b.get(q.B)
	if err != nil {
		return Shape{}, err
	}
	if !target.planar() {
		return Shape{}, &failureError{"UNSUPPORTED", "to-face termination currently requires a planar target BRep"}
	}
	face, err := matchingPlanarFace(target.solid, q.Origin.vec(), q.Normal.vec())
	if err != nil {
		return Shape{}, fmt.Errorf("to-face reference was lost or is ambiguous: %w", err)
	}
	if q.Op == "circleToFace" {
		if q.Center == nil {
			return Shape{}, errors.New("circular to-face extrusion requires a center")
		}
		if len(q.Holes) != 0 {
			return Shape{}, &failureError{"UNSUPPORTED", "circular to-face extrusion does not support inner loops"}
		}
		_, proof, err := trimmed.CylinderToFace(context.Background(), q.Center.vec(), q.Direction.vec(), float64(q.Radius), target.solid, face.Face, geom.DiskPolygonOptions{})
		if err != nil {
			return Shape{}, fmt.Errorf("extrude to face failed: %w", err)
		}
		return b.circle(q.Center.vec(), q.Direction.vec(), float64(q.Radius), proof.Height)
	}
	n := len(q.Outer)
	for _, h := range q.Holes {
		n += len(h)
	}
	if n > MaxProfileVertices {
		return Shape{}, &failureError{"LIMIT", "profile exceeds 128 vertices"}
	}
	holes := make([][]geom.Vec3, len(q.Holes))
	for i, h := range q.Holes {
		holes[i] = loop(h)
	}
	solid, err := brep.PrismToFace(context.Background(), loop(q.Outer), holes, q.Direction.vec(), target.solid, face.Face)
	if err != nil {
		return Shape{}, fmt.Errorf("extrude to face failed: %w", err)
	}
	return b.put(solid)
}
