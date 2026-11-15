package collision

import (
	"context"
	"math/big"
	"math/bits"
	"sort"

	"plainsolid/geom"
)

// PointBoundaryDistanceOptions bounds a complete exact boundary query. Zero
// fields select the locator's query work limit, 4096 input bits per rational
// numerator and denominator, and 1024 distinct nearest witnesses.
type PointBoundaryDistanceOptions struct{ MaxWork, MaxInputBits, MaxWitnesses int }

// PointBoundaryWitness represents one exact model point and every original
// component face that owns it. Owners are sorted by component and face.
type PointBoundaryWitness struct {
	Point  [3]*big.Rat
	Owners []AssemblyFace
}

// PointBoundaryDistance contains the global exact squared distance and all
// distinct closest boundary points in lexicographic order. It measures the
// boundary, so a point inside material can have positive distance.
type PointBoundaryDistance struct {
	SquaredDistance *big.Rat
	Witnesses       []PointBoundaryWitness
	Work            int
}

// DistanceToBoundary wraps the rational query for represented input points.
func (l *PointLocator) DistanceToBoundary(ctx context.Context, p geom.Vec3, o PointBoundaryDistanceOptions) (PointBoundaryDistance, error) {
	if !p.IsFinite() {
		return PointBoundaryDistance{}, ErrInvalid
	}
	return l.DistanceToBoundaryRational(ctx, exact(p), o)
}

// DistanceToBoundaryRational returns all exact nearest points of the validated
// represented planar boundary, including ties across original faces, shells or
// trim edges. A limit, empty material or cancellation yields no partial result.
func (l *PointLocator) DistanceToBoundaryRational(ctx context.Context, p [3]*big.Rat, o PointBoundaryDistanceOptions) (PointBoundaryDistance, error) {
	if l == nil {
		return PointBoundaryDistance{}, ErrInvalid
	}
	return nearestPlanarBoundary(ctx, p, []*PointLocator{l}, l.options.MaxWork, o)
}

// DistanceToBoundaryRational measures the exact boundary of the independent
// components. At a declared shared vertex, one geometric witness retains the
// original component faces on both sides of the contact.
func (l *VertexContactAssemblyLocator) DistanceToBoundaryRational(ctx context.Context, p [3]*big.Rat, o PointBoundaryDistanceOptions) (PointBoundaryDistance, error) {
	if l == nil {
		return PointBoundaryDistance{}, ErrInvalid
	}
	parts := make([]*PointLocator, len(l.components))
	for i, c := range l.components {
		parts[i] = c.loc
	}
	return nearestPlanarBoundary(ctx, p, parts, l.maxWork, o)
}

type boundaryWitnessCandidate struct {
	p      point
	owners map[AssemblyFace]bool
}

func nearestPlanarBoundary(ctx context.Context, p point, parts []*PointLocator, maxWork int, o PointBoundaryDistanceOptions) (PointBoundaryDistance, error) {
	var zero PointBoundaryDistance
	if ctx == nil || o.MaxWork < 0 || o.MaxInputBits < 0 || o.MaxInputBits > 1<<16 || o.MaxWitnesses < 0 || o.MaxWitnesses > MaxTriangles {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if o.MaxInputBits == 0 {
		o.MaxInputBits = 4096
	}
	if o.MaxWitnesses == 0 {
		o.MaxWitnesses = 1024
	}
	for _, x := range p {
		if x == nil {
			return zero, ErrInvalid
		}
		if x.Num().BitLen() > o.MaxInputBits || x.Denom().BitLen() > o.MaxInputBits {
			return zero, ErrLimit
		}
	}
	if maxWork == 0 {
		maxWork = 2000000
	}
	if o.MaxWork > 0 {
		maxWork = min(maxWork, o.MaxWork)
	}
	b := &locationBudget{ctx: ctx, limit: maxWork}
	var best *big.Rat
	var overflow bool
	witnesses := make(map[[3]string]boundaryWitnessCandidate)
	for solid, l := range parts {
		for _, t := range l.triangles {
			if err := b.tick(1); err != nil {
				return zero, err
			}
			if best != nil && triangleBoxPointDistance(t.v, p).Cmp(best) > 0 {
				continue
			}
			if err := b.tick(1); err != nil {
				return zero, err
			}
			q := nearestPlanarTrianglePoint(p, t)
			d := minus(q, p)
			squared := dot(d, d)
			if best != nil && squared.Cmp(best) > 0 {
				continue
			}
			if best == nil || squared.Cmp(best) < 0 {
				best, overflow = squared, false
				clear(witnesses)
			}
			key := [3]string{q[0].RatString(), q[1].RatString(), q[2].RatString()}
			candidate, ok := witnesses[key]
			if !ok {
				if len(witnesses) == o.MaxWitnesses {
					// A later strictly closer candidate resets the interim limit.
					overflow = true
					continue
				}
				candidate = boundaryWitnessCandidate{q, make(map[AssemblyFace]bool)}
				witnesses[key] = candidate
			}
			candidate.owners[AssemblyFace{Solid: solid, Face: t.face}] = true
		}
	}
	if best == nil {
		return zero, ErrEmpty
	}
	if overflow {
		return zero, ErrLimit
	}
	if err := b.tick(len(witnesses) * bits.Len(uint(len(witnesses)))); err != nil {
		return zero, err
	}
	r := PointBoundaryDistance{SquaredDistance: best, Witnesses: make([]PointBoundaryWitness, 0, len(witnesses))}
	for _, candidate := range witnesses {
		entry := PointBoundaryWitness{Owners: make([]AssemblyFace, 0, len(candidate.owners))}
		for i, x := range candidate.p {
			entry.Point[i] = new(big.Rat).Set(x)
		}
		if err := b.tick(len(candidate.owners) * bits.Len(uint(len(candidate.owners)))); err != nil {
			return zero, err
		}
		for owner := range candidate.owners {
			entry.Owners = append(entry.Owners, owner)
		}
		sort.Slice(entry.Owners, func(i, j int) bool {
			a, z := entry.Owners[i], entry.Owners[j]
			if a.Solid != z.Solid {
				return a.Solid < z.Solid
			}
			return a.Face < z.Face
		})
		r.Witnesses = append(r.Witnesses, entry)
	}
	sort.Slice(r.Witnesses, func(i, j int) bool {
		for k, x := range r.Witnesses[i].Point {
			if c := x.Cmp(r.Witnesses[j].Point[k]); c != 0 {
				return c < 0
			}
		}
		return false
	})
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	r.Work = b.work
	return r, nil
}

func triangleBoxPointDistance(triangle [3]point, p point) *big.Rat {
	bound := new(big.Rat)
	for k, x := range p {
		lo, hi := triangle[0][k], triangle[0][k]
		for j := 1; j < 3; j++ {
			if y := triangle[j][k]; y.Cmp(lo) < 0 {
				lo = y
			} else if y.Cmp(hi) > 0 {
				hi = y
			}
		}
		if x.Cmp(lo) < 0 {
			d := sub(lo, x)
			bound.Add(bound, mul(d, d))
		} else if x.Cmp(hi) > 0 {
			d := sub(x, hi)
			bound.Add(bound, mul(d, d))
		}
	}
	return bound
}

func nearestPlanarTrianglePoint(p point, t locationTriangle) point {
	q := minus(p, scale(t.n, div(sub(dot(t.n, p), t.d), dot(t.n, t.n))))
	if inside, _ := triangleMembership(q, t); inside {
		return q
	}
	var near closest
	for i, x := range t.v {
		near.consider(p, projection(p, x, t.v[(i+1)%3]))
	}
	return near.b
}
