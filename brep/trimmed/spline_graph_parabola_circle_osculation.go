package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func proveParabolaCircleOsculationSection(ctx context.Context, sources [3]*geom.BSplineSurface, axis int, tolerance float64,
	o ParabolaCircleGraphOptions) (parabolaCircleTangentSection, *geom.SplineGraphParabolaCircleOsculation, error) {
	var sec parabolaCircleTangentSection
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return sec, nil, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return sec, nil, err
	}
	p, err := geom.DiscoverSplineGraphParabolaCircleOsculation(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return sec, nil, err
	}
	open, err := splitParabolaCircleTangentOpenRegions(p.ParabolaControls, p.Parameters, axis)
	if err != nil {
		return sec, nil, err
	}
	node, err := exactGraphPartitionUV(p.Parameters)
	if err != nil {
		return sec, nil, err
	}
	circle, _, err := exactGraphCircleRings(p.CircleCenter, p.RadiusSquared)
	if err != nil {
		return sec, nil, err
	}
	contacts := 0
	for _, arc := range circle {
		if arc.a == node {
			contacts++
		}
	}
	if contacts != 1 {
		return sec, nil, geom.ErrConvergence
	}
	sec = parabolaCircleTangentSection{
		axis: axis, diskRegion: p.DiskRegion, parameters: p.Parameters, circleCenter: p.CircleCenter,
		point: p.Point, slope: new(big.Rat), second: [2]*big.Rat{p.SharedSecondDerivative, p.SharedSecondDerivative},
		regions: [3]openGraphRegion{open[0], open[1], {loop: circle}},
	}
	for i, sign := range p.RegionSides {
		sec.regions[i].sign = sign
	}
	return sec, p, nil
}

// bindParabolaCircleOsculationRay re-proves the matched lower jet and first
// unequal fourth derivative in both original directed chart uses. The
// preceding native source ledger has already bound their model lift and
// both exact owner faces.
func bindParabolaCircleOsculationRay(s Solid, id, direction, branch int, p *geom.SplineGraphParabolaCircleOsculation) error {
	node, err := exactGraphPartitionUV(p.Parameters)
	if err != nil {
		return err
	}
	want := new(big.Rat)
	if branch == 1 {
		want = p.CircleFourthDerivative
	}
	uses := 0
	for _, face := range s.faces {
		for _, use := range face.Outer {
			if use.Edge != id {
				continue
			}
			uv, ok := use.Curve.(ExactQuadraticUV)
			if !ok || (use.UVStart == node) == (use.UVEnd == node) {
				return geom.ErrConvergence
			}
			if use.UVEnd == node {
				uv = reversePCurve(uv).(ExactQuadraticUV)
			}
			if err := verifyOsculatingUVJet(uv, p, direction, want); err != nil {
				return err
			}
			uses++
		}
	}
	if uses != 2 {
		return geom.ErrConvergence
	}
	return nil
}

func verifyOsculatingUVJet(uv ExactQuadraticUV, p *geom.SplineGraphParabolaCircleOsculation, direction int, want *big.Rat) error {
	axis := p.FreeAxis
	var x, y, w [3]*big.Rat
	for i, h := range uv.control {
		w[i] = h[2]
		x[i] = new(big.Rat).Sub(h[axis], new(big.Rat).Mul(p.Parameters[axis], w[i]))
		y[i] = new(big.Rat).Sub(h[1-axis], new(big.Rat).Mul(p.Parameters[1-axis], w[i]))
	}
	if x[0].Sign() != 0 || y[0].Sign() != 0 {
		return geom.ErrConvergence
	}
	power := func(row [3]*big.Rat) [3]*big.Rat {
		linear := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(row[1], row[0]))
		quadratic := new(big.Rat).Sub(new(big.Rat).Add(row[2], row[0]), new(big.Rat).Mul(big.NewRat(2, 1), row[1]))
		return [3]*big.Rat{row[0], linear, quadratic}
	}
	x, y, w = power(x), power(y), power(w)
	dx := new(big.Rat).Quo(x[1], w[0])
	if dx.Sign() != 2*direction-1 {
		return geom.ErrConvergence
	}
	// The degree-four homogeneous residual represents
	// (v−s)−k*(u−x0)^2 = (W*Y−k*X^2)/W^2 in the original trim clock,
	// where X and Y are centered homogeneous coordinates.
	k := new(big.Rat).Quo(p.SharedSecondDerivative, big.NewRat(2, 1))
	residual := [5]big.Rat{}
	for i := range x {
		for j := range x {
			circle := new(big.Rat).Mul(w[i], y[j])
			parabola := new(big.Rat).Mul(k, new(big.Rat).Mul(x[i], x[j]))
			residual[i+j].Add(&residual[i+j], circle.Sub(circle, parabola))
		}
	}
	for _, q := range residual[:4] {
		if q.Sign() != 0 {
			return geom.ErrConvergence
		}
	}
	dx2 := new(big.Rat).Mul(dx, dx)
	factor := new(big.Rat).Mul(dx2, dx2)
	factor.Mul(factor, new(big.Rat).Mul(w[0], w[0]))
	fourth := new(big.Rat).Mul(&residual[4], big.NewRat(24, 1))
	if fourth.Quo(fourth, factor).Cmp(want) != 0 {
		return geom.ErrConvergence
	}
	return nil
}

type parabolaCircleOsculationRecipeJets struct {
	SharedSecond      string `json:"sharedSecond"`
	CircleFourth      string `json:"circleFourth"`
	SubstitutedFourth string `json:"substitutedFourth"`
}

func parabolaCircleOsculationJets(p *geom.SplineGraphParabolaCircleOsculation) parabolaCircleOsculationRecipeJets {
	return parabolaCircleOsculationRecipeJets{
		SharedSecond: p.SharedSecondDerivative.RatString(), CircleFourth: p.CircleFourthDerivative.RatString(),
		SubstitutedFourth: p.SubstitutedFourthDerivative.RatString(),
	}
}
