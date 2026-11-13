package feature

import (
	"context"
	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// HoledResult extends feature provenance with a tool wall face for every inner
// ring segment. HoleWalls preserves caller ring/segment order, irrespective of
// winding. Result.SketchWalls maps the outer ring as in Extrude.
type HoledResult struct {
	Result
	HoleWalls [][]int
}

// ExtrudeWithHoles makes a protrusion or blind pocket from a simple outer ring
// with strictly contained disjoint holes. Holes preserve existing base material
// during subtraction and remain empty in a protrusion. Convex-base, exact
// representability and strict support-containment requirements match Extrude.
func ExtrudeWithHoles(ctx context.Context, base brep.Solid, face int, outer []geom.Vec3, holes [][]geom.Vec3, offset geom.Vec3, kind Kind, o Options) (HoledResult, error) {
	return extrudeHoles(ctx, base, face, -1, outer, holes, offset, kind, o)
}

// ThroughPocketWithHoles sweeps the holed profile exactly between named opposite
// supports. A through annular cut may disconnect the central material island;
// resulting valid disconnected solids are retained with full Boolean ancestry.
func ThroughPocketWithHoles(ctx context.Context, base brep.Solid, entry, exit int, outer []geom.Vec3, holes [][]geom.Vec3, offset geom.Vec3, o Options) (HoledResult, error) {
	if exit < 0 || exit == entry || exit >= len(base.Faces()) {
		return HoledResult{}, ErrInvalid
	}
	return extrudeHoles(ctx, base, entry, exit, outer, holes, offset, Pocket, o)
}
func extrudeHoles(ctx context.Context, base brep.Solid, face, exit int, outer []geom.Vec3, holes [][]geom.Vec3, offset geom.Vec3, kind Kind, o Options) (HoledResult, error) {
	empty := HoledResult{}
	if ctx == nil || o.MaxSketchVertices < 0 {
		return empty, ErrInvalid
	}
	limit := o.MaxSketchVertices
	if limit == 0 {
		limit = 256
	}
	count := len(outer)
	for _, h := range holes {
		if len(h) < 3 {
			return empty, ErrInvalid
		}
		if len(h) > limit-count {
			return empty, ErrLimit
		}
		count += len(h)
	}
	if count > limit {
		return empty, ErrLimit
	}
	// Reuse the exact support/endpoint checks on each ring. The closed material
	// tool is assembled only after all rings pass, so no partial feature escapes.
	rings := append([][]geom.Vec3{outer}, holes...)
	for _, ring := range rings {
		if _, e := extrude(ctx, base, face, exit, ring, offset, kind, o, true); e != nil {
			return empty, e
		}
	}
	tool, e := brep.PrismWithHoles(outer, holes, offset)
	if e != nil {
		return empty, ErrInvalid
	}
	vs, es, fs := tool.Vertices(), tool.Edges(), tool.Faces()
	maps := make([][]int, len(rings))
	work, maxWork := 0, o.MaxWork
	if maxWork == 0 {
		maxWork = 100000
	}
	for i, ring := range rings {
		maps[i] = make([]int, len(ring))
		for j, a := range ring {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			b := ring[(j+1)%len(ring)]
			maps[i][j] = -1
			for fi, f := range fs {
				if fi < 2 {
					continue
				}
				for _, u := range f.Boundary {
					work++
					if work > maxWork {
						return empty, ErrLimit
					}
					edge := es[u.Edge]
					p, q := vs[edge.Start], vs[edge.End]
					if (p == a && q == b) || (p == b && q == a) {
						maps[i][j] = fi
					}
				}
			}
			if maps[i][j] < 0 {
				return empty, ErrInvalid
			}
		}
	}
	op := poly.Union
	if kind == Pocket {
		op = poly.Difference
	}
	solid, history, e := poly.CombineWithTopologyHistory(ctx, base, tool, op, o.Boolean)
	if e != nil {
		return empty, e
	}
	return HoledResult{Result{solid, tool, history, 0, 1, maps[0]}, maps[1:]}, nil
}
