package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// This is used only after the ordinary display triangulator rejects. Every
// represented coordinate is retained exactly. The existing rational region
// proof authenticates simplicity, hole containment, all directed edge atoms,
// triangle incidence and equal rational area; no tolerance removes a corner.
func triangulateExactUVFallback(ctx context.Context, rings [][]geom.Vec3, maximum int) ([][3]geom.Vec2, error) {
	if ctx == nil || len(rings) == 0 || len(rings) > 32 || maximum < 3 || maximum > 2048 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	count := 0
	for _, ring := range rings {
		if len(ring) < 3 || len(ring) > maximum-count {
			return nil, parametric.ErrBudget
		}
		count += len(ring)
	}
	p := partitionFillProof{ctx: ctx, limit: 64000000}
	loops := make([][]int, len(rings))
	var points []geom.Vec2
	for i, ring := range rings {
		for _, v := range ring {
			if !v.IsFinite() || v.Z != 0 {
				return nil, geom.ErrInvalid
			}
			loops[i] = append(loops[i], len(points))
			points = append(points, geom.Vec2{X: v.X, Y: v.Y})
			p.points = append(p.points, [2]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y)})
		}
	}
	tris, _, e := mixedTriangulateDirectedRegion(&p, loops, 2*maximum)
	if e != nil {
		return nil, e
	}
	out := make([][3]geom.Vec2, len(tris))
	for i, tri := range tris {
		for j, id := range tri {
			out[i][j] = points[id]
		}
	}
	return out, nil
}
