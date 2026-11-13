package csg

import (
	"fmt"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
)

func featurePlanes(s brep.Solid, tolerance float64) ([]plane, geom.Vec3, error) {
	if math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance <= 0 {
		return nil, geom.Vec3{}, fmt.Errorf("csg: tolerance must be finite and positive")
	}
	vertices := s.Vertices()
	if len(vertices) == 0 {
		return nil, geom.Vec3{}, nil
	}
	origin := vertices[0]
	planes, err := convexPlanes(s, origin, tolerance)
	return planes, origin, err
}

// Offset translates each outward supporting plane by distance along its normal
// and reconstructs their intersection. Negative distance erodes the convex solid;
// positive distance produces sharp mitered corners, not a rounded Euclidean
// offset or rolling-ball surface. Planes can disappear and inward offsets can
// collapse to the empty solid. Tolerance and numeric limits match Intersect.
func Offset(s brep.Solid, distance, tolerance float64) (brep.Solid, error) {
	if math.IsNaN(distance) || math.IsInf(distance, 0) {
		return brep.Solid{}, fmt.Errorf("csg: offset distance must be finite")
	}
	planes, origin, err := featurePlanes(s, tolerance)
	if err != nil {
		return brep.Solid{}, err
	}
	if len(planes) == 0 || distance == 0 {
		return s, nil
	}
	for i := range planes {
		planes[i].d += distance
		if math.IsInf(planes[i].d, 0) || math.IsNaN(planes[i].d) {
			return brep.Solid{}, fmt.Errorf("csg: offset supporting-plane coordinate overflows")
		}
	}
	return reconstruct(planes, origin, tolerance)
}

// Chamfer clips selected topological edges with planar equal-setback bevels.
// Distance is measured along each incident face, perpendicular to the edge.
// The input must be outward and convex; selected edges must join two distinct
// noncoplanar faces. All bevel planes refer to the original shape, so multiple
// chamfers are applied simultaneously rather than modifying edge IDs in sequence.
// The selection list is bounded to MaxPlanes*MaxPlanes entries, and source faces
// plus distinct selected edges must fit the MaxPlanes reconstruction budget.
// Duplicate indices are ignored. A large distance can remove faces or the entire
// solid. No fillets, variable setbacks, or corner blending surfaces are generated.
func Chamfer(s brep.Solid, edgeIndices []int, distance, tolerance float64) (brep.Solid, error) {
	if len(edgeIndices) > MaxPlanes*MaxPlanes {
		return brep.Solid{}, fmt.Errorf("csg: chamfer selection work limit exceeded")
	}
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 0 {
		return brep.Solid{}, fmt.Errorf("csg: chamfer distance must be finite and nonnegative")
	}
	planes, origin, err := featurePlanes(s, tolerance)
	if err != nil {
		return brep.Solid{}, err
	}
	edges, vertices, faces := s.Edges(), s.Vertices(), s.Faces()
	seen := map[int]bool{}
	selected := make([]int, 0, len(edgeIndices))
	for _, index := range edgeIndices {
		if index < 0 || index >= len(edges) {
			return brep.Solid{}, fmt.Errorf("csg: chamfer edge index out of range")
		}
		if !seen[index] {
			seen[index] = true
			selected = append(selected, index)
		}
	}
	if len(planes)+len(selected) > MaxPlanes {
		return brep.Solid{}, fmt.Errorf("csg: supporting-plane limit exceeded (%d)", MaxPlanes)
	}
	if len(selected) == 0 || distance == 0 {
		return s, nil
	}
	adjacent := make([][]int, len(edges))
	for f, face := range faces {
		for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
			for _, use := range loop {
				adjacent[use.Edge] = append(adjacent[use.Edge], f)
			}
		}
	}
	original := append([]plane(nil), planes...)
	for _, index := range selected {
		incident := adjacent[index]
		if len(incident) != 2 || incident[0] == incident[1] {
			return brep.Solid{}, fmt.Errorf("csg: selected edge must join two faces")
		}
		a, b := original[incident[0]].n, original[incident[1]].n
		sinHalf := a.Sub(b).Length() / 2
		normal, err := a.Add(b).Unit()
		if err != nil || sinHalf <= angularTolerance || a.Cross(b).Length() <= angularTolerance {
			return brep.Solid{}, fmt.Errorf("csg: selected edge has coplanar or unresolved incident faces")
		}
		point := vertices[edges[index].Start].Sub(origin)
		d := normal.Dot(point) - distance*sinHalf
		if math.IsNaN(d) || math.IsInf(d, 0) {
			return brep.Solid{}, fmt.Errorf("csg: chamfer supporting-plane coordinate overflows")
		}
		planes = append(planes, plane{n: normal, d: d})
	}
	return reconstruct(planes, origin, tolerance)
}
