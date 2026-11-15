package collision

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

// PlanarRegionContact is the coincidence proof between two selected sets of
// complete original faces. Original face divisions may differ. Only the
// outer seams need matching original vertex and edge identities.
type PlanarRegionContact struct {
	Faces               [2][]int
	TwiceProjectedAreas [2][]*big.Rat
	DropAxes            [2][]int
	Vertices, Edges     [][2]int
	SignedVolume        *big.Rat
	BroadPairs          int
}

type planarRegionFace struct {
	normal point
	drop   int
	area   *big.Rat
}

// ValidatePlanarRegionContact proves that two single-shell planar bodies meet
// only along the supplied original face regions. Each face must be covered
// completely by oppositely oriented material on the other body. The external
// seams must match edge for edge; internal subdivisions need not match.
func ValidatePlanarRegionContact(ctx context.Context, a, b brep.Solid, faces [2][]int, o PlanarFaceContactOptions) (PlanarRegionContact, error) {
	var zero PlanarRegionContact
	if ctx == nil || len(faces[0]) == 0 || len(faces[1]) == 0 || o.Cross.MaxContacts < 0 || o.Cross.MaxPairs < 0 || o.MaxPerimeterChecks < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	opt, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	am, bm := a.Tessellate(), b.Tessellate()
	if len(am.Triangles) > opt.MaxTriangles-len(bm.Triangles) {
		return zero, ErrLimit
	}
	fm := [2][]int{a.TriangleFaces(), b.TriangleFaces()}
	af := [2][]brep.Face{a.Faces(), b.Faces()}
	ed := [2][]brep.Edge{a.Edges(), b.Edges()}
	vs := [2][]geom.Vec3{am.Vertices, bm.Vertices}
	use := [2][]bool{make([]bool, len(af[0])), make([]bool, len(af[1]))}
	region := [2][]planarRegionFace{make([]planarRegionFace, len(af[0])), make([]planarRegionFace, len(af[1]))}
	count := [2][]uint8{make([]uint8, len(ed[0])), make([]uint8, len(ed[1]))}
	have := [2][]big.Rat{make([]big.Rat, len(af[0])), make([]big.Rat, len(af[1]))}
	out := PlanarRegionContact{}
	for i, fs := range faces {
		if len(fs) > len(af[i]) {
			return zero, ErrInvalid
		}
		for _, n := range fs {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			if n < 0 || n >= len(af[i]) || use[i][n] {
				return zero, fmt.Errorf("%w: region must declare distinct original face IDs", ErrInvalid)
			}
			use[i][n] = true
			q, err := measurePlanarRegionFace(af[i][n], ed[i], vs[i], count[i])
			if err != nil {
				return zero, err
			}
			region[i][n] = q
			out.TwiceProjectedAreas[i] = append(out.TwiceProjectedAreas[i], q.area)
			out.DropAxes[i] = append(out.DropAxes[i], q.drop)
		}
		out.Faces[i] = append([]int(nil), fs...)
	}
	edges, vertices, seams, err := pairOriginalRegionSeams(ed, vs, count)
	if err != nil {
		return zero, err
	}
	out.Edges, out.Vertices = edges, vertices
	la, err := NewMeshPointLocator(ctx, am, fm[0], opt)
	if err != nil {
		return zero, err
	}
	lb, err := NewMeshPointLocator(ctx, bm, fm[1], opt)
	if err != nil {
		return zero, err
	}
	if len(la.shells) != 1 || len(lb.shells) != 1 {
		return zero, fmt.Errorf("%w: planar region contact requires two single-shell bodies", ErrInvalid)
	}
	remaining := o.MaxPerimeterChecks
	if remaining == 0 {
		remaining = DefaultMaxPairs
	}
	report, err := sweepIntersections(ctx, am, bm, false, o.Cross, func(i, j int, k Kind, ps []point) error {
		f, g := fm[0][i], fm[1][j]
		selected := use[0][f] && use[1][g]
		if k == CoplanarOverlap && selected {
			p, q := region[0][f], region[1][g]
			if dot(p.normal, q.normal).Sign() >= 0 {
				return fmt.Errorf("%w: coincident contact regions do not oppose", ErrInvalid)
			}
			have[0][f].Add(&have[0][f], absoluteProjectedFaceArea2(ps, p.drop))
			have[1][g].Add(&have[1][g], absoluteProjectedFaceArea2(ps, q.drop))
			return nil
		}
		if k != Touching || len(ps) == 0 || len(ps) > 2 {
			return fmt.Errorf("%w: crossing or undeclared region overlap", ErrInvalid)
		}
		if selected {
			return nil
		}
		for _, s := range seams {
			if remaining == 0 {
				return ErrLimit
			}
			remaining--
			if onSegment(ps[0], s[0], s[1]) && (len(ps) == 1 || onSegment(ps[1], s[0], s[1])) {
				return nil
			}
		}
		return fmt.Errorf("%w: exterior contact leaves the original interface seam", ErrInvalid)
	})
	if err != nil {
		return zero, err
	}
	if !report.Complete {
		return zero, ErrLimit
	}
	for i, fs := range faces {
		for _, n := range fs {
			if have[i][n].Cmp(region[i][n].area) != 0 {
				return zero, fmt.Errorf("%w: opposing sources do not cover all of body %d face %d", ErrInvalid, i, n)
			}
		}
	}
	if err := faceContactExterior(ctx, la, lb, af[0], len(ed[0]), use[0], opt.MaxWork); err != nil {
		return zero, err
	}
	if err := faceContactExterior(ctx, lb, la, af[1], len(ed[1]), use[1], opt.MaxWork); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.SignedVolume = add(la.shells[0].SignedVolume, lb.shells[0].SignedVolume)
	out.BroadPairs = report.BroadPhasePairs
	return out, nil
}

func measurePlanarRegionFace(f brep.Face, e []brep.Edge, v []geom.Vec3, counts []uint8) (planarRegionFace, error) {
	var q planarRegionFace
	var origin point
	for w, wire := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
		p := make([]point, len(wire))
		for i, u := range wire {
			counts[u.Edge]++
			id := e[u.Edge].Start
			if u.Reversed {
				id = e[u.Edge].End
			}
			p[i] = exact(v[id])
		}
		if w == 0 {
			origin = p[0]
			q.normal = point{new(big.Rat), new(big.Rat), new(big.Rat)}
			for i := 1; i+1 < len(p); i++ {
				q.normal = plus(q.normal, cross(minus(p[i], origin), minus(p[i+1], origin)))
			}
			q.drop = nonzeroAxis(q.normal)
			if q.drop < 0 {
				return planarRegionFace{}, fmt.Errorf("%w: original face has no exact oriented area", ErrInvalid)
			}
			q.area = absoluteProjectedFaceArea2(p, q.drop)
		} else {
			q.area.Sub(q.area, absoluteProjectedFaceArea2(p, q.drop))
		}
		for _, x := range p {
			if dot(minus(x, origin), q.normal).Sign() != 0 {
				return planarRegionFace{}, fmt.Errorf("%w: original contact region is not exactly planar", ErrInvalid)
			}
		}
	}
	if q.area.Sign() <= 0 {
		return planarRegionFace{}, fmt.Errorf("%w: contact region has no projected area", ErrInvalid)
	}
	return q, nil
}

func pairOriginalRegionSeams(e [2][]brep.Edge, v [2][]geom.Vec3, count [2][]uint8) (edges, vertices [][2]int, seams [][2]point, err error) {
	fail := func() ([][2]int, [][2]int, [][2]point, error) {
		return nil, nil, nil, fmt.Errorf("%w: contact regions have different original exterior seams", ErrInvalid)
	}
	key := func(part, i int) [2]geom.Vec3 {
		edge := e[part][i]
		a, b := v[part][edge.Start], v[part][edge.End]
		if lexVecLess(b, a) {
			a, b = b, a
		}
		return [2]geom.Vec3{a, b}
	}
	index := make(map[[2]geom.Vec3]int)
	for i, n := range count[1] {
		if n != 1 {
			continue
		}
		k := key(1, i)
		if _, ok := index[k]; ok {
			return fail()
		}
		index[k] = i
	}
	seen := [2]map[int]int{make(map[int]int), make(map[int]int)}
	for i, n := range count[0] {
		if n != 1 {
			continue
		}
		k := key(0, i)
		j, ok := index[k]
		if !ok {
			return fail()
		}
		delete(index, k)
		a, b := e[0][i], e[1][j]
		if v[0][a.Start] != v[1][b.Start] {
			b.Start, b.End = b.End, b.Start
		}
		for _, pair := range [2][2]int{{a.Start, b.Start}, {a.End, b.End}} {
			new, ok := bindPlanarFaceIdentity(seen, pair)
			if !ok {
				return fail()
			}
			if new {
				vertices = append(vertices, pair)
			}
		}
		edges = append(edges, [2]int{i, j})
		seams = append(seams, [2]point{exact(k[0]), exact(k[1])})
	}
	if len(index) != 0 || len(edges) == 0 {
		return fail()
	}
	return edges, vertices, seams, nil
}
