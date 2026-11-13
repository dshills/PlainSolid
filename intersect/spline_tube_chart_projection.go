package intersect

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type tubeChartProjection struct {
	from  int // 0 maps chart A to affine B; 1 maps B to affine A
	mapUV geom.SurfaceAffineProjection
}

func tubeProjectionLookup(ctx context.Context, a, b *geom.SurfacePatch, o geom.SurfaceTubeJunctionOptions) func() *tubeChartProjection {
	var projection *tubeChartProjection
	checked := false
	return func() *tubeChartProjection {
		if !checked {
			projection = exactTubeChartProjection(ctx, a, b, o)
			checked = true
		}
		return projection
	}
}

func exactTubeChartProjection(ctx context.Context, a, b *geom.SurfacePatch, o geom.SurfaceTubeJunctionOptions) *tubeChartProjection {
	for i, pair := range [2][2]*geom.SurfacePatch{{a, b}, {b, a}} {
		chart, e := geom.NewAffineSurfaceChart(ctx, pair[1], geom.AffineSurfaceChartOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			continue
		}
		m, e := geom.ClipSurfaceAffineProjection(ctx, pair[0], chart, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e == nil && !m.Empty {
			return &tubeChartProjection{i, m}
		}
	}
	return nil
}

// At a common model point, coordinates linked by the proved chart projection
// obey an exact affine relation. If the two tubes fix linked coordinates to
// consistent endpoint values, an enclosure of the first root strictly inside
// the second's other axes proves the same root belongs to both tubes. This
// does not require the other coordinates to be rational.
func (m tubeChartProjection) junction(da, db, ea, eb geom.ParameterRectangle,
	o geom.SurfaceTubeEndpointOptions, x geom.SurfaceTubeEvaluation, r geom.SurfaceTubeEndpointJunction) geom.SurfaceTubeEndpointJunction {
	if !x.Enclosed || !r.Left.ProvenBranch || !r.Right.ProvenBranch || o.LeftFreeAxis/2 == o.RightFreeAxis/2 {
		return r
	}
	from, to := o.LeftFreeAxis, o.RightFreeAxis
	if from/2 != m.from {
		from, to = to, from
	}
	if m.mapUV.Axis[to%2] != from%2 {
		return r
	}
	boxes := [2][4][2]float64{
		{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {db.UMin, db.UMax}, {db.VMin, db.VMax}},
		{{ea.UMin, ea.UMax}, {ea.VMin, ea.VMax}, {eb.UMin, eb.UMax}, {eb.VMin, eb.VMax}},
	}
	value := func(axis int) *big.Rat {
		if axis == o.LeftFreeAxis {
			return new(big.Rat).SetFloat64(boxes[0][axis][o.LeftEnd])
		}
		return new(big.Rat).SetFloat64(boxes[1][axis][o.RightEnd])
	}
	want := new(big.Rat).Mul(m.mapUV.Scale[to%2], value(from))
	want.Add(want, m.mapUV.Offset[to%2])
	if want.Cmp(value(to)) != 0 {
		r.Separated = true
		return r
	}
	lo := [4]float64{x.ParametersA.Min.X, x.ParametersA.Min.Y, x.ParametersB.Min.X, x.ParametersB.Min.Y}
	hi := [4]float64{x.ParametersA.Max.X, x.ParametersA.Max.Y, x.ParametersB.Max.X, x.ParametersB.Max.Y}
	for i := range lo {
		if i != o.RightFreeAxis && (lo[i] <= boxes[1][i][0] || hi[i] >= boxes[1][i][1]) {
			return r
		}
	}
	r.Connected = true
	return r
}
