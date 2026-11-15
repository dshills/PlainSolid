package collision

import (
	"context"
	"fmt"
	"math/big"
	"math/bits"
	"slices"
	"sort"

	"plainsolid/geom"
)

// RationalLinePartitionOptions bounds the station and span proof. Zero
// MaxWork uses the locator's query budget. MaxInputBits defaults to 4096 and
// caps each original rational numerator and denominator.
type RationalLinePartitionOptions struct{ MaxWork, MaxInputBits int }

// RationalLineStation records one exact raw line parameter. BoundaryFaces
// contains the sorted original face IDs at that point, including all owners
// on coincident face or seam spans.
type RationalLineStation struct {
	Parameter     *big.Rat
	Location      Location
	BoundaryFaces []int
}

// RationalLineSpan records one open interval between adjacent Stations.
// BoundaryFaces names all original faces incident on the entire open span.
type RationalLineSpan struct {
	Start, End    int
	Location      Location
	BoundaryFaces []int
}

// RationalLinePartition is the complete exact partition of a finite closed
// line range. Inside, Outside and OnBoundary are explicit. The first/last
// stations are the query bounds, even when their membership is Outside.
type RationalLinePartition struct {
	Stations []RationalLineStation
	Spans    []RationalLineSpan
	Work     int
}

// PartitionLine converts represented inputs to exact rationals and preserves
// the caller's raw parameter units. Direction is never normalized.
func (l *PointLocator) PartitionLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64, o RationalLinePartitionOptions) (RationalLinePartition, error) {
	lo, hi := new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT)
	if !origin.IsFinite() || !direction.IsFinite() || lo == nil || hi == nil {
		return RationalLinePartition{}, ErrInvalid
	}
	return l.PartitionRationalLine(ctx, exact(origin), exact(direction), lo, hi, o)
}

// PartitionRationalLine records all isolated boundary crossings, coplanar
// spans and their original face ownership for origin+t*direction. It proves
// each planar triangle's finite line intersection and classifies complementary
// cells by exact oriented winding. Bounding coordinates and all station/span
// work share one budget. No partial partition is returned on failure.
func (l *PointLocator) PartitionRationalLine(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o RationalLinePartitionOptions) (RationalLinePartition, error) {
	var zero RationalLinePartition
	if l == nil || ctx == nil || minT == nil || maxT == nil || o.MaxWork < 0 || o.MaxInputBits < 0 || o.MaxInputBits > 1<<16 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if o.MaxInputBits == 0 {
		o.MaxInputBits = 4096
	}
	for _, r := range []*big.Rat{origin[0], origin[1], origin[2], direction[0], direction[1], direction[2], minT, maxT} {
		if r == nil {
			return zero, ErrInvalid
		}
		if r.Num().BitLen() > o.MaxInputBits || r.Denom().BitLen() > o.MaxInputBits {
			return zero, ErrLimit
		}
	}
	if minT.Cmp(maxT) > 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 {
		return zero, ErrInvalid
	}
	if o.MaxWork == 0 {
		o.MaxWork = l.options.MaxWork
		if o.MaxWork == 0 {
			o.MaxWork = 2000000
		}
	}
	b := &locationBudget{ctx: ctx, limit: o.MaxWork}
	lo, hi := new(big.Rat).Set(minT), new(big.Rat).Set(maxT)
	stations := []*big.Rat{lo, hi}
	var intervals []rationalLineFaceRange
	for _, tri := range l.triangles {
		if err := b.tick(1); err != nil {
			return zero, err
		}
		a, z, found := planarTriangleLineRange(tri, origin, direction, lo, hi)
		if found {
			intervals = append(intervals, rationalLineFaceRange{a, z, tri.face})
			stations = append(stations, a)
			if a.Cmp(z) != 0 {
				stations = append(stations, z)
			}
		}
	}
	if err := b.tick(len(stations) * bits.Len(uint(len(stations)))); err != nil {
		return zero, err
	}
	stations = uniqueLineParameters(stations)
	count := len(stations)
	r := RationalLinePartition{Stations: make([]RationalLineStation, count), Spans: make([]RationalLineSpan, count-1)}
	for i, at := range stations {
		which, owners, err := l.classifyLineStation(origin, direction, at, intervals, b)
		if err != nil {
			return zero, err
		}
		r.Stations[i] = RationalLineStation{new(big.Rat).Set(at), which, owners}
		if i == 0 {
			continue
		}
		mid := div(add(stations[i-1], at), big.NewRat(2, 1))
		which, owners, err = l.classifyLineStation(origin, direction, mid, intervals, b)
		if err != nil {
			return zero, err
		}
		r.Spans[i-1] = RationalLineSpan{i - 1, i, which, owners}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	r.Work = b.work
	return removeInternalLineStations(r), nil
}

func uniqueLineParameters(parameters []*big.Rat) []*big.Rat {
	sort.Slice(parameters, func(i, j int) bool { return parameters[i].Cmp(parameters[j]) < 0 })
	count := 0
	for _, p := range parameters {
		if count == 0 || p.Cmp(parameters[count-1]) != 0 {
			parameters[count] = p
			count++
		}
	}
	return parameters[:count]
}

// Remove parameters introduced solely by triangulation of an original face.
// Any change in membership or original owners pins the station in the record.
func removeInternalLineStations(r RationalLinePartition) RationalLinePartition {
	if len(r.Stations) < 3 {
		return r
	}
	out := RationalLinePartition{Stations: make([]RationalLineStation, 1, len(r.Stations)), Spans: make([]RationalLineSpan, 0, len(r.Spans)), Work: r.Work}
	out.Stations[0] = r.Stations[0]
	for i := 1; i < len(r.Stations); i++ {
		station, left := r.Stations[i], r.Spans[i-1]
		if i < len(r.Spans) {
			right := r.Spans[i]
			if station.Location == left.Location && station.Location == right.Location &&
				slices.Equal(station.BoundaryFaces, left.BoundaryFaces) && slices.Equal(station.BoundaryFaces, right.BoundaryFaces) {
				continue
			}
		}
		j := len(out.Stations)
		out.Stations = append(out.Stations, station)
		out.Spans = append(out.Spans, RationalLineSpan{j - 1, j, left.Location, left.BoundaryFaces})
	}
	return out
}

type rationalLineFaceRange struct {
	lo, hi *big.Rat
	face   int
}

func planarTriangleLineRange(t locationTriangle, o, dir point, lo, hi *big.Rat) (*big.Rat, *big.Rat, bool) {
	den, num := dot(t.n, dir), sub(t.d, dot(t.n, o))
	if den.Sign() != 0 {
		at := div(num, den)
		if at.Cmp(lo) < 0 || at.Cmp(hi) > 0 {
			return nil, nil, false
		}
		inside, _ := triangleMembership(plus(o, scale(dir, at)), t)
		return at, at, inside
	}
	if num.Sign() != 0 {
		return nil, nil, false
	}
	// The coplanar line remains inside each oriented triangle half-plane
	// exactly when C+t*D >= 0, so three rational clips exhaust its contact.
	for i, first := range t.v {
		edge := minus(t.v[(i+1)%3], first)
		c := dot(cross(edge, minus(o, first)), t.n)
		d := dot(cross(edge, dir), t.n)
		switch d.Sign() {
		case 0:
			if c.Sign() < 0 {
				return nil, nil, false
			}
		case 1:
			at := div(new(big.Rat).Neg(c), d)
			if at.Cmp(lo) > 0 {
				lo = at
			}
		case -1:
			at := div(new(big.Rat).Neg(c), d)
			if at.Cmp(hi) < 0 {
				hi = at
			}
		}
		if lo.Cmp(hi) > 0 {
			return nil, nil, false
		}
	}
	return lo, hi, true
}

func (l *PointLocator) classifyLineStation(origin, direction point, t *big.Rat, boundaries []rationalLineFaceRange, b *locationBudget) (Location, []int, error) {
	if err := b.tick(1); err != nil {
		return Outside, nil, err
	}
	var owners map[int]bool
	for _, span := range boundaries {
		if err := b.tick(1); err != nil {
			return Outside, nil, err
		}
		if span.lo.Cmp(t) <= 0 && span.hi.Cmp(t) >= 0 {
			if owners == nil {
				owners = make(map[int]bool)
			}
			owners[span.face] = true
		}
	}
	if len(owners) > 0 {
		faces := make([]int, 0, len(owners))
		for face := range owners {
			faces = append(faces, face)
		}
		sort.Ints(faces)
		return OnBoundary, faces, nil
	}
	if len(l.triangles) == 0 {
		return Outside, nil, nil
	}
	hits, _, err := l.ray(plus(origin, scale(direction, t)), nil, b)
	if err != nil {
		return Outside, nil, err
	}
	winding := 0
	for _, hit := range hits {
		winding += hit.sign
	}
	switch winding {
	case 0:
		return Outside, nil, nil
	case 1:
		return Inside, nil, nil
	default:
		return Outside, nil, fmt.Errorf("%w: inconsistent winding of exact line cell", ErrInvalid)
	}
}
