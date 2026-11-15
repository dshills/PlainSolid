package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"sort"
)

type CoincidentChartRegionOptions struct {
	Overlay          CoincidentRectangleOverlapOptions
	MaxBoundaryEdges int
}
type CoincidentChartUse struct {
	Owner CoincidentTrimOwner
	Range [2]*big.Rat
}

// CoincidentChartEdge is oriented with retained material to its left in A's
// chart. Native exact curves have domain[0,1] and carry UV in X/Y, Z=0.
// Source ranges refer to original coedge traversal, not model-edge storage.
type CoincidentChartEdge struct {
	A, B           [2][2]*big.Rat
	CurveA, CurveB *geom.ExactSplineCurve
	Sources        []CoincidentChartUse
}
type CoincidentChartLoop struct {
	Edges       []CoincidentChartEdge
	SignedAreaA *big.Rat
}
type CoincidentChartComponent struct {
	Outer CoincidentChartLoop
	Holes []CoincidentChartLoop
}
type CoincidentChartRegions struct {
	Faces                    [2]int
	Map                      intersect.SplineChartMap
	Injectivity              geom.SurfaceInjectivity
	UVArea                   [2]*big.Rat
	GridCells, RetainedCells int
	Operation                parametric.BooleanOp
	Components               []CoincidentChartComponent
	CanceledSeams            int
	Complete                 bool
}

// BuildCoincidentChartRegions regularizes a certified shared-chart grid. It
// cancels every internal edge only against its exact opposite incidence and
// rejects vertices of degree other than two (including point-pinched rings).
// Isolated edge/point contacts are not material regions. This returns exact
// chart topology, not a sewn native solid or general solid Boolean result.
func (a Solid) BuildCoincidentChartRegions(ctx context.Context, faceA int, b Solid, faceB int, operation parametric.BooleanOp, o CoincidentChartRegionOptions) (CoincidentChartRegions, error) {
	fail := func(e error) (CoincidentChartRegions, error) { return CoincidentChartRegions{}, e }
	if operation > parametric.Difference {
		return fail(geom.ErrInvalid)
	}
	if o.MaxBoundaryEdges == 0 {
		o.MaxBoundaryEdges = 4096
	}
	if o.MaxBoundaryEdges < 1 || o.MaxBoundaryEdges > 65536 {
		return fail(geom.ErrInvalid)
	}
	overlay, e := a.intersectCoincidentOrthogonalTrims(ctx, faceA, b, faceB, o.Overlay, false, operation)
	if e != nil {
		return fail(e)
	}
	// The operation grid's positive-area cells are the only material evidence.
	// Remove its auxiliary closed-set contacts: they are not part of this API.
	overlay.Segments = nil
	overlay.Points = nil
	type edge struct {
		a, b   [2]*big.Rat
		owners []CoincidentTrimOwner
	}
	pointKey := func(p [2]*big.Rat) string { return p[0].RatString() + "," + p[1].RatString() }
	key := func(p, q [2]*big.Rat) string { return pointKey(p) + "|" + pointKey(q) }
	live := map[string]edge{}
	canceled := 0
	for _, tile := range overlay.Tiles {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		corners := [4][2]*big.Rat{tile.A.Min, {tile.A.Max[0], tile.A.Min[1]}, tile.A.Max, {tile.A.Min[0], tile.A.Max[1]}}
		for i, p := range corners {
			q := corners[(i+1)%4]
			forward, reverse := key(p, q), key(q, p)
			if _, ok := live[forward]; ok {
				return fail(geom.ErrConvergence)
			}
			if _, ok := live[reverse]; ok {
				delete(live, reverse)
				canceled++
			} else {
				live[forward] = edge{p, q, tile.BoundaryOwners[[4]int{2, 1, 3, 0}[i]]}
			}
		}
	}
	if len(live) > o.MaxBoundaryEdges {
		return fail(parametric.ErrBudget)
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
	toB := splineCoincidenceUVMatrix(overlay.Map)
	native := func(points [2][2]*big.Rat) (*geom.ExactSplineCurve, error) {
		h := make([][4]*big.Rat, 2)
		for i, p := range points {
			h[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), big.NewRat(1, 1)}
		}
		return geom.NewExactHomogeneousBezier(ctx, h, o.Overlay.Coincidence.Patches)
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
	result := CoincidentChartRegions{Faces: overlay.Faces, Map: overlay.Map, Injectivity: overlay.Injectivity, UVArea: overlay.UVArea, GridCells: overlay.Cells, RetainedCells: len(overlay.Tiles), Operation: operation, CanceledSeams: canceled}
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
			points := make([][2]*big.Rat, len(component.Outer.Edges))
			for j, e := range component.Outer.Edges {
				points[j] = e.A[0]
			}
			if coincidentOrthogonalLocation(points, hole.Edges[0].A[0]) == 1 && (parent < 0 || component.Outer.SignedAreaA.Cmp(result.Components[parent].Outer.SignedAreaA) < 0) {
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
