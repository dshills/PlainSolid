package collision

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

// PlanarFaceContactOptions bounds local embedding, cross-boundary tests,
// and checks against the declared original perimeter wires.
type PlanarFaceContactOptions struct {
	Validation LocationOptions
	Cross      Options
	// Zero MaxPerimeterChecks selects one million comparisons.
	MaxPerimeterChecks int
}

// PlanarFaceContact is an exact full-face incidence proof between two
// separate materials. Vertex and edge pairs follow the first face's outer
// boundary and each inner wire in order. WireOffsets delimit those cycles
// in both arrays, starting at zero and ending at their common length.
type PlanarFaceContact struct {
	FaceA, FaceB       int
	Vertices, Edges    [][2]int
	WireOffsets        []int
	DropAxis           int
	TwiceProjectedArea *big.Rat
	SignedVolume       *big.Rat
	BroadPairs         int
}

// ValidatePlanarFaceContact proves that the only contact between two
// outward single-shell parts is precisely the specified matching original
// face. Concave faces are supported. The two face cycles must have opposite
// orientation and identical complete boundaries, including every inner wire.
func ValidatePlanarFaceContact(ctx context.Context, a, b brep.Solid, faceA, faceB int, o PlanarFaceContactOptions) (PlanarFaceContact, error) {
	m, err := ValidatePlanarFaceContactSet(ctx, a, b, [][2]int{{faceA, faceB}}, o)
	if err != nil {
		return PlanarFaceContact{}, err
	}
	return m.Faces[0], nil
}

// PlanarFaceContactSet proves that two single-shell bodies meet along the
// declared complete original faces. Original adjacent matches may share edges
// and vertices; the common faces carry opposite outward orientations.
type PlanarFaceContactSet struct {
	Faces        []PlanarFaceContact
	SignedVolume *big.Rat
	BroadPairs   int
}

type planarFaceBoundary struct {
	PlanarFaceContact
	perimeter []point
}

// ValidatePlanarFaceContactSet validates all matches in one cross-boundary
// sweep and proves there is no other contact or common interior.
func ValidatePlanarFaceContactSet(ctx context.Context, a, b brep.Solid, pairs [][2]int, o PlanarFaceContactOptions) (PlanarFaceContactSet, error) {
	var zero PlanarFaceContactSet
	if ctx == nil || len(pairs) == 0 || o.Cross.MaxContacts < 0 || o.Cross.MaxPairs < 0 || o.MaxPerimeterChecks < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	options, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	af, bf := a.Faces(), b.Faces()
	if len(pairs) > min(len(af), len(bf)) {
		return zero, ErrInvalid
	}
	av, bv := a.Vertices(), b.Vertices()
	ae, be := a.Edges(), b.Edges()
	ma, mb := a.Tessellate(), b.Tessellate()
	if len(ma.Triangles) > options.MaxTriangles-len(mb.Triangles) {
		return zero, ErrLimit
	}
	am, bm := a.TriangleFaces(), b.TriangleFaces()
	la, err := NewMeshPointLocator(ctx, ma, am, options)
	if err != nil {
		return zero, err
	}
	lb, err := NewMeshPointLocator(ctx, mb, bm, options)
	if err != nil {
		return zero, err
	}
	if len(la.shells) != 1 || len(lb.shells) != 1 {
		return zero, fmt.Errorf("%w: original face contact requires two single-shell parts", ErrInvalid)
	}
	match := make(map[[2]int]int, len(pairs))
	usedA, usedB := make([]bool, len(af)), make([]bool, len(bf))
	seams := make([]planarFaceBoundary, 0, len(pairs))
	vertexMap, edgeMap := [2]map[int]int{make(map[int]int), make(map[int]int)}, [2]map[int]int{make(map[int]int), make(map[int]int)}
	type segment [2]point
	perimeter := make([]segment, 0)
	for _, pair := range pairs {
		i, j := pair[0], pair[1]
		if i < 0 || i >= len(af) || j < 0 || j >= len(bf) || usedA[i] || usedB[j] {
			return zero, fmt.Errorf("%w: declared planar face pairs must be distinct and one-to-one", ErrInvalid)
		}
		p, err := matchPlanarFaceBoundary(ctx, av, bv, ae, be, af[i], bf[j])
		if err != nil {
			return zero, err
		}
		p.FaceA, p.FaceB = i, j
		match[pair] = len(seams)
		seams = append(seams, p)
		usedA[i], usedB[j] = true, true
		for _, v := range p.Vertices {
			if _, ok := bindPlanarFaceIdentity(vertexMap, v); !ok {
				return zero, fmt.Errorf("%w: inconsistent matched vertex incidence", ErrInvalid)
			}
		}
		for w := 0; w+1 < len(p.WireOffsets); w++ {
			from, to := p.WireOffsets[w], p.WireOffsets[w+1]
			for k := from; k < to; k++ {
				newEdge, ok := bindPlanarFaceIdentity(edgeMap, p.Edges[k])
				if !ok {
					return zero, fmt.Errorf("%w: inconsistent matched edge incidence", ErrInvalid)
				}
				if newEdge {
					perimeter = append(perimeter, segment{p.perimeter[k], p.perimeter[from+(k-from+1)%(to-from)]})
				}
			}
		}
	}
	remaining := o.MaxPerimeterChecks
	if remaining == 0 {
		remaining = DefaultMaxPairs
	}
	haveArea := make([]big.Rat, len(seams))
	crossing, err := sweepIntersections(ctx, ma, mb, false, o.Cross, func(i, j int, k Kind, ps []point) error {
		id, declared := match[[2]int{am[i], bm[j]}]
		if k == CoplanarOverlap && declared {
			haveArea[id].Add(&haveArea[id], absoluteProjectedFaceArea2(ps, seams[id].DropAxis))
			return nil
		}
		if k != Touching || len(ps) == 0 || len(ps) > 2 {
			return fmt.Errorf("%w: intersection outside the declared common faces", ErrInvalid)
		}
		if declared {
			return nil
		}
		for _, edge := range perimeter {
			if remaining == 0 {
				return ErrLimit
			}
			remaining--
			if onSegment(ps[0], edge[0], edge[1]) && (len(ps) == 1 || onSegment(ps[1], edge[0], edge[1])) {
				return nil
			}
		}
		return fmt.Errorf("%w: side contact leaves the original common perimeters", ErrInvalid)
	})
	if err != nil {
		return zero, err
	}
	if !crossing.Complete {
		return zero, ErrLimit
	}
	for i, p := range seams {
		if haveArea[i].Cmp(p.TwiceProjectedArea) != 0 {
			return zero, fmt.Errorf("%w: triangle overlaps do not cover complete original face pair %d", ErrInvalid, i)
		}
	}
	if err := faceContactExterior(ctx, la, lb, af, len(ae), usedA, options.MaxWork); err != nil {
		return zero, err
	}
	if err := faceContactExterior(ctx, lb, la, bf, len(be), usedB, options.MaxWork); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out := PlanarFaceContactSet{
		Faces:        make([]PlanarFaceContact, len(seams)),
		SignedVolume: add(la.shells[0].SignedVolume, lb.shells[0].SignedVolume),
		BroadPairs:   crossing.BroadPhasePairs,
	}
	for i, p := range seams {
		out.Faces[i] = p.PlanarFaceContact
		out.Faces[i].SignedVolume = new(big.Rat).Set(out.SignedVolume)
		out.Faces[i].BroadPairs = out.BroadPairs
	}
	return out, nil
}

// bindPlanarFaceIdentity requires the same original incidence on every
// neighboring match, in both directions.
func bindPlanarFaceIdentity(index [2]map[int]int, pair [2]int) (added, ok bool) {
	a, hasA := index[0][pair[0]]
	b, hasB := index[1][pair[1]]
	if hasA || hasB {
		return false, hasA && hasB && a == pair[1] && b == pair[0]
	}
	index[0][pair[0]], index[1][pair[1]] = pair[1], pair[0]
	return true, true
}

func matchPlanarFaceBoundary(ctx context.Context, av, bv []geom.Vec3, ae, be []brep.Edge, fa, fb brep.Face) (planarFaceBoundary, error) {
	if len(fa.Holes) != len(fb.Holes) || len(fa.Boundary) < 3 || len(fa.Boundary) != len(fb.Boundary) {
		return planarFaceBoundary{}, fmt.Errorf("%w: expected matching complete perimeter wires", ErrInvalid)
	}
	start := func(u brep.EdgeUse, edges []brep.Edge) int {
		e := edges[u.Edge]
		if u.Reversed {
			return e.End
		}
		return e.Start
	}
	loopsA := append([][]brep.EdgeUse{fa.Boundary}, fa.Holes...)
	loopsB := append([][]brep.EdgeUse{fb.Boundary}, fb.Holes...)
	stations := make(map[geom.Vec3][2]int)
	for w, loop := range loopsB {
		for j, u := range loop {
			stations[bv[start(u, be)]] = [2]int{w, j}
		}
	}
	var vertices, edges [][2]int
	var perimeter []point
	offsets := []int{0}
	used := make([]bool, len(loopsB))
	for w, loop := range loopsA {
		if err := ctx.Err(); err != nil {
			return planarFaceBoundary{}, err
		}
		q, ok := stations[av[start(loop[0], ae)]]
		other := loopsB[q[0]]
		if !ok || (w == 0) != (q[0] == 0) || used[q[0]] || len(loop) != len(other) {
			return planarFaceBoundary{}, fmt.Errorf("%w: original perimeter wires differ", ErrInvalid)
		}
		used[q[0]] = true
		n := len(loop)
		for i, u := range loop {
			j := (q[1] + n - i) % n
			va, vb := start(u, ae), start(other[j], be)
			if av[va] != bv[vb] {
				return planarFaceBoundary{}, fmt.Errorf("%w: perimeter wires must match with opposite directions", ErrInvalid)
			}
			vertices = append(vertices, [2]int{va, vb})
			edges = append(edges, [2]int{u.Edge, other[(j+n-1)%n].Edge})
			perimeter = append(perimeter, exact(av[va]))
		}
		offsets = append(offsets, len(perimeter))
	}
	outer := perimeter[:offsets[1]]
	normal := point{new(big.Rat), new(big.Rat), new(big.Rat)}
	for i := 1; i+1 < len(outer); i++ {
		normal = cross(minus(outer[i], outer[0]), minus(outer[i+1], outer[0]))
		if nonzeroAxis(normal) >= 0 {
			break
		}
	}
	drop := nonzeroAxis(normal)
	if drop < 0 {
		return planarFaceBoundary{}, fmt.Errorf("%w: original face has no area", ErrInvalid)
	}
	for _, p := range perimeter {
		if dot(minus(p, perimeter[0]), normal).Sign() != 0 {
			return planarFaceBoundary{}, fmt.Errorf("%w: original common face is not exactly planar", ErrInvalid)
		}
	}
	wantArea := absoluteProjectedFaceArea2(outer, drop)
	for w := 1; w+1 < len(offsets); w++ {
		wantArea.Sub(wantArea, absoluteProjectedFaceArea2(perimeter[offsets[w]:offsets[w+1]], drop))
	}
	if wantArea.Sign() <= 0 {
		return planarFaceBoundary{}, fmt.Errorf("%w: original common face has zero projected area", ErrInvalid)
	}
	return planarFaceBoundary{PlanarFaceContact: PlanarFaceContact{
		Vertices: vertices, Edges: edges, WireOffsets: offsets, DropAxis: drop, TwiceProjectedArea: wantArea,
	}, perimeter: perimeter}, nil
}

func absoluteProjectedFaceArea2(p []point, drop int) *big.Rat {
	area := new(big.Rat)
	for i := 1; i+1 < len(p); i++ {
		area.Add(area, orient(p[0], p[i], p[i+1], drop))
	}
	return area.Abs(area)
}

// Each connected component of the boundary after removing the common faces
// (including their perimeters) must lie outside the other part. Joining remaining
// faces across original edges enumerates these components, even when a hole
// disconnects them. An exact triangle centroid lies strictly inside its
// original face. The prior cross sweep proved it cannot be on the other shell.
func faceContactExterior(ctx context.Context, a, b *PointLocator, faces []brep.Face, edgeCount int, common []bool, maxWork int) error {
	budget := locationBudget{ctx: ctx, limit: maxWork}
	parents := make([]int, len(faces))
	owner := make([]int, edgeCount)
	for i := range parents {
		parents[i] = i
	}
	for i := range owner {
		owner[i] = -1
	}
	find := func(i int) int {
		for parents[i] != i {
			parents[i] = parents[parents[i]]
			i = parents[i]
		}
		return i
	}
	for f, face := range faces {
		if common[f] {
			continue
		}
		for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
			for _, u := range loop {
				if err := budget.tick(1); err != nil {
					return err
				}
				k := owner[u.Edge]
				if k < 0 {
					owner[u.Edge] = f
				} else {
					parents[find(f)] = find(k)
				}
			}
		}
	}
	checked := make([]bool, len(faces))
	third := big.NewRat(1, 3)
	for _, t := range a.triangles {
		if err := budget.tick(1); err != nil {
			return err
		}
		if common[t.face] {
			continue
		}
		root := find(t.face)
		if checked[root] {
			continue
		}
		checked[root] = true
		q := point{}
		for i := range q {
			q[i] = mul(add(add(t.v[0][i], t.v[1][i]), t.v[2][i]), third)
		}
		hits, _, err := b.ray(q, nil, &budget)
		if err != nil {
			return err
		}
		winding := 0
		for _, h := range hits {
			winding += h.sign
		}
		if winding != 0 {
			return fmt.Errorf("%w: material overlaps behind the declared common face", ErrInvalid)
		}
	}
	return nil
}
