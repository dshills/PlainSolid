package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

type splineKnotTangency struct {
	uv  [2]*big.Rat
	xyz [3]*big.Rat
}

// A complete common-projection knot partition proves every contact. Accept
// only finite interior corners with identical strict order on all cells;
// any contact line, patch, boundary corner or unresolved cell disqualifies
// this route. No partition edge is promoted to an input topological edge.
func isolatedSplineKnotTangencies(ctx context.Context, a, b *geom.SurfacePatch, o geom.GraphPointContactOptions) ([]splineKnotTangency, int, int, error) {
	q, e := geom.PartitionSplineGraphContact(ctx, a, b, o)
	work := q.ExactOperations
	if e != nil {
		return nil, 0, work, e
	}
	if !q.Complete || len(q.Cells) == 0 || len(q.ZeroSides) != 0 || len(q.ZeroCorners) == 0 || len(q.ZeroCorners) > 128 {
		return nil, 0, work, geom.ErrConvergence
	}
	sign := q.Cells[0].Side
	for _, c := range q.Cells {
		if c.Coincident || sign == 0 || c.Side != sign {
			return nil, 0, work, geom.ErrConvergence
		}
	}
	d := a.Domain()
	limits := [2][2]*big.Rat{{ratInset(d.UMin), ratInset(d.UMax)}, {ratInset(d.VMin), ratInset(d.VMax)}}
	for _, corner := range q.ZeroCorners {
		for i, k := range corner.Parameters {
			if k.Cmp(limits[i][0]) <= 0 || k.Cmp(limits[i][1]) >= 0 {
				return nil, 0, work, geom.ErrConvergence
			}
		}
	}
	out := make([]splineKnotTangency, 0, len(q.ZeroCorners))
	for _, corner := range q.ZeroCorners {
		if e := ctx.Err(); e != nil {
			return nil, 0, work, e
		}
		if left := o.MaxOperations - work; left > 0 {
			uv := corner.Parameters
			p, e := a.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{
				MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			work += p.ExactOperations
			if e != nil {
				return nil, 0, work, e
			}
			out = append(out, splineKnotTangency{uv: uv, xyz: p.Point})
		} else {
			return nil, 0, work, geom.ErrConvergence
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if c := out[i].uv[0].Cmp(out[j].uv[0]); c != 0 {
			return c < 0
		}
		return out[i].uv[1].Cmp(out[j].uv[1]) < 0
	})
	return out, sign, work, nil
}
