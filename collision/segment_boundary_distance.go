package collision

import (
	"context"
	"math/big"
	"math/bits"
	"sort"

	"plainsolid/geom"
)

// SegmentBoundaryDistanceOptions bounds the complete exact segment query. Zero
// selects the locator's query work limit and 4096 bits per original rational
// numerator or denominator.
type SegmentBoundaryDistanceOptions struct{ MaxWork, MaxInputBits int }

// SegmentBoundaryDistance reports the global exact squared separation from the
// finite segment to the represented planar boundary. Parameter is in [0,1].
// Ties select the earliest segment parameter and then the lexicographically
// first boundary point. Owners contains every face at that exact witness.
type SegmentBoundaryDistance struct {
	SquaredDistance, Parameter  *big.Rat
	SegmentPoint, BoundaryPoint [3]*big.Rat
	Owners                      []AssemblyFace
	Work                        int
}

func (l *PointLocator) DistanceFromSegment(ctx context.Context, start, end geom.Vec3, o SegmentBoundaryDistanceOptions) (SegmentBoundaryDistance, error) {
	if !start.IsFinite() || !end.IsFinite() {
		return SegmentBoundaryDistance{}, ErrInvalid
	}
	return l.DistanceFromRationalSegment(ctx, exact(start), exact(end), o)
}

// DistanceFromRationalSegment measures exact boundary clearance, including
// interior segment/edge minima, parallel face spans and exact contacts.
// A coincident pair of segment endpoints is a point query at parameter zero.
func (l *PointLocator) DistanceFromRationalSegment(ctx context.Context, start, end [3]*big.Rat, o SegmentBoundaryDistanceOptions) (SegmentBoundaryDistance, error) {
	if l == nil {
		return SegmentBoundaryDistance{}, ErrInvalid
	}
	return nearestPlanarSegment(ctx, start, end, []*PointLocator{l}, l.options.MaxWork, o)
}

// DistanceFromRationalSegment measures a finite segment against the proved
// independent component boundaries. A shared vertex retains both owners.
func (l *VertexContactAssemblyLocator) DistanceFromRationalSegment(ctx context.Context, start, end [3]*big.Rat, o SegmentBoundaryDistanceOptions) (SegmentBoundaryDistance, error) {
	if l == nil {
		return SegmentBoundaryDistance{}, ErrInvalid
	}
	parts := make([]*PointLocator, len(l.components))
	for i, c := range l.components {
		parts[i] = c.loc
	}
	return nearestPlanarSegment(ctx, start, end, parts, l.maxWork, o)
}

type planarSegmentCandidate struct {
	squared *big.Rat
	t       *big.Rat
	p, q    point
}

func (c *planarSegmentCandidate) consider(t *big.Rat, p, q point) {
	d := minus(p, q)
	squared := dot(d, d)
	if c.squared == nil || squared.Cmp(c.squared) < 0 || squared.Cmp(c.squared) == 0 &&
		(t.Cmp(c.t) < 0 || t.Cmp(c.t) == 0 && exactSegmentPointLess(q, c.q)) {
		*c = planarSegmentCandidate{squared, t, p, q}
	}
}

func exactSegmentPointLess(a, b point) bool {
	for i, x := range a {
		if c := x.Cmp(b[i]); c != 0 {
			return c < 0
		}
	}
	return false
}

func nearestPlanarSegment(ctx context.Context, start, end point, parts []*PointLocator, maxWork int, o SegmentBoundaryDistanceOptions) (SegmentBoundaryDistance, error) {
	var zero SegmentBoundaryDistance
	if ctx == nil || o.MaxWork < 0 || o.MaxInputBits < 0 || o.MaxInputBits > 1<<16 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if o.MaxInputBits == 0 {
		o.MaxInputBits = 4096
	}
	for _, p := range [2]point{start, end} {
		for _, x := range p {
			if x == nil {
				return zero, ErrInvalid
			}
			if x.Num().BitLen() > o.MaxInputBits || x.Denom().BitLen() > o.MaxInputBits {
				return zero, ErrLimit
			}
		}
	}
	if maxWork == 0 {
		maxWork = 2000000
	}
	if o.MaxWork > 0 {
		maxWork = min(maxWork, o.MaxWork)
	}
	b := &locationBudget{ctx: ctx, limit: maxWork}
	box := exactBoundsForPoints(start, end)
	direction := minus(end, start)
	d2 := dot(direction, direction)
	var best planarSegmentCandidate
	for _, l := range parts {
		for _, tri := range l.triangles {
			if err := b.tick(1); err != nil {
				return zero, err
			}
			if best.squared != nil && separation(box, exactBoundsForPoints(tri.v[:]...)).Cmp(best.squared) > 0 {
				continue
			}
			if err := b.tick(1); err != nil {
				return zero, err
			}
			considerPlanarSegmentTriangle(&best, start, end, direction, d2, tri)
		}
	}
	if best.squared == nil {
		return zero, ErrEmpty
	}
	r := SegmentBoundaryDistance{SquaredDistance: best.squared, Parameter: new(big.Rat).Set(best.t)}
	for i := 0; i < 3; i++ {
		r.SegmentPoint[i], r.BoundaryPoint[i] = new(big.Rat).Set(best.p[i]), new(big.Rat).Set(best.q[i])
	}
	owners := map[AssemblyFace]bool{}
	for solid, l := range parts {
		for _, tri := range l.triangles {
			if err := b.tick(1); err != nil {
				return zero, err
			}
			if dot(tri.n, best.q).Cmp(tri.d) != 0 {
				continue
			}
			if inside, _ := triangleMembership(best.q, tri); inside {
				owners[AssemblyFace{Solid: solid, Face: tri.face}] = true
			}
		}
	}
	if len(owners) == 0 {
		return zero, ErrInvalid
	}
	if err := b.tick(len(owners) * bits.Len(uint(len(owners)))); err != nil {
		return zero, err
	}
	for owner := range owners {
		r.Owners = append(r.Owners, owner)
	}
	sort.Slice(r.Owners, func(i, j int) bool {
		a, b := r.Owners[i], r.Owners[j]
		if a.Solid != b.Solid {
			return a.Solid < b.Solid
		}
		return a.Face < b.Face
	})
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	r.Work = b.work
	return r, nil
}

func exactBoundsForPoints(points ...point) distanceItem {
	r := distanceItem{lo: points[0], hi: points[0]}
	for _, p := range points[1:] {
		for i, x := range p {
			if x.Cmp(r.lo[i]) < 0 {
				r.lo[i] = x
			} else if x.Cmp(r.hi[i]) > 0 {
				r.hi[i] = x
			}
		}
	}
	return r
}

func considerPlanarSegmentTriangle(best *planarSegmentCandidate, start, end, direction point, directionSquared *big.Rat, tri locationTriangle) {
	zero, one := new(big.Rat), big.NewRat(1, 1)
	if directionSquared.Sign() == 0 {
		best.consider(zero, start, nearestPlanarTrianglePoint(start, tri))
		return
	}
	if t, _, hit := planarTriangleLineRange(tri, start, direction, zero, one); hit {
		p := plus(start, scale(direction, t))
		best.consider(t, p, p)
		return
	}
	if best.squared != nil && best.squared.Sign() == 0 {
		return
	}
	best.consider(zero, start, nearestPlanarTrianglePoint(start, tri))
	best.consider(one, end, nearestPlanarTrianglePoint(end, tri))
	if dot(tri.n, direction).Sign() == 0 {
		// An interior segment/interior face minimizer requires parallelism.
		// Under that condition, clip the entire orthogonal projection to
		// retain even a nearest span whose endpoints start outside the face.
		projected := minus(start, scale(tri.n, div(sub(dot(tri.n, start), tri.d), dot(tri.n, tri.n))))
		if t, _, hit := planarTriangleLineRange(tri, projected, direction, zero, one); hit {
			shift := scale(direction, t)
			best.consider(t, plus(start, shift), plus(projected, shift))
		}
	}
	for i, q := range tri.v {
		// Endpoint/segment projections cover both ends of a parallel-edge
		// minimizer; the remaining nonparallel stationary pair is explicit.
		w := minus(start, q)
		d := dot(direction, w)
		station := div(new(big.Rat).Neg(d), directionSquared)
		if station.Sign() > 0 && station.Cmp(one) < 0 {
			best.consider(station, plus(start, scale(direction, station)), q)
		}
		edge := minus(tri.v[(i+1)%3], q)
		b, c, e := dot(direction, edge), dot(edge, edge), dot(edge, w)
		den := sub(mul(directionSquared, c), mul(b, b))
		if den.Sign() == 0 {
			continue
		}
		s, t := div(sub(mul(b, e), mul(c, d)), den), div(sub(mul(directionSquared, e), mul(b, d)), den)
		if s.Sign() > 0 && s.Cmp(one) < 0 && t.Sign() > 0 && t.Cmp(one) < 0 {
			best.consider(s, plus(start, scale(direction, s)), plus(q, scale(edge, t)))
		}
	}
}
