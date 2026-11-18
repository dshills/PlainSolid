//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// CoincidentConvexRegionOptions bounds exact endpoint subdivision independently
// of the certified material partition. MaxSegments defaults to 4096, max16384;
// MaxSplitWork defaults to 1000000, max4000000 endpoint-edge, source-use,
// and loop-containment edge checks. Native curve proof limits apply per edge.
type CoincidentConvexRegionOptions struct {
	Material                  CoincidentConvexHoleOptions
	MaxSegments, MaxSplitWork int
}

// BuildCoincidentConvexRegions reconstructs regularized intersection loops from
// positive-area convex cells. It splits every edge at all incident original
// cell vertices, cancels exact opposite intervals, and rejects pinched cycles.
// It returns chart regions only, not solid Boolean assembly or isolated contacts.
func (a Solid) BuildCoincidentConvexRegions(ctx context.Context, faceA int, b Solid, faceB int, o CoincidentConvexRegionOptions) (CoincidentChartRegions, error) {
	fail := func(e error) (CoincidentChartRegions, error) { return CoincidentChartRegions{}, e }
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxSplitWork == 0 {
		o.MaxSplitWork = 1000000
	}
	if o.MaxSegments < 1 || o.MaxSegments > 16384 || o.MaxSplitWork < 1 || o.MaxSplitWork > 4000000 {
		return fail(geom.ErrInvalid)
	}
	overlay, e := a.IntersectCoincidentConvexHoleMaterial(ctx, faceA, b, faceB, o.Material)
	if e != nil {
		return fail(e)
	}
	return a.buildCoincidentConvexCells(ctx, faceA, b, faceB, overlay, o)
}

// buildCoincidentConvexCells consumes only independently proved disjoint-interior
// positive convex cells. Exposed ownership is rechecked against original faces.
func (a Solid) buildCoincidentConvexCells(ctx context.Context, faceA int, b Solid, faceB int, overlay CoincidentConvexHoleMaterial, o CoincidentConvexRegionOptions) (CoincidentChartRegions, error) {
	fail := func(e error) (CoincidentChartRegions, error) { return CoincidentChartRegions{}, e }
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxSplitWork == 0 {
		o.MaxSplitWork = 1000000
	}
	if o.MaxSegments < 1 || o.MaxSegments > 16384 || o.MaxSplitWork < 1 || o.MaxSplitWork > 4000000 {
		return fail(geom.ErrInvalid)
	}
	var e error
	type edge struct {
		a, b   [2]*big.Rat
		owners []CoincidentTrimOwner
	}
	pointKey := func(p [2]*big.Rat) string { return p[0].RatString() + "," + p[1].RatString() }
	key := func(p, q [2]*big.Rat) string { return pointKey(p) + "|" + pointKey(q) }
	var raw []edge
	vertices := map[string][2]*big.Rat{}
	for _, cell := range overlay.Cells {
		for i, p := range cell.A {
			q := cell.A[(i+1)%len(cell.A)]
			raw = append(raw, edge{a: p, b: q})
			vertices[pointKey(p)] = p
			if len(raw) > o.MaxSegments {
				return fail(parametric.ErrBudget)
			}
		}
	}
	live := map[string]edge{}
	canceled, work, segments := 0, 0, 0
	parameter := func(p, x, y [2]*big.Rat) (*big.Rat, bool) {
		axis := 0
		if x[0].Cmp(y[0]) == 0 {
			axis = 1
		}
		delta := new(big.Rat).Sub(y[axis], x[axis])
		if delta.Sign() == 0 {
			return nil, false
		}
		t := new(big.Rat).Quo(new(big.Rat).Sub(p[axis], x[axis]), delta)
		if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
			return nil, false
		}
		for k := 0; k < 2; k++ {
			v := new(big.Rat).Add(x[k], new(big.Rat).Mul(t, new(big.Rat).Sub(y[k], x[k])))
			if v.Cmp(p[k]) != 0 {
				return nil, false
			}
		}
		return t, true
	}
	for _, source := range raw {
		type cut struct {
			t *big.Rat
			p [2]*big.Rat
		}
		var cuts []cut
		for _, p := range vertices {
			work++
			if work > o.MaxSplitWork {
				return fail(parametric.ErrBudget)
			}
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			if t, ok := parameter(p, source.a, source.b); ok {
				cuts = append(cuts, cut{t, p})
			}
		}
		sort.Slice(cuts, func(i, j int) bool { return cuts[i].t.Cmp(cuts[j].t) < 0 })
		for i := 1; i < len(cuts); i++ {
			p, q := cuts[i-1].p, cuts[i].p
			segments++
			if segments > o.MaxSegments {
				return fail(parametric.ErrBudget)
			}
			f, r := key(p, q), key(q, p)
			if _, ok := live[f]; ok {
				return fail(geom.ErrConvergence)
			}
			if _, ok := live[r]; ok {
				delete(live, r)
				canceled++
			} else {
				live[f] = edge{a: p, b: q}
			}
		}
	}
	// Authenticate surviving edge intervals against actual original uses; no
	// partition edge is allowed to become an invented material boundary.
	matrix := splineCoincidenceUVMatrix(overlay.Outer.Map)
	for k, ed := range live {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		for source, f := range []Face{a.faces[faceA], b.faces[faceB]} {
			p, q := ed.a, ed.b
			if source == 1 {
				p = coincidentMapPoint(matrix, p)
				q = coincidentMapPoint(matrix, q)
			}
			for li, loop := range append([][]Use{f.Outer}, f.Holes...) {
				for ui, u := range loop {
					work++
					if work > o.MaxSplitWork {
						return fail(parametric.ErrBudget)
					}
					x, y := [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}, [2]*big.Rat{ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}
					_, ok := parameter(p, x, y)
					_, ok2 := parameter(q, x, y)
					if ok && ok2 {
						ed.owners = append(ed.owners, CoincidentTrimOwner{source, []int{faceA, faceB}[source], li, ui, u.Edge})
					}
				}
			}
		}
		live[k] = ed
	}
	outgoing := map[string]string{}
	incoming := map[string]string{}
	keys := make([]string, 0, len(live))
	for k, edge := range live {
		p, q := pointKey(edge.a), pointKey(edge.b)
		if _, ok := outgoing[p]; ok {
			return fail(geom.ErrDegenerate)
		}
		if _, ok := incoming[q]; ok {
			return fail(geom.ErrDegenerate)
		}
		outgoing[p] = k
		incoming[q] = k
		keys = append(keys, k)
	}
	for p := range outgoing {
		if _, ok := incoming[p]; !ok {
			return fail(geom.ErrConvergence)
		}
	}
	sort.Strings(keys)
	visited := map[string]bool{}
	var loops []CoincidentChartLoop
	toB := splineCoincidenceUVMatrix(overlay.Outer.Map)
	native := func(points [2][2]*big.Rat) (*geom.ExactSplineCurve, error) {
		h := make([][4]*big.Rat, 2)
		for i, p := range points {
			h[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), big.NewRat(1, 1)}
		}
		return geom.NewExactHomogeneousBezier(ctx, h, o.Material.Outer.Coincidence.Patches)
	}
	for _, start := range keys {
		if visited[start] {
			continue
		}
		loop := CoincidentChartLoop{SignedAreaA: new(big.Rat)}
		current := start
		for {
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			if visited[current] {
				if current != start {
					return fail(geom.ErrConvergence)
				}
				break
			}
			visited[current] = true
			source := live[current]
			if len(source.owners) == 0 {
				return fail(geom.ErrConvergence)
			}
			out := CoincidentChartEdge{A: [2][2]*big.Rat{coincidentCopyPoint(source.a), coincidentCopyPoint(source.b)}, B: [2][2]*big.Rat{coincidentMapPoint(toB, source.a), coincidentMapPoint(toB, source.b)}}
			for _, owner := range source.owners {
				face := a.faces[faceA]
				points := out.A
				if owner.Source == 1 {
					face = b.faces[faceB]
					points = out.B
				}
				use := append([][]Use{face.Outer}, face.Holes...)[owner.Loop][owner.Use]
				begin, end := [2]*big.Rat{ratInset(use.UVStart.X), ratInset(use.UVStart.Y)}, [2]*big.Rat{ratInset(use.UVEnd.X), ratInset(use.UVEnd.Y)}
				axis := 0
				if begin[0].Cmp(end[0]) == 0 {
					axis = 1
				}
				delta := new(big.Rat).Sub(end[axis], begin[axis])
				if delta.Sign() == 0 {
					return fail(geom.ErrDegenerate)
				}
				var interval [2]*big.Rat
				for i, p := range points {
					interval[i] = new(big.Rat).Quo(new(big.Rat).Sub(p[axis], begin[axis]), delta)
					if interval[i].Sign() < 0 || interval[i].Cmp(big.NewRat(1, 1)) > 0 {
						return fail(geom.ErrConvergence)
					}
					for k := 0; k < 2; k++ {
						v := new(big.Rat).Add(begin[k], new(big.Rat).Mul(interval[i], new(big.Rat).Sub(end[k], begin[k])))
						if v.Cmp(p[k]) != 0 {
							return fail(geom.ErrConvergence)
						}
					}
				}
				out.Sources = append(out.Sources, CoincidentChartUse{owner, interval})
			}
			out.CurveA, e = native(out.A)
			if e != nil {
				return fail(e)
			}
			out.CurveB, e = native(out.B)
			if e != nil {
				return fail(e)
			}
			loop.Edges = append(loop.Edges, out)
			loop.SignedAreaA.Add(loop.SignedAreaA, new(big.Rat).Sub(new(big.Rat).Mul(source.a[0], source.b[1]), new(big.Rat).Mul(source.a[1], source.b[0])))
			next, ok := outgoing[pointKey(source.b)]
			if !ok {
				return fail(geom.ErrConvergence)
			}
			current = next
		}
		loop.SignedAreaA.Quo(loop.SignedAreaA, big.NewRat(2, 1))
		if loop.SignedAreaA.Sign() == 0 {
			return fail(geom.ErrDegenerate)
		}
		loops = append(loops, loop)
	}
	result := CoincidentChartRegions{Faces: overlay.Outer.Faces, Map: overlay.Outer.Map, Injectivity: overlay.Outer.Injectivity, UVArea: overlay.UVArea, GridCells: len(overlay.Cells), RetainedCells: len(overlay.Cells), Operation: parametric.Intersection, CanceledSeams: canceled}
	total := new(big.Rat)
	for _, loop := range loops {
		total.Add(total, loop.SignedAreaA)
		if loop.SignedAreaA.Sign() > 0 {
			result.Components = append(result.Components, CoincidentChartComponent{Outer: loop})
		}
	}
	if total.Cmp(overlay.UVArea[0]) != 0 {
		return fail(geom.ErrConvergence)
	}
	for _, hole := range loops {
		if hole.SignedAreaA.Sign() > 0 {
			continue
		}
		parent := -1
		for i, component := range result.Components {
			work += len(component.Outer.Edges)
			if work > o.MaxSplitWork {
				return fail(parametric.ErrBudget)
			}
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			points := make([][2]*big.Rat, len(component.Outer.Edges))
			for j, e := range component.Outer.Edges {
				points[j] = e.A[0]
			}
			if coincidentPolygonLocation(points, hole.Edges[0].A[0]) == 1 && (parent < 0 || component.Outer.SignedAreaA.Cmp(result.Components[parent].Outer.SignedAreaA) < 0) {
				parent = i
			}
		}
		if parent < 0 {
			return fail(geom.ErrConvergence)
		}
		result.Components[parent].Holes = append(result.Components[parent].Holes, hole)
	}
	result.Complete = true
	return result, nil
}

// Exact half-open ray parity, valid for nonorthogonal simple loops.
func coincidentPolygonLocation(loop [][2]*big.Rat, p [2]*big.Rat) int {
	inside := false
	for i, a := range loop {
		b := loop[(i+1)%len(loop)]
		cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(p[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(p[0], a[0])))
		if cross.Sign() == 0 {
			within := true
			for k := 0; k < 2; k++ {
				lo, hi := a[k], b[k]
				if lo.Cmp(hi) > 0 {
					lo, hi = hi, lo
				}
				within = within && p[k].Cmp(lo) >= 0 && p[k].Cmp(hi) <= 0
			}
			if within {
				return 2
			}
		}
		if (a[1].Cmp(p[1]) > 0) != (b[1].Cmp(p[1]) > 0) {
			x := new(big.Rat).Add(a[0], new(big.Rat).Quo(new(big.Rat).Mul(new(big.Rat).Sub(p[1], a[1]), new(big.Rat).Sub(b[0], a[0])), new(big.Rat).Sub(b[1], a[1])))
			if p[0].Cmp(x) < 0 {
				inside = !inside
			}
		}
	}
	if inside {
		return 1
	}
	return 0
}
