package collision

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

var ErrEmpty = errors.New("collision: distance to empty boundary is undefined")
var ErrDistanceRange = errors.New("collision: distance outside float64 range")

// DistanceOptions bounds exact BVH node-pair lower-bound evaluations, including
// pruned children. It no longer counts every pair in a flat triangle product.
// Zero uses 100000 pairs and 4096 triangles per operand; negative values are
// invalid. Exact AABB separation cannot omit a closer candidate through rounding.
type DistanceOptions struct{ MaxPairs, MaxTriangles int }

// DistanceResult reports boundary separation, not distance between occupied
// volumes. Nested boundaries may therefore have positive distance. ExactPoints
// and SquaredDistance are caller-owned rationals and determine the answer;
// Distance and PointA/PointB are rounded convenience values. Rounded witnesses
// may move off the exact face or coincide for extremely close features.
// Tied minima choose the first candidate in tessellation order.
type DistanceResult struct {
	Distance                           float64
	SquaredDistance                    *big.Rat
	PointA, PointB                     geom.Vec3
	ExactPointA, ExactPointB           [3]*big.Rat
	FaceA, FaceB, TriangleA, TriangleB int
	// Pairs counts BVH node bounds; NarrowPhasePairs counts triangle predicates.
	Pairs, NarrowPhasePairs int
}
type closest struct {
	a, b    point
	squared scalar
}

func (c *closest) consider(a, b point) {
	d := minus(a, b)
	s := dot(d, d)
	if c.squared == nil || s.Cmp(c.squared) < 0 {
		c.a, c.b, c.squared = a, b, s
	}
}
func projection(p, a, b point) point {
	d := minus(b, a)
	t := div(dot(minus(p, a), d), dot(d, d))
	if t.Sign() <= 0 {
		return a
	}
	if t.Cmp(big.NewRat(1, 1)) >= 0 {
		return b
	}
	return interpolate(a, b, t)
}
func (c *closest) edges(a0, a1, b0, b1 point) {
	c.consider(a0, projection(a0, b0, b1))
	c.consider(a1, projection(a1, b0, b1))
	c.consider(projection(b0, a0, a1), b0)
	c.consider(projection(b1, a0, a1), b1)
	u, v, w := minus(a1, a0), minus(b1, b0), minus(a0, b0)
	a, b, cc := dot(u, u), dot(u, v), dot(v, v)
	d, e := dot(u, w), dot(v, w)
	den := sub(mul(a, cc), mul(b, b))
	if den.Sign() == 0 {
		return
	}
	s := div(sub(mul(b, e), mul(cc, d)), den)
	t := div(sub(mul(a, e), mul(b, d)), den)
	one := big.NewRat(1, 1)
	if s.Sign() > 0 && t.Sign() > 0 && s.Cmp(one) < 0 && t.Cmp(one) < 0 {
		c.consider(interpolate(a0, a1, s), interpolate(b0, b1, t))
	}
}
func faceProjection(p point, t [3]point) (point, bool) {
	n := triangleNormal(t)
	x := minus(p, scale(n, div(dot(minus(p, t[0]), n), dot(n, n))))
	for i, a := range t {
		if dot(cross(minus(t[(i+1)%3], a), minus(x, a)), n).Sign() < 0 {
			return point{}, false
		}
	}
	return x, true
}
func closestTriangles(a, b [3]geom.Vec3) closest {
	if kind, ps := triangleIntersection(a, b); kind != Disjoint {
		return closest{ps[0], ps[0], new(big.Rat)}
	}
	aa, bb := exactTriangle(a), exactTriangle(b)
	var c closest
	for _, p := range aa {
		if q, ok := faceProjection(p, bb); ok {
			c.consider(p, q)
		}
	}
	for _, p := range bb {
		if q, ok := faceProjection(p, aa); ok {
			c.consider(q, p)
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			c.edges(aa[i], aa[(i+1)%3], bb[j], bb[(j+1)%3])
		}
	}
	return c
}
func distanceResult(c closest) (DistanceResult, error) {
	// Converting the squared distance first would underflow for small finite gaps
	// or overflow even when the distance itself is representable.
	f := new(big.Float).SetPrec(256).SetRat(c.squared)
	f.Sqrt(f)
	distance, _ := f.Float64()
	if math.IsInf(distance, 0) || (distance == 0 && c.squared.Sign() != 0) {
		return DistanceResult{}, ErrDistanceRange
	}
	r := DistanceResult{Distance: distance, SquaredDistance: new(big.Rat).Set(c.squared), FaceA: -1, FaceB: -1, TriangleA: -1, TriangleB: -1}
	for i := 0; i < 3; i++ {
		r.ExactPointA[i] = new(big.Rat).Set(c.a[i])
		r.ExactPointB[i] = new(big.Rat).Set(c.b[i])
	}
	pts := rounded([]point{c.a, c.b})
	r.PointA, r.PointB = pts[0], pts[1]
	return r, nil
}

// TriangleDistance gives exact closest-point classification for two finite,
// nondegenerate triangles. Face and triangle indices are -1. Crossing and
// coplanar overlap return zero with a common exact witness.
func TriangleDistance(a, b [3]geom.Vec3) (DistanceResult, error) {
	if e := validateTriangle(a); e != nil {
		return DistanceResult{}, e
	}
	if e := validateTriangle(b); e != nil {
		return DistanceResult{}, e
	}
	return distanceResult(closestTriangles(a, b))
}

type distanceItem struct {
	t      [3]geom.Vec3
	lo, hi point
}

func distanceItems(ctx context.Context, s brep.Solid, limit int) ([]distanceItem, error) {
	m := s.Tessellate()
	if len(m.Triangles) > limit || len(m.Triangles) > MaxTriangles {
		return nil, ErrLimit
	}
	result := make([]distanceItem, len(m.Triangles))
	for i, t := range m.Triangles {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		d := &result[i]
		for j, k := range t {
			d.t[j] = m.Vertices[k]
		}
		if e := validateTriangle(d.t); e != nil {
			return nil, e
		}
		et := exactTriangle(d.t)
		for k := 0; k < 3; k++ {
			d.lo[k], d.hi[k] = et[0][k], et[0][k]
			for j := 1; j < 3; j++ {
				if et[j][k].Cmp(d.lo[k]) < 0 {
					d.lo[k] = et[j][k]
				}
				if et[j][k].Cmp(d.hi[k]) > 0 {
					d.hi[k] = et[j][k]
				}
			}
		}
	}
	return result, nil
}
func separation(a, b distanceItem) scalar {
	s := new(big.Rat)
	for k := 0; k < 3; k++ {
		var d scalar
		if a.lo[k].Cmp(b.hi[k]) > 0 {
			d = sub(a.lo[k], b.hi[k])
		} else if b.lo[k].Cmp(a.hi[k]) > 0 {
			d = sub(b.lo[k], a.hi[k])
		} else {
			continue
		}
		s = add(s, mul(d, d))
	}
	return s
}

// BoundaryDistance returns the global minimum over the two tessellated planar
// boundaries, preserving original polygon-face ancestry. Concave faces and holes
// are handled by the BRep tessellation. It does not certify either operand's
// embedding or material-shell orientation: the query concerns their surfaces.
// Deterministic BVH pair search uses exact AABB bounds, remaining quadratic in
// the worst case. Equal-distance candidates preserve original triangle-order
// tie selection. Cancellation is checked during tree construction and search.
// MaxPairs counts node-pair bounds; NarrowPhasePairs counts triangle tests. Failure returns no
// partial minimum. Empty boundaries return ErrEmpty.
func BoundaryDistance(ctx context.Context, a, b brep.Solid, o DistanceOptions) (DistanceResult, error) {
	if ctx == nil || o.MaxPairs < 0 || o.MaxTriangles < 0 {
		return DistanceResult{}, ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 4096
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 100000
	}
	if e := ctx.Err(); e != nil {
		return DistanceResult{}, e
	}
	aa, e := distanceItems(ctx, a, o.MaxTriangles)
	if e != nil {
		return DistanceResult{}, e
	}
	bb, e := distanceItems(ctx, b, o.MaxTriangles)
	if e != nil {
		return DistanceResult{}, e
	}
	if len(aa) == 0 || len(bb) == 0 {
		return DistanceResult{}, ErrEmpty
	}
	best, ai, bi, pairs, narrow, e := searchDistanceTrees(ctx, aa, bb, o.MaxPairs)
	if e != nil {
		return DistanceResult{}, e
	}
	r, e := distanceResult(best)
	if e != nil {
		return DistanceResult{}, e
	}
	r.TriangleA, r.TriangleB = ai, bi
	r.FaceA, r.FaceB = a.TriangleFaces()[ai], b.TriangleFaces()[bi]
	r.Pairs, r.NarrowPhasePairs = pairs, narrow
	return r, nil
}
