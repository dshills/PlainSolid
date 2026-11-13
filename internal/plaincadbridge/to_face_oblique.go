package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// obliqueCylinderToFace uses the retained BRep support plane at its numerical
// modeling tolerance. The finite footprint is proved exactly after projecting
// the face and all its holes along the (cardinal) cylinder generators.
func obliqueCylinderToFace(ctx context.Context, center, direction geom.Vec3, radius float64, target brep.Solid, faceIndex int) (*analyticBody, error) {
	frame, err := geom.FrameFromNormal(center, direction)
	if err != nil || !cardinalFrame(frame) {
		return nil, geom.ErrDomain
	}
	direction = frame.ZAxis()
	plane, err := target.FacePlane(faceIndex)
	if err != nil {
		return nil, err
	}
	normal := plane.Normal()
	if normal.Dot(direction) < 0 {
		normal = normal.Scale(-1)
		plane, err = geom.NewPlane(plane.Origin(), normal)
		if err != nil {
			return nil, err
		}
	}
	axial := normal.Dot(direction)
	if axial <= 1e-12 {
		return nil, geom.ErrDomain
	}
	disk, err := geom.NewPlanarCircularProfile(center, direction, radius)
	if err != nil {
		return nil, err
	}
	vertices, edges, face := target.Vertices(), target.Edges(), target.Faces()[faceIndex]
	projected := make([][]geom.Vec3, 0, 1+len(face.Holes))
	for _, wire := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
		points := make([]geom.Vec3, len(wire))
		for i, use := range wire {
			edge := edges[use.Edge]
			index := edge.Start
			if use.Reversed {
				index = edge.End
			}
			point := vertices[index]
			switch {
			case direction.X != 0:
				point.X = center.X
			case direction.Y != 0:
				point.Y = center.Y
			default:
				point.Z = center.Z
			}
			points[i] = point
		}
		projected = append(projected, points)
	}
	proof, err := geom.ClassifyDiskPlanarPolygon(ctx, disk, projected[0], projected[1:], geom.DiskPolygonOptions{})
	if err != nil {
		return nil, err
	}
	if proof.Relation != geom.DiskPolygonStrictlyCovered && proof.Relation != geom.DiskPolygonCoveredTouching {
		return nil, brep.ErrTerminationUncovered
	}
	height := -plane.SignedDistance(center) / axial
	spread := radius * math.Hypot(normal.Dot(frame.XAxis()), normal.Dot(frame.YAxis())) / axial
	tolerance := max(1, radius, math.Abs(height), spread) * 1e-10
	if !finite(height+spread) || height-spread <= tolerance {
		return nil, geom.ErrDomain
	}
	clipped, err := parametric.ClipCylinder(frame, radius, height+spread+max(radius, height), plane, tolerance)
	if err != nil {
		return nil, err
	}
	return revolutionPatches(clipped)
}
