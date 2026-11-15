package brep

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ConvexOffsetOptions bounds the number of supporting faces and the work
// of certifying their halfspace intersections. Zero selects 64 faces and
// four million plane tests.
type ConvexOffsetOptions struct {
	MaxFaces, MaxPlaneTests int
}

// ConvexOffsetHistory maps each source face to its translated output face.
// An entry of -1 identifies a plane with no positive-area output facet.
type ConvexOffsetHistory struct {
	Faces      []int
	PlaneTests int
}

// OffsetConvex offsets the supporting planes of a single outward convex
// polyhedron by the same signed normal distance. Adjacent facets meet at
// their exact translated plane intersections (miter joins). Positive
// distance grows material and negative distance erodes it. Collapse to
// zero volume returns the empty solid.
//
// Each source plane must own one convex face. For nonzero distance its
// normal length must be rational, and all resulting vertices must be
// exactly representable. An unrepresentable result returns an error.
func (s Solid) OffsetConvex(ctx context.Context, distance float64, o ConvexOffsetOptions) (Solid, ConvexOffsetHistory, error) {
	if ctx == nil || math.IsNaN(distance) || math.IsInf(distance, 0) {
		return Solid{}, ConvexOffsetHistory{}, geom.ErrInvalid
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 64
	}
	if o.MaxPlaneTests == 0 {
		o.MaxPlaneTests = 4000000
	}
	if o.MaxFaces < 4 || o.MaxFaces > 64 || o.MaxPlaneTests < 1 || o.MaxPlaneTests > 10000000 {
		return Solid{}, ConvexOffsetHistory{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, ConvexOffsetHistory{}, e
	}
	if len(s.faces) == 0 {
		return Solid{}, ConvexOffsetHistory{}, nil
	}
	if len(s.faces) > o.MaxFaces || len(s.vertices) > 2*len(s.faces)-4 {
		return Solid{}, ConvexOffsetHistory{}, fmt.Errorf("brep: convex offset input exceeds supporting face bound: %w", geom.ErrDomain)
	}
	work := offsetPlaneWork{o.MaxPlaneTests, 0}
	planes, e := s.convexOffsetPlanes(ctx, new(big.Rat).SetFloat64(distance), &work)
	if e != nil {
		return Solid{}, ConvexOffsetHistory{}, e
	}
	h := ConvexOffsetHistory{Faces: make([]int, len(planes))}
	if distance == 0 {
		for i := range planes {
			h.Faces[i] = i
		}
		h.PlaneTests = work.tests
		return s, h, nil
	}
	points, incident, e := intersectOffsetHalfspaces(ctx, planes, &work)
	if e != nil {
		return Solid{}, ConvexOffsetHistory{}, e
	}
	var polygons [][]int
	for i, p := range planes {
		h.Faces[i] = -1
		ring := offsetFacetHull(points, incident[i], p.n)
		if len(ring) >= 3 {
			h.Faces[i] = len(polygons)
			polygons = append(polygons, ring)
		}
	}
	h.PlaneTests = work.tests
	if len(polygons) < 4 {
		for i := range h.Faces {
			h.Faces[i] = -1
		}
		return Solid{}, h, nil
	}
	vertices := make([]geom.Vec3, len(points))
	for i, p := range points {
		vertices[i], e = geom.RepresentedExactPoint([3]*big.Rat(p))
		if e != nil {
			return Solid{}, ConvexOffsetHistory{}, fmt.Errorf("brep: translated supporting planes meet at vertex %d: %w", i, e)
		}
	}
	if e = ctx.Err(); e != nil {
		return Solid{}, ConvexOffsetHistory{}, e
	}
	out, e := fromPolygons(vertices, polygons)
	if e != nil {
		return Solid{}, ConvexOffsetHistory{}, e
	}
	return out, h, nil
}

type offsetPlaneWork struct{ limit, tests int }

func (w *offsetPlaneWork) test() error {
	if w.tests == w.limit {
		return fmt.Errorf("brep: convex offset plane test limit: %w", geom.ErrConvergence)
	}
	w.tests++
	return nil
}

type offsetVector [3]*big.Rat

func (a offsetVector) sub(b offsetVector) offsetVector {
	return offsetVector{new(big.Rat).Sub(a[0], b[0]), new(big.Rat).Sub(a[1], b[1]), new(big.Rat).Sub(a[2], b[2])}
}

func (a offsetVector) dot(b offsetVector) *big.Rat {
	z := new(big.Rat)
	for i := range a {
		z.Add(z, new(big.Rat).Mul(a[i], b[i]))
	}
	return z
}

func (a offsetVector) cross(b offsetVector) (z offsetVector) {
	for i := range a {
		j, k := (i+1)%3, (i+2)%3
		z[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
	}
	return z
}

type offsetPlane struct {
	n offsetVector
	d *big.Rat
}

func (s Solid) convexOffsetPlanes(ctx context.Context, distance *big.Rat, work *offsetPlaneWork) ([]offsetPlane, error) {
	points := make([]offsetVector, len(s.vertices))
	for i, p := range s.vertices {
		points[i] = offsetVector{new(big.Rat).SetFloat64(p.X), new(big.Rat).SetFloat64(p.Y), new(big.Rat).SetFloat64(p.Z)}
	}
	planes := make([]offsetPlane, 0, len(s.faces))
	for i, face := range s.faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		ids, e := s.faceIndices(face)
		if e != nil || len(face.Holes) != 0 {
			return nil, fmt.Errorf("brep: convex offset requires a single convex wire on source face %d: %w", i, geom.ErrDomain)
		}
		a, b, c := points[ids[0]], points[ids[1]], points[ids[2]]
		n := b.sub(a).cross(c.sub(a))
		n2 := n.dot(n)
		if n2.Sign() == 0 {
			return nil, geom.ErrDegenerate
		}
		d := n.dot(a)
		faceVertices := make(map[int]bool, len(ids))
		for k, id := range ids {
			p, q, r := points[id], points[ids[(k+1)%len(ids)]], points[ids[(k+2)%len(ids)]]
			if q.sub(p).cross(r.sub(q)).dot(n).Sign() <= 0 {
				return nil, fmt.Errorf("brep: concave or degenerate source face %d: %w", i, geom.ErrDomain)
			}
			faceVertices[id] = true
		}
		for j, p := range points {
			if e := work.test(); e != nil {
				return nil, e
			}
			side := n.dot(p).Cmp(d)
			if faceVertices[j] && side != 0 || !faceVertices[j] && side >= 0 {
				return nil, fmt.Errorf("brep: source face %d must own the full supporting facet: %w", i, geom.ErrDomain)
			}
		}
		if distance.Sign() != 0 {
			length, ok := exactOffsetSqrt(n2)
			if !ok {
				return nil, fmt.Errorf("brep: source face %d has irrational normal length: %w", i, geom.ErrUnrepresentable)
			}
			d.Add(d, length.Mul(length, distance))
		}
		planes = append(planes, offsetPlane{n, d})
	}
	return planes, nil
}

func exactOffsetSqrt(r *big.Rat) (*big.Rat, bool) {
	n, d := new(big.Int).Sqrt(r.Num()), new(big.Int).Sqrt(r.Denom())
	if new(big.Int).Mul(n, n).Cmp(r.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(r.Denom()) != 0 {
		return nil, false
	}
	return new(big.Rat).SetFrac(n, d), true
}

func intersectOffsetHalfspaces(ctx context.Context, planes []offsetPlane, work *offsetPlaneWork) ([]offsetVector, [][]int, error) {
	var points []offsetVector
	incident := make([][]int, len(planes))
	seen := make(map[[3]string]bool)
	for i, a := range planes {
		for j := i + 1; j < len(planes); j++ {
			b := planes[j]
			ab := a.n.cross(b.n)
			for _, c := range planes[j+1:] {
				if e := ctx.Err(); e != nil {
					return nil, nil, e
				}
				if e := work.test(); e != nil {
					return nil, nil, e
				}
				det := c.n.dot(ab)
				if det.Sign() == 0 {
					continue
				}
				bc, ca := b.n.cross(c.n), c.n.cross(a.n)
				var p offsetVector
				for m := range p {
					n := new(big.Rat).Mul(a.d, bc[m])
					n.Add(n, new(big.Rat).Mul(b.d, ca[m]))
					n.Add(n, new(big.Rat).Mul(c.d, ab[m]))
					p[m] = n.Quo(n, det)
				}
				var owners []int
				inside := true
				for n, q := range planes {
					if e := work.test(); e != nil {
						return nil, nil, e
					}
					side := q.n.dot(p).Cmp(q.d)
					if side > 0 {
						inside = false
						break
					}
					if side == 0 {
						owners = append(owners, n)
					}
				}
				if !inside {
					continue
				}
				key := [3]string{p[0].RatString(), p[1].RatString(), p[2].RatString()}
				if seen[key] {
					continue
				}
				seen[key] = true
				for _, owner := range owners {
					incident[owner] = append(incident[owner], len(points))
				}
				points = append(points, p)
			}
		}
	}
	return points, incident, nil
}

func offsetFacetHull(points []offsetVector, ids []int, normal offsetVector) []int {
	if len(ids) < 3 {
		return nil
	}
	axis := 0
	for normal[axis].Sign() == 0 {
		axis++
	}
	x, y := (axis+1)%3, (axis+2)%3
	slices.SortFunc(ids, func(i, j int) int {
		if s := points[i][x].Cmp(points[j][x]); s != 0 {
			return s
		}
		return points[i][y].Cmp(points[j][y])
	})
	turn := func(i, j, k int) int {
		return points[j].sub(points[i]).cross(points[k].sub(points[i]))[axis].Sign()
	}
	chain := func() []int {
		var path []int
		for _, id := range ids {
			for len(path) >= 2 && turn(path[len(path)-2], path[len(path)-1], id) <= 0 {
				path = path[:len(path)-1]
			}
			path = append(path, id)
		}
		return path[:len(path)-1]
	}
	lower := chain()
	slices.Reverse(ids)
	ring := append(lower, chain()...)
	if normal[axis].Sign() < 0 {
		slices.Reverse(ring)
	}
	return ring
}
