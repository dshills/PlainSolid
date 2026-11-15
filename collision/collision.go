// Package collision detects exact intersections of finite, nondegenerate indexed
// triangles. It distinguishes proper crossings, coplanar area overlaps, and
// boundary contacts. It is not a proximity or tolerance-based gap detector.
package collision

import (
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

type Kind uint8

const (
	Disjoint Kind = iota
	// Touching means intersection only on at least one triangle's boundary.
	Touching
	// Crossing means a noncoplanar segment meets both relative interiors.
	Crossing
	// CoplanarOverlap means a coplanar intersection has positive area.
	CoplanarOverlap
)

// Intersection classifies the represented float64 inputs exactly, using rational
// arithmetic. Points contain one point, a segment, or an ordered overlap polygon.
// Output coordinates are rounded to float64 and can coincide at extreme scales;
// the Kind classification is made before rounding and remains authoritative.
type Intersection struct {
	Kind   Kind
	Points []geom.Vec3
}
type Contact struct {
	TriangleA, TriangleB int
	Intersection
}

// Options bounds broadphase comparisons and reported contacts. Zero values use
// defaults. Negative values are invalid. Limits return ErrLimit with Complete=false.
type Options struct{ MaxPairs, MaxContacts int }

const MaxTriangles = 100000
const DefaultMaxPairs = 1000000
const DefaultMaxContacts = 10000

var ErrLimit = errors.New("collision: work or contact limit exceeded")
var ErrInvalid = errors.New("collision: invalid mesh or options")

// Report is partial when an error is returned. Complete is set only after all
// broadphase candidates have been examined. BroadPhasePairs counts sweep-axis
// interval comparisons, including pairs rejected along the other two axes.
type Report struct {
	Contacts                          []Contact
	BroadPhasePairs, NarrowPhasePairs int
	Complete                          bool
}

// Triangles performs exact triangle-triangle classification without a broadphase.
// Nonfinite or exactly collinear input is rejected, including repeated vertices.
func Triangles(a, b [3]geom.Vec3) (Intersection, error) {
	if err := validateTriangle(a); err != nil {
		return Intersection{}, err
	}
	if err := validateTriangle(b); err != nil {
		return Intersection{}, err
	}
	kind, points := triangleIntersection(a, b)
	return Intersection{Kind: kind, Points: rounded(points)}, nil
}
func validateTriangle(t [3]geom.Vec3) error {
	for _, p := range t {
		if !p.IsFinite() {
			return fmt.Errorf("%w: nonfinite vertex", ErrInvalid)
		}
	}
	if nonzeroAxis(triangleNormal(exactTriangle(t))) < 0 {
		return fmt.Errorf("%w: degenerate triangle", ErrInvalid)
	}
	return nil
}

// MeshIntersections reports every contacting pair from the two meshes, including
// boundary-only contacts. It does not assume that either mesh is closed or manifold.
func MeshIntersections(a, b mesh.Mesh, options Options) (Report, error) {
	return intersections(a, b, false, options)
}

// SelfIntersections omits a triangle paired with itself and expected contact
// confined to a shared indexed vertex or edge. Duplicate triangles, coplanar
// overlap, and intersections extending beyond the shared feature are reported.
// Coincident coordinates with different vertex indices are not shared topology.
// This function complements, and does not replace, mesh.ValidateSolid.
func SelfIntersections(m mesh.Mesh, options Options) (Report, error) {
	return intersections(m, m, true, options)
}

type item struct {
	triangle, group int
	min, max        [3]float64
}

func items(m mesh.Mesh, group int) ([]item, error) {
	for _, v := range m.Vertices {
		if !v.IsFinite() {
			return nil, fmt.Errorf("%w: nonfinite vertex", ErrInvalid)
		}
	}
	result := make([]item, len(m.Triangles))
	for i, ids := range m.Triangles {
		for _, id := range ids {
			if id < 0 || id >= len(m.Vertices) {
				return nil, fmt.Errorf("%w: triangle %d index out of range", ErrInvalid, i)
			}
		}
		t := [3]geom.Vec3{m.Vertices[ids[0]], m.Vertices[ids[1]], m.Vertices[ids[2]]}
		if err := validateTriangle(t); err != nil {
			return nil, fmt.Errorf("triangle %d: %w", i, err)
		}
		result[i] = item{triangle: i, group: group, min: [3]float64{t[0].X, t[0].Y, t[0].Z}, max: [3]float64{t[0].X, t[0].Y, t[0].Z}}
		for _, p := range t[1:] {
			for axis, v := range [3]float64{p.X, p.Y, p.Z} {
				result[i].min[axis] = math.Min(result[i].min[axis], v)
				result[i].max[axis] = math.Max(result[i].max[axis], v)
			}
		}
	}
	return result, nil
}
func triangle(m mesh.Mesh, index int) [3]geom.Vec3 {
	ids := m.Triangles[index]
	return [3]geom.Vec3{m.Vertices[ids[0]], m.Vertices[ids[1]], m.Vertices[ids[2]]}
}
func intersections(a, b mesh.Mesh, self bool, options Options) (Report, error) {
	return sweepIntersections(nil, a, b, self, options, nil)
}

// The optional visitor sees the exact contact points before any float64
// conversion. A nil context keeps the public legacy sweep behavior.
func sweepIntersections(ctx context.Context, a, b mesh.Mesh, self bool, options Options, visit func(int, int, Kind, []point) error) (Report, error) {
	var report Report
	if options.MaxPairs < 0 || options.MaxContacts < 0 {
		return report, ErrInvalid
	}
	if options.MaxPairs == 0 {
		options.MaxPairs = DefaultMaxPairs
	}
	if options.MaxContacts == 0 {
		options.MaxContacts = DefaultMaxContacts
	}
	count := len(a.Triangles)
	if !self {
		count += len(b.Triangles)
	}
	if count > MaxTriangles {
		return report, ErrLimit
	}
	all, err := items(a, 0)
	if err != nil {
		return report, err
	}
	if !self {
		second, e := items(b, 1)
		if e != nil {
			return report, e
		}
		all = append(all, second...)
		if len(a.Triangles) == 0 || len(b.Triangles) == 0 {
			report.Complete = true
			return report, nil
		}
	}
	if len(all) < 2 {
		report.Complete = true
		return report, nil
	}
	low, high := all[0].min, all[0].max
	for _, it := range all[1:] {
		for axis := 0; axis < 3; axis++ {
			low[axis] = math.Min(low[axis], it.min[axis])
			high[axis] = math.Max(high[axis], it.max[axis])
		}
	}
	axis := 0
	for i := 1; i < 3; i++ {
		if high[i]-low[i] > high[axis]-low[axis] {
			axis = i
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].min[axis] < all[j].min[axis] })
	contactCount := 0
	for i, left := range all {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return report, err
			}
		}
		for _, right := range all[i+1:] {
			if right.min[axis] > left.max[axis] {
				break
			}
			if report.BroadPhasePairs >= options.MaxPairs {
				return report, ErrLimit
			}
			if ctx != nil {
				if err := ctx.Err(); err != nil {
					return report, err
				}
			}
			report.BroadPhasePairs++
			if !self && left.group == right.group {
				continue
			}
			overlap := true
			for k := 0; k < 3; k++ {
				if left.max[k] < right.min[k] || right.max[k] < left.min[k] {
					overlap = false
					break
				}
			}
			if !overlap {
				continue
			}
			ta, tb := left.triangle, right.triangle
			if self {
				if ta > tb {
					ta, tb = tb, ta
				}
			} else if left.group == 1 {
				ta, tb = tb, ta
			}
			report.NarrowPhasePairs++
			kind, points := triangleIntersection(triangle(a, ta), triangle(b, tb))
			if kind == Disjoint {
				continue
			}
			if self && expectedSharedContact(a, ta, tb, kind, points) {
				continue
			}
			if contactCount >= options.MaxContacts {
				return report, ErrLimit
			}
			contactCount++
			if visit != nil {
				if err := visit(ta, tb, kind, points); err != nil {
					return report, err
				}
			} else {
				report.Contacts = append(report.Contacts, Contact{TriangleA: ta, TriangleB: tb, Intersection: Intersection{Kind: kind, Points: rounded(points)}})
			}
		}
	}
	sort.Slice(report.Contacts, func(i, j int) bool {
		a, b := report.Contacts[i], report.Contacts[j]
		if a.TriangleA != b.TriangleA {
			return a.TriangleA < b.TriangleA
		}
		return a.TriangleB < b.TriangleB
	})
	report.Complete = true
	return report, nil
}
func expectedSharedContact(m mesh.Mesh, a, b int, kind Kind, points []point) bool {
	if kind != Touching {
		return false
	}
	var shared []int
	for _, i := range m.Triangles[a] {
		for _, j := range m.Triangles[b] {
			if i == j {
				shared = append(shared, i)
			}
		}
	}
	if len(shared) == 1 {
		return len(points) == 1 && same(points[0], exact(m.Vertices[shared[0]]))
	}
	if len(shared) == 2 {
		start, end := exact(m.Vertices[shared[0]]), exact(m.Vertices[shared[1]])
		for _, p := range points {
			if !onSegment(p, start, end) {
				return false
			}
		}
		return true
	}
	return false
}
