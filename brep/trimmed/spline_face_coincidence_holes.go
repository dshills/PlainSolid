package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Only original exact vertices propose hole correspondences. Both loop sets
// have already been independently certified disjoint; all proposed pairings
// still undergo complete support-map and wire identities. No centroid/bounds
// similarity is used as evidence. Work is bounded by MaxUses² exact compares
// and at most MaxUses per-use endpoint extractions.
func coincidentHoleOrder(ctx context.Context, a, b [][]Use, matrix [3][4]*big.Rat, o geom.SurfacePatchOptions) ([]int, error) {
	point := func(u Use, mapped bool) ([2]*big.Rat, error) {
		c, rev, e := coincidentUseCurve(ctx, u, o)
		if e != nil {
			return [2]*big.Rat{}, e
		}
		if mapped {
			c, e = c.TransformRational(ctx, matrix, o)
			if e != nil {
				return [2]*big.Rat{}, e
			}
		}
		net, _, e := c.BezierControlNet(ctx, o)
		if e != nil {
			return [2]*big.Rat{}, e
		}
		i := 0
		if rev {
			i = len(net.Control) - 1
		}
		h := net.Control[i]
		return [2]*big.Rat{new(big.Rat).Quo(h[0], h[3]), new(big.Rat).Quo(h[1], h[3])}, nil
	}
	vertices := make([][][2]*big.Rat, len(b))
	for j := 1; j < len(b); j++ {
		for _, u := range b[j] {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			p, e := point(u, false)
			if e != nil {
				return nil, e
			}
			vertices[j] = append(vertices[j], p)
		}
	}
	ids := make([]int, len(a))
	used := make([]bool, len(b))
	used[0] = true
	for i := 1; i < len(a); i++ {
		if len(a[i]) == 0 {
			return nil, geom.ErrInvalid
		}
		p, e := point(a[i][0], true)
		if e != nil {
			return nil, e
		}
		found := -1
		for j := 1; j < len(b); j++ {
			for _, v := range vertices[j] {
				if p[0].Cmp(v[0]) == 0 && p[1].Cmp(v[1]) == 0 {
					if found >= 0 && found != j {
						return nil, geom.ErrConvergence
					}
					found = j
				}
			}
		}
		if found < 0 || used[found] {
			return nil, geom.ErrConvergence
		}
		ids[i] = found
		used[found] = true
	}
	return ids, nil
}
