// Package csg provides bounded convex-operand planar Booleans, plane sections
// and splits, supporting-plane offsets, and edge chamfers. Union and difference
// support transverse and exactly coplanar subsets; general nonconvex operands
// and curved-surface Booleans are unsupported.
package csg

import (
	"fmt"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// MaxPlanes bounds the combined unique supporting planes of an intersection.
// The algorithm enumerates plane triples and checks all halfspaces: O(P^4).
const MaxPlanes = 64
const maxVertices = 4096
const angularTolerance = 1e-12

type plane struct {
	n geom.Vec3
	d float64
}

// Intersect returns the regularized intersection of two outward-oriented convex
// planar solids. Empty or lower-dimensional (touching) intersections are empty.
// Tolerance is a positive finite absolute model-space distance; features at or
// below it may collapse. Inputs must each have one shell and no face holes.
// Supporting planes and output coordinates use float64. Near-dependent plane
// triples are solved with exact rational arithmetic on those represented planes;
// only exactly singular triples are skipped. Plane merging is bounded by the
// distance tolerance over the input vertices. Coordinate conversion or boundary
// reconstruction failures return errors. The supporting-plane derivation and
// other predicates remain numerical; this is not a general exact B-rep Boolean.
// No input is mutated.
func Intersect(a, b brep.Solid, tolerance float64) (brep.Solid, error) {
	if math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance <= 0 {
		return brep.Solid{}, fmt.Errorf("csg: tolerance must be finite and positive")
	}
	av, bv := a.Vertices(), b.Vertices()
	if len(av) == 0 || len(bv) == 0 {
		return brep.Solid{}, nil
	}
	origin := av[0]
	pa, err := convexPlanes(a, origin, tolerance)
	if err != nil {
		return brep.Solid{}, fmt.Errorf("csg: first input: %w", err)
	}
	pb, err := convexPlanes(b, origin, tolerance)
	if err != nil {
		return brep.Solid{}, fmt.Errorf("csg: second input: %w", err)
	}
	aa, _ := a.Bounds()
	bb, _ := b.Bounds()
	if aa.Max.X <= bb.Min.X || bb.Max.X <= aa.Min.X || aa.Max.Y <= bb.Min.Y || bb.Max.Y <= aa.Min.Y || aa.Max.Z <= bb.Min.Z || bb.Max.Z <= aa.Min.Z {
		return brep.Solid{}, nil
	}
	planes := make([]plane, 0, len(pa)+len(pb))
	for _, p := range append(pa, pb...) {
		found := false
		for i, q := range planes {
			if equivalentPlanes(p, q, av, bv, origin, tolerance) {
				if p.d < q.d {
					planes[i] = p
				}
				found = true
				break
			}
		}
		if !found {
			planes = append(planes, p)
		}
	}
	if len(planes) > MaxPlanes {
		return brep.Solid{}, fmt.Errorf("csg: supporting-plane limit exceeded (%d)", MaxPlanes)
	}
	return reconstruct(planes, origin, tolerance)
}

// reconstruct builds a closed boundary from a bounded convex halfspace system.
func reconstruct(planes []plane, origin geom.Vec3, tolerance float64) (brep.Solid, error) {
	// Exact duplicate normal directions contribute only their strictest bound.
	// This also accepts solids whose planar sides are subdivided into faces.
	unique := make([]plane, 0, len(planes))
	for _, p := range planes {
		found := false
		for i, q := range unique {
			if p.n == q.n {
				if p.d < q.d {
					unique[i] = p
				}
				found = true
				break
			}
		}
		if !found {
			unique = append(unique, p)
		}
	}
	planes = unique
	var err error
	var points []geom.Vec3
	for i := 0; i < len(planes); i++ {
		for j := i + 1; j < len(planes); j++ {
			for k := j + 1; k < len(planes); k++ {
				p, q, r := planes[i], planes[j], planes[k]
				cross := q.n.Cross(r.n)
				det := p.n.Dot(cross)
				var point geom.Vec3
				if math.Abs(det) <= angularTolerance {
					var exists bool
					point, exists, err = exactIntersection(p, q, r, planes, tolerance)
					if err != nil {
						return brep.Solid{}, err
					}
					if !exists {
						continue
					}
				} else {
					point = cross.Scale(p.d).Add(r.n.Cross(p.n).Scale(q.d)).Add(p.n.Cross(q.n).Scale(r.d)).Scale(1 / det)
				}
				if !point.IsFinite() {
					return brep.Solid{}, fmt.Errorf("csg: plane intersection overflows")
				}
				inside := true
				for _, h := range planes {
					if h.n.Dot(point)-h.d > tolerance {
						inside = false
						break
					}
				}
				if !inside {
					continue
				}
				duplicate := false
				for _, v := range points {
					if point.Sub(v).Length() <= tolerance {
						duplicate = true
						break
					}
				}
				if !duplicate {
					points = append(points, point)
					if len(points) > maxVertices {
						return brep.Solid{}, fmt.Errorf("csg: vertex limit exceeded")
					}
				}
			}
		}
	}
	if len(points) < 4 {
		return brep.Solid{}, nil
	}
	// A full-dimensional result must have positive thickness from every support.
	for _, p := range planes {
		extent := 0.0
		active := false
		for _, v := range points {
			distance := p.d - p.n.Dot(v)
			if math.Abs(distance) <= tolerance {
				active = true
			}
			extent = math.Max(extent, distance)
		}
		if active && extent <= tolerance {
			return brep.Solid{}, nil
		}
	}
	polygons := make([][]int, 0, len(planes))
	for _, p := range planes {
		var ids []int
		center := geom.Vec3{}
		for i, v := range points {
			if math.Abs(p.n.Dot(v)-p.d) <= tolerance {
				ids = append(ids, i)
			}
		}
		if len(ids) < 3 {
			continue
		}
		// Recompute the center as a convex average of only the active face vertices.
		center = geom.Vec3{}
		for _, i := range ids {
			center = center.Add(points[i].Scale(1 / float64(len(ids))))
		}
		axis := geom.Vec3{X: 1}
		if math.Abs(p.n.X) > 0.8 {
			axis = geom.Vec3{Y: 1}
		}
		u, _ := axis.Cross(p.n).Unit()
		v := p.n.Cross(u)
		sort.SliceStable(ids, func(i, j int) bool {
			a, b := points[ids[i]].Sub(center), points[ids[j]].Sub(center)
			return math.Atan2(a.Dot(v), a.Dot(u)) < math.Atan2(b.Dot(v), b.Dot(u))
		})
		// A face with collinear boundary vertices indicates an ill-conditioned or
		// redundant reconstruction; let the B-rep validator reject it, not create T-junctions.
		polygons = append(polygons, ids)
	}
	if len(polygons) < 4 {
		return brep.Solid{}, nil
	}
	vertices := make([]geom.Vec3, len(points))
	for i, p := range points {
		vertices[i] = p.Add(origin)
		if !vertices[i].IsFinite() {
			return brep.Solid{}, fmt.Errorf("csg: output coordinate overflows")
		}
	}
	var edges []brep.Edge
	faces := make([]brep.Face, len(polygons))
	lookup := map[[2]int]int{}
	for i, polygon := range polygons {
		for j, a := range polygon {
			b := polygon[(j+1)%len(polygon)]
			lo, hi := a, b
			if lo > hi {
				lo, hi = hi, lo
			}
			key := [2]int{lo, hi}
			id, ok := lookup[key]
			if !ok {
				id = len(edges)
				lookup[key] = id
				edges = append(edges, brep.Edge{Start: lo, End: hi})
			}
			faces[i].Boundary = append(faces[i].Boundary, brep.EdgeUse{Edge: id, Reversed: a > b})
		}
	}
	result, err := brep.NewSolid(vertices, edges, faces)
	if err != nil {
		return brep.Solid{}, fmt.Errorf("csg: unstable boundary reconstruction: %w", err)
	}
	volume := result.Tessellate().SignedVolume()
	if math.IsNaN(volume) || math.IsInf(volume, 0) || volume <= 0 {
		return brep.Solid{}, fmt.Errorf("csg: reconstructed boundary has nonpositive volume")
	}
	return result, nil
}

func convexPlanes(s brep.Solid, origin geom.Vec3, tolerance float64) ([]plane, error) {
	if len(s.Vertices()) > maxVertices || len(s.Faces()) > MaxPlanes {
		return nil, fmt.Errorf("input work limit exceeded")
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	topology := s.Topology()
	if topology.Shells != 1 {
		return nil, fmt.Errorf("requires one connected shell")
	}
	if topology.Faces > MaxPlanes {
		return nil, fmt.Errorf("supporting-plane limit exceeded")
	}
	volume := s.Tessellate().SignedVolume()
	if math.IsNaN(volume) || math.IsInf(volume, 0) || volume <= 0 {
		return nil, fmt.Errorf("requires outward orientation")
	}
	vertices := s.Vertices()
	result := make([]plane, topology.Faces)
	for i := range result {
		supporting, err := s.FacePlane(i)
		if err != nil {
			return nil, err
		}
		n := supporting.Normal()
		d := n.Dot(supporting.Origin().Sub(origin))
		if math.IsNaN(d) || math.IsInf(d, 0) {
			return nil, fmt.Errorf("coordinate range overflows")
		}
		for _, v := range vertices {
			distance := n.Dot(v.Sub(origin)) - d
			if math.IsNaN(distance) || math.IsInf(distance, 0) || distance > tolerance {
				return nil, fmt.Errorf("requires convex outward faces")
			}
		}
		result[i] = plane{n: n, d: d}
	}
	return result, nil
}
