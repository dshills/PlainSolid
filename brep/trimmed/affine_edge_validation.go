package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Each cache entry proves the entire source surface affine, so its exact
// restriction to any straight UV segment is determined by the endpoints.
// An unsupported evaluator or failed chart proof never establishes identity.
type affineEdgeCharts map[*geom.BSplineSurface]*geom.AffineSurfaceChart

func (cache affineEdgeCharts) endpoints(ctx context.Context, x analyticUVLine, o edgecheck.Options) (r [2][3]*big.Rat, ok bool, err error) {
	s, placement := parametric.SurfaceMapping(x.surface)
	b, supported := s.(*geom.BSplineSurface)
	if !supported || b == nil {
		return r, false, nil
	}
	c, checked := cache[b]
	if !checked {
		p, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
		if e == nil {
			c, e = geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
		}
		if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrDegenerate) && !errors.Is(e, geom.ErrInvalid) {
			return r, false, e
		}
		cache[b] = c
	}
	if c == nil {
		return r, false, nil
	}
	d := c.Domain()
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	for i, uv := range x.uv {
		for j, q := range uv {
			if q.Cmp(ratInset(lo[j])) < 0 || q.Cmp(ratInset(hi[j])) > 0 {
				return r, false, nil
			}
		}
		point, e := c.EvalExact(uv[0], uv[1])
		if e != nil {
			return r, false, e
		}
		for j := range point {
			r[i][j] = ratInset(placement[j][3])
			for k, q := range point {
				r[i][j].Add(r[i][j], new(big.Rat).Mul(ratInset(placement[j][k]), q))
			}
		}
	}
	return r, true, nil
}

func (cache affineEdgeCharts) matchUse(ctx context.Context, e parametric.Edge, s geom.Surface, u Use, o edgecheck.Options) (bool, error) {
	a, ok := analyticModelLine(e)
	b, supported := analyticUseLine(s, u)
	if !ok || !supported {
		return false, nil
	}
	return cache.match(ctx, a, b, o)
}

func (cache affineEdgeCharts) matchModel(ctx context.Context, a, b parametric.Edge, reversed bool, o edgecheck.Options) (bool, error) {
	x, ok := analyticModelLine(a)
	y, supported := analyticModelLine(b)
	if !ok || !supported {
		return false, nil
	}
	if reversed {
		y.uv[0], y.uv[1] = y.uv[1], y.uv[0]
	}
	return cache.match(ctx, x, y, o)
}

func (cache affineEdgeCharts) match(ctx context.Context, a, b analyticUVLine, o edgecheck.Options) (bool, error) {
	p, ok, e := cache.endpoints(ctx, a, o)
	if e != nil || !ok {
		return false, e
	}
	q, ok, e := cache.endpoints(ctx, b, o)
	if e != nil || !ok {
		return false, e
	}
	return exactPointCoordinatesEqual(p[0], q[0]) && exactPointCoordinatesEqual(p[1], q[1]), nil
}
