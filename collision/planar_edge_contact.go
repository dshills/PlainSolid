package collision

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/geom"
)

// PlanarEdgeContactOptions bounds the two material validations and the
// cross-boundary proof separately.
type PlanarEdgeContactOptions struct {
	Validation LocationOptions
	Cross      Options
	// MaxQueryWork bounds each query on a prepared pair. Zero selects
	// two million units.
	MaxQueryWork int
}

// PlanarEdgeContact identifies the full original edge in each independently
// valid planar body and the two original incident faces of each edge.
// Start and End follow the direction of EdgeA. ParameterA and ParameterB
// are the exact original [0,1] edge parameters of those two endpoints.
type PlanarEdgeContact struct {
	EdgeA, EdgeB             int
	FacesA, FacesB           [2]int
	Start, End               geom.Vec3
	Reversed                 bool
	ParameterA, ParameterB   [2]*big.Rat
	SignedVolume             *big.Rat
	BroadPairs, ContactPairs int
}

// DiscoverPlanarEdgeContact finds the one full original edge common to both
// bodies and verifies it is their only contact. No edge is welded. A partial
// edge overlap, zero or multiple matching edges, or other material overlap
// is outside this entry point.
func DiscoverPlanarEdgeContact(ctx context.Context, a, b brep.Solid, o PlanarEdgeContactOptions) (PlanarEdgeContact, error) {
	return discoverPlanarEdgeContact(ctx, a, b, o, nil)
}

func discoverPlanarEdgeContact(ctx context.Context, a, b brep.Solid, o PlanarEdgeContactOptions, prepared *[2]vertexContactComponent) (PlanarEdgeContact, error) {
	if ctx == nil {
		return PlanarEdgeContact{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return PlanarEdgeContact{}, err
	}
	av, bv := a.Vertices(), b.Vertices()
	ea, eb := a.Edges(), b.Edges()
	keys := make(map[[2]geom.Vec3]int, len(ea))
	key := func(a, b geom.Vec3) [2]geom.Vec3 {
		if a.X > b.X || a.X == b.X && (a.Y > b.Y || a.Y == b.Y && a.Z > b.Z) {
			return [2]geom.Vec3{b, a}
		}
		return [2]geom.Vec3{a, b}
	}
	for i, e := range ea {
		if err := ctx.Err(); err != nil {
			return PlanarEdgeContact{}, err
		}
		k := key(av[e.Start], av[e.End])
		if _, exists := keys[k]; exists {
			return PlanarEdgeContact{}, fmt.Errorf("%w: coincident original edges in one part", ErrInvalid)
		}
		keys[k] = i
	}
	pair := [2]int{-1, -1}
	for j, e := range eb {
		if err := ctx.Err(); err != nil {
			return PlanarEdgeContact{}, err
		}
		if i, ok := keys[key(bv[e.Start], bv[e.End])]; ok {
			if pair[0] >= 0 {
				return PlanarEdgeContact{}, fmt.Errorf("%w: contact requires exactly one full original edge", ErrInvalid)
			}
			pair = [2]int{i, j}
		}
	}
	if pair[0] < 0 {
		return PlanarEdgeContact{}, fmt.Errorf("%w: no common full original edge", ErrInvalid)
	}
	return validatePlanarEdgeContact(ctx, a, b, pair[0], pair[1], o, true, prepared)
}

// ValidatePlanarEdgeContact verifies that two independently outward,
// single-shell planar bodies have disjoint interiors and meet on precisely
// one complete original segment. All cross-boundary intersections are exact
// and must be covered by the declared segment. The proof publishes nothing
// if it cannot inspect the entire boundary.
func ValidatePlanarEdgeContact(ctx context.Context, a, b brep.Solid, edgeA, edgeB int, o PlanarEdgeContactOptions) (PlanarEdgeContact, error) {
	return validatePlanarEdgeContact(ctx, a, b, edgeA, edgeB, o, true, nil)
}

// ValidatePlanarEdgeIntervalContact accepts a positive common subinterval of
// two specified original straight edges and proves it is their only contact.
// The original edge ranges remain separate.
func ValidatePlanarEdgeIntervalContact(ctx context.Context, a, b brep.Solid, edgeA, edgeB int, o PlanarEdgeContactOptions) (PlanarEdgeContact, error) {
	return validatePlanarEdgeContact(ctx, a, b, edgeA, edgeB, o, false, nil)
}

// DiscoverPlanarEdgeIntervalContact finds one pair of original edges with
// a positive shared interval. Exact line keys avoid comparing unrelated
// edges. The complete boundary proof still requires that the recovered
// interval is the only inter-body contact.
func DiscoverPlanarEdgeIntervalContact(ctx context.Context, a, b brep.Solid, o PlanarEdgeContactOptions) (PlanarEdgeContact, error) {
	var zero PlanarEdgeContact
	if ctx == nil || o.Cross.MaxPairs < 0 {
		return zero, ErrInvalid
	}
	limit := o.Cross.MaxPairs
	if limit == 0 {
		limit = DefaultMaxPairs
	}
	av, bv := a.Vertices(), b.Vertices()
	type candidate struct {
		id     int
		lo, hi scalar
	}
	first := make(map[[6]string][]candidate)
	for i, e := range a.Edges() {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		key, lo, hi, err := exactPlanarEdgeLineKey(av[e.Start], av[e.End])
		if err != nil {
			return zero, err
		}
		first[key] = append(first[key], candidate{i, lo, hi})
	}
	pair := [2]int{-1, -1}
	for j, e := range b.Edges() {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		key, lo, hi, err := exactPlanarEdgeLineKey(bv[e.Start], bv[e.End])
		if err != nil {
			return zero, err
		}
		for _, c := range first[key] {
			if limit == 0 {
				return zero, ErrLimit
			}
			limit--
			if c.lo.Cmp(hi) >= 0 || lo.Cmp(c.hi) >= 0 {
				continue
			}
			if pair[0] >= 0 {
				return zero, fmt.Errorf("%w: interval contact requires one original edge pair", ErrInvalid)
			}
			pair = [2]int{c.id, j}
		}
	}
	if pair[0] < 0 {
		return zero, fmt.Errorf("%w: no positive original edge interval in common", ErrInvalid)
	}
	return ValidatePlanarEdgeIntervalContact(ctx, a, b, pair[0], pair[1], o)
}

func exactPlanarEdgeLineKey(a, b geom.Vec3) (key [6]string, lo, hi scalar, err error) {
	p, q := exact(a), exact(b)
	d := minus(q, p)
	axis := nonzeroAxis(d)
	if axis < 0 {
		return key, nil, nil, ErrInvalid
	}
	for i := range d {
		direction := div(d[i], d[axis])
		intercept := sub(p[i], mul(p[axis], direction))
		key[i], key[i+3] = direction.RatString(), intercept.RatString()
	}
	lo, hi = p[axis], q[axis]
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	return key, lo, hi, nil
}

func validatePlanarEdgeContact(ctx context.Context, a, b brep.Solid, edgeA, edgeB int, o PlanarEdgeContactOptions, full bool, prepared *[2]vertexContactComponent) (PlanarEdgeContact, error) {
	var zero PlanarEdgeContact
	if ctx == nil || o.Cross.MaxPairs < 0 || o.Cross.MaxContacts < 0 || o.MaxQueryWork < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	opts, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	as, bs := a.Edges(), b.Edges()
	if edgeA < 0 || edgeA >= len(as) || edgeB < 0 || edgeB >= len(bs) {
		return zero, ErrInvalid
	}
	ea, eb := as[edgeA], bs[edgeB]
	av, bv := a.Vertices(), b.Vertices()
	ac, bc := [2]geom.Vec3{av[ea.Start], av[ea.End]}, [2]geom.Vec3{bv[eb.Start], bv[eb.End]}
	if full && ac != bc && ac != ([2]geom.Vec3{bc[1], bc[0]}) {
		return zero, fmt.Errorf("%w: original edge endpoints do not coincide", ErrInvalid)
	}
	ends, intervalA, intervalB, err := planarEdgeCommonInterval(ac, bc)
	if err != nil {
		return zero, err
	}
	ma, mb := a.Tessellate(), b.Tessellate()
	if len(ma.Triangles) > opts.MaxTriangles-len(mb.Triangles) {
		return zero, ErrLimit
	}
	la, err := NewMeshPointLocator(ctx, ma, a.TriangleFaces(), opts)
	if err != nil {
		return zero, err
	}
	lb, err := NewMeshPointLocator(ctx, mb, b.TriangleFaces(), opts)
	if err != nil {
		return zero, err
	}
	if len(la.shells) != 1 || len(lb.shells) != 1 {
		return zero, fmt.Errorf("%w: edge contact requires two single-shell parts", ErrInvalid)
	}
	pa, pb := exact(ends[0]), exact(ends[1])
	axis := nonzeroAxis(minus(pb, pa))
	width := sub(pb[axis], pa[axis])
	parameter := func(p point) scalar { return div(sub(p[axis], pa[axis]), width) }
	var intervals [][2]scalar
	count := 0
	r, err := sweepIntersections(ctx, ma, mb, false, o.Cross, func(_, _ int, kind Kind, points []point) error {
		if kind != Touching || len(points) < 1 || len(points) > 2 {
			return fmt.Errorf("%w: intersection outside the declared segment", ErrInvalid)
		}
		for _, p := range points {
			if !onSegment(p, pa, pb) {
				return fmt.Errorf("%w: contact outside the declared segment", ErrInvalid)
			}
		}
		count++
		if len(points) == 2 {
			low, high := parameter(points[0]), parameter(points[1])
			if low.Cmp(high) > 0 {
				low, high = high, low
			}
			intervals = append(intervals, [2]scalar{low, high})
		}
		return nil
	})
	if err != nil {
		return zero, err
	}
	if !r.Complete {
		return zero, ErrLimit
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i][0].Cmp(intervals[j][0]) < 0 })
	covered := new(big.Rat)
	for _, interval := range intervals {
		if covered.Cmp(interval[0]) < 0 {
			return zero, fmt.Errorf("%w: contact interval has an uncovered gap", ErrInvalid)
		}
		if covered.Cmp(interval[1]) < 0 {
			covered = interval[1]
		}
	}
	if covered.Cmp(big.NewRat(1, 1)) != 0 {
		return zero, fmt.Errorf("%w: contact does not cover the declared edge", ErrInvalid)
	}
	boundsA, err := geom.Bounds(av)
	if err != nil {
		return zero, err
	}
	boundsB, err := geom.Bounds(bv)
	if err != nil {
		return zero, err
	}
	ca := vertexContactComponent{ma, boundsA, la}
	cb := vertexContactComponent{mb, boundsB, lb}
	// Removing one embedded closed arc from the boundary of a connected
	// closed shell does not disconnect it. Material membership is constant
	// on the remainder because the sweep proved there are no other contacts.
	skip := func(v []geom.Vec3, edge brep.Edge) map[int]bool {
		return map[int]bool{edge.Start: onSegment(exact(v[edge.Start]), pa, pb), edge.End: onSegment(exact(v[edge.End]), pa, pb)}
	}
	if err := validateComponentOutside(ctx, ca, cb, skip(av, ea)); err != nil {
		return zero, err
	}
	if err := validateComponentOutside(ctx, cb, ca, skip(bv, eb)); err != nil {
		return zero, err
	}
	fa, err := planarEdgeContactFaces(a, edgeA)
	if err != nil {
		return zero, err
	}
	fb, err := planarEdgeContactFaces(b, edgeB)
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if prepared != nil {
		*prepared = [2]vertexContactComponent{ca, cb}
	}
	return PlanarEdgeContact{
		EdgeA: edgeA, EdgeB: edgeB, FacesA: fa, FacesB: fb, Start: ends[0], End: ends[1],
		ParameterA: intervalA, ParameterB: intervalB,
		Reversed: intervalB[0].Cmp(intervalB[1]) > 0, SignedVolume: add(la.shells[0].SignedVolume, lb.shells[0].SignedVolume),
		BroadPairs: r.BroadPhasePairs, ContactPairs: count,
	}, nil
}

func planarEdgeCommonInterval(a, b [2]geom.Vec3) ([2]geom.Vec3, [2]scalar, [2]scalar, error) {
	fail := func(reason string) ([2]geom.Vec3, [2]scalar, [2]scalar, error) {
		return [2]geom.Vec3{}, [2]scalar{}, [2]scalar{}, fmt.Errorf("%w: %s", ErrInvalid, reason)
	}
	p, q := exact(a[0]), exact(a[1])
	d := minus(q, p)
	axis := nonzeroAxis(d)
	if axis < 0 || b[0] == b[1] {
		return fail("original edge has zero extent")
	}
	var t [2]scalar
	for i, v := range b {
		offset := minus(exact(v), p)
		if nonzeroAxis(cross(offset, d)) >= 0 {
			return fail("original edge supports are not exactly collinear")
		}
		t[i] = div(offset[axis], d[axis])
	}
	low, high := 0, 1
	if t[low].Cmp(t[high]) > 0 {
		low, high = high, low
	}
	endpoints := a
	pa := [2]scalar{new(big.Rat), big.NewRat(1, 1)}
	if t[low].Cmp(pa[0]) > 0 {
		pa[0], endpoints[0] = t[low], b[low]
	}
	if t[high].Cmp(pa[1]) < 0 {
		pa[1], endpoints[1] = t[high], b[high]
	}
	if pa[0].Cmp(pa[1]) >= 0 {
		return fail("original edges have no positive common interval")
	}
	w := sub(t[1], t[0])
	pb := [2]scalar{div(sub(pa[0], t[0]), w), div(sub(pa[1], t[0]), w)}
	return endpoints, pa, pb, nil
}

func planarEdgeContactFaces(s brep.Solid, edge int) ([2]int, error) {
	var owners [2]int
	n := 0
	for i, face := range s.Faces() {
		for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
			for _, use := range loop {
				if use.Edge == edge {
					if n == 2 || n != 0 && i == owners[0] {
						return [2]int{}, ErrInvalid
					}
					owners[n] = i
					n++
				}
			}
		}
	}
	if n != 2 {
		return [2]int{}, ErrInvalid
	}
	return owners, nil
}
