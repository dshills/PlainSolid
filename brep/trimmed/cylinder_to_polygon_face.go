package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CylinderToFaceProof retains exact selected-cap coverage and the represented
// construction height/endpoint. It does not prove first contact with other
// target faces, stock embedding, or a Boolean cut. The native analytic topology
// retains its own numerical construction/validation tolerance independently of
// this exact finite-face footprint predicate.
type CylinderToFaceProof struct {
	TargetFace int
	Height     float64
	Endpoint   geom.Vec3
	Coverage   geom.DiskPolygonComparison
}

// CylinderToFace extrudes a circular profile along a coordinate-axis normal to
// a selected parallel finite polygon face, including polygon holes. Direction
// magnitude is ignored. Only positive, exactly representable height is accepted;
// the actual cylinder constructor's frame map must reproduce the exact endpoint.
// Covered tangency is allowed. Uncovered profiles return ErrTerminationUncovered.
func CylinderToFace(ctx context.Context, center, normal geom.Vec3, radius float64, target brep.Solid, faceIndex int, o geom.DiskPolygonOptions) (Solid, CylinderToFaceProof, error) {
	var proof CylinderToFaceProof
	proof.TargetFace = faceIndex
	if ctx == nil || !center.IsFinite() || !normal.IsFinite() {
		return Solid{}, proof, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, proof, e
	}
	components := [3]float64{normal.X, normal.Y, normal.Z}
	axis := -1
	sign := 1.
	for i, q := range components {
		if q != 0 {
			if axis >= 0 {
				return Solid{}, proof, geom.ErrDomain
			}
			axis = i
			if q < 0 {
				sign = -1
			}
		}
	}
	if axis < 0 {
		return Solid{}, proof, geom.ErrInvalid
	}
	faces, edges, vertices := target.Faces(), target.Edges(), target.Vertices()
	if faceIndex < 0 || faceIndex >= len(faces) {
		return Solid{}, proof, geom.ErrInvalid
	}
	face := faces[faceIndex]
	recoverLoop := func(uses []brep.EdgeUse) ([]geom.Vec3, error) {
		if len(uses) < 3 {
			return nil, geom.ErrInvalid
		}
		points := make([]geom.Vec3, len(uses))
		for i, use := range uses {
			if use.Edge < 0 || use.Edge >= len(edges) {
				return nil, geom.ErrInvalid
			}
			edge := edges[use.Edge]
			start, end := edge.Start, edge.End
			if use.Reversed {
				start, end = end, start
			}
			if start < 0 || start >= len(vertices) || end < 0 || end >= len(vertices) {
				return nil, geom.ErrInvalid
			}
			next := uses[(i+1)%len(uses)]
			if next.Edge < 0 || next.Edge >= len(edges) {
				return nil, geom.ErrInvalid
			}
			nextStart := edges[next.Edge].Start
			if next.Reversed {
				nextStart = edges[next.Edge].End
			}
			if end != nextStart {
				return nil, geom.ErrInvalid
			}
			points[i] = vertices[start]
		}
		return points, nil
	}
	outer, e := recoverLoop(face.Boundary)
	if e != nil {
		return Solid{}, proof, e
	}
	holes := make([][]geom.Vec3, len(face.Holes))
	for i, loop := range face.Holes {
		holes[i], e = recoverLoop(loop)
		if e != nil {
			return Solid{}, proof, e
		}
	}
	coordinate := func(p geom.Vec3) float64 { return [3]float64{p.X, p.Y, p.Z}[axis] }
	cap := coordinate(outer[0])
	for _, loop := range append([][]geom.Vec3{outer}, holes...) {
		for _, p := range loop {
			if coordinate(p) != cap {
				return Solid{}, proof, geom.ErrDomain
			}
		}
	}
	height := new(big.Rat).Sub(new(big.Rat).SetFloat64(cap), new(big.Rat).SetFloat64(coordinate(center)))
	if sign < 0 {
		height.Neg(height)
	}
	if height.Sign() <= 0 {
		return Solid{}, proof, geom.ErrDomain
	}
	h, e := geom.RepresentedExactScalar(height)
	if e != nil {
		return Solid{}, proof, e
	}
	endpoint := center
	unit := geom.Vec3{}
	switch axis {
	case 0:
		endpoint.X = cap
		unit.X = sign
	case 1:
		endpoint.Y = cap
		unit.Y = sign
	case 2:
		endpoint.Z = cap
		unit.Z = sign
	}
	disk, e := geom.NewPlanarCircularProfile(endpoint, unit, radius)
	if e != nil {
		return Solid{}, proof, e
	}
	proof.Coverage, e = geom.ClassifyDiskPlanarPolygon(ctx, disk, outer, holes, o)
	if e != nil {
		return Solid{}, proof, e
	}
	if proof.Coverage.Relation != geom.DiskPolygonStrictlyCovered && proof.Coverage.Relation != geom.DiskPolygonCoveredTouching {
		return Solid{}, proof, brep.ErrTerminationUncovered
	}
	frame, e := geom.FrameFromNormal(center, unit)
	if e != nil {
		return Solid{}, proof, e
	}
	if frame.ToWorld(geom.Vec3{Z: h}) != endpoint {
		return Solid{}, proof, geom.ErrUnrepresentable
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, proof, e
	}
	native, e := parametric.Cylinder(frame, radius, h)
	if e != nil {
		return Solid{}, proof, e
	}
	solid, e := FromParametric(native)
	if e != nil {
		return Solid{}, proof, e
	}
	proof.Height = h
	proof.Endpoint = endpoint
	return solid, proof, nil
}
