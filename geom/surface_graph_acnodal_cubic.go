package geom

import (
	"context"
	"math/big"
)

// AcnodalCubicSection proves the isolated real double point of a section
// y²=Scale*x²*(x-Gap), Gap>0. ChartMaps map its canonical coordinates to
// each original roof. The optional separate arc has no connection to
// the double point, and the height order does not switch at that point.
type AcnodalCubicSection struct {
	ProjectionAxes        [2]int
	ProjectionOrientation int
	FreeAxis              int
	SourceDirection       int
	Domains               [2]ParameterRectangle
	ChartMaps             [2][2][3]*big.Rat
	Scale, Gap            *big.Rat
	Isolated              NodalCubicVertex
	IsolatedTangent       [2][3]*big.Rat // common original model derivatives by canonical axis
	Arc                   *AcnodalCubicArc
	Regions               []NodalCubicRegion // exterior, then between the arc and the far side if present
	ExactOperations       int
}

// AcnodalCubicArc is a regular branch with x=Gap+t²/Scale, y=t*x.
// The two ends lie strictly inside the same original footprint side.
// Increasing-t jets use the original source parameters and model curve.
type AcnodalCubicArc struct {
	Vertices [3]NodalCubicVertex // turning point, negative exit, positive exit
	Edges    [2]NodalCubicEdge   // negative exit to the turn, then turn to positive exit
	UVJet    [3][2][2]*big.Rat   // vertex, owner, chart axis
	ModelJet [3][3]*big.Rat      // vertex, model axis
}

// DiscoverProjectedGraphAcnodalCubic proves an interior isolated double
// point and, when it enters the footprint, the entire disjoint regular
// branch. The models have aligned affine projected footprints. A branch
// cut by a transverse side, a turning point exactly on the far side, or
// an unrepresentable exit is refused. Source jets must be unambiguous.
func DiscoverProjectedGraphAcnodalCubic(ctx context.Context, a, c *SurfacePatch, axes [2]int, freeAxis int,
	o GraphPointContactOptions) (*AcnodalCubicSection, error) {
	proj, err := projectGraphCubicSource(ctx, a, c, axes, freeAxis, o)
	if err != nil {
		return nil, err
	}
	b := proj.budget
	law, err := recoverDoubleRootGraphCubic(proj.powers, b)
	if err != nil {
		return nil, err
	}
	if law.loopDepth.Sign() >= 0 {
		return nil, ErrDomain
	}
	gap := new(big.Rat).Neg(law.loopDepth)
	near, far, tau, err := acnodalCubicLimits(law, gap, b)
	if err != nil {
		return nil, err
	}
	out := &AcnodalCubicSection{ProjectionAxes: axes, FreeAxis: freeAxis, SourceDirection: law.direction,
		Domains: [2]ParameterRectangle{a.Domain(), c.Domain()}, Scale: law.scale, Gap: gap}
	out.ChartMaps, err = graphCubicChartMaps(out.Domains[0], freeAxis, law.direction, law.center, proj.alignment, b)
	if err != nil {
		return nil, err
	}
	sources := [2]*SurfacePatch{a, c}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	atPoint := func(v *NodalCubicVertex, x, y *big.Rat, interior bool) ([2]RationalSurfacePatchJet, error) {
		var jets [2]RationalSurfacePatchJet
		uv, err := applyGraphCubicChartMaps(out.ChartMaps, [2]*big.Rat{x, y}, false, b)
		if err != nil {
			return jets, err
		}
		jets, err = graphCubicVertexJets(ctx, sources, uv, b)
		if err != nil {
			return jets, err
		}
		for k, p := range jets[0].Point {
			if p.Cmp(jets[1].Point[k]) != 0 {
				return jets, ErrConvergence
			}
		}
		v.UV, v.Point = uv, jets[0].Point
		for i, d := range out.Domains {
			mask := graphCubicBoundary(d, uv[i])
			if interior && mask != 0 || !interior && (mask == 0 || mask&(mask-1) != 0) {
				return jets, ErrConvergence
			}
			v.Boundary[i] = mask
		}
		return jets, nil
	}
	jets, err := atPoint(&out.Isolated, zero, zero, true)
	if err != nil {
		return nil, err
	}
	for i, v := range [2][2]*big.Rat{{one, zero}, {zero, one}} {
		uv, err := applyGraphCubicChartMaps(out.ChartMaps, v, true, b)
		if err != nil {
			return nil, err
		}
		out.IsolatedTangent[i], err = bindGraphCubicModelVector(uv, jets, b)
		if err != nil {
			return nil, err
		}
	}
	t0, t1 := out.IsolatedTangent[0], out.IsolatedTangent[1]
	x, y := axes[0], axes[1]
	det := new(big.Rat).Sub(new(big.Rat).Mul(t0[x], t1[y]), new(big.Rat).Mul(t0[y], t1[x]))
	if err := b.check(det); err != nil {
		return nil, err
	}
	if det.Sign() == 0 {
		return nil, ErrDegenerate
	}
	out.ProjectionOrientation = det.Sign()

	witnesses := []*big.Rat{new(big.Rat).Mul(near, big.NewRat(-1, 2))}
	if tau != nil {
		arc := &AcnodalCubicArc{}
		exitHeight := new(big.Rat).Mul(far, tau)
		for i, station := range [3]*big.Rat{zero, new(big.Rat).Neg(tau), tau} {
			xy := [3][2]*big.Rat{{gap, zero}, {far, new(big.Rat).Neg(exitHeight)}, {far, exitHeight}}[i]
			jets, err := atPoint(&arc.Vertices[i], xy[0], xy[1], i == 0)
			if err != nil {
				return nil, err
			}
			dx := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(2, 1), station), law.scale)
			dy := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(station, station)), law.scale)
			dy.Add(dy, gap)
			arc.UVJet[i], err = applyGraphCubicChartMaps(out.ChartMaps, [2]*big.Rat{dx, dy}, true, b)
			if err != nil {
				return nil, err
			}
			arc.ModelJet[i], err = bindGraphCubicModelVector(arc.UVJet[i], jets, b)
			if err != nil {
				return nil, err
			}
		}
		if arc.Vertices[1].Boundary != arc.Vertices[2].Boundary {
			return nil, ErrConvergence
		}
		arc.Edges = [2]NodalCubicEdge{
			{Start: 1, End: 0, Range: [2]*big.Rat{new(big.Rat).Neg(tau), new(big.Rat)}},
			{Start: 0, End: 2, Range: [2]*big.Rat{new(big.Rat), new(big.Rat).Set(tau)}},
		}
		out.Arc = arc
		witnesses = append(witnesses, new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(gap, far)))
	}
	out.Regions = make([]NodalCubicRegion, len(witnesses))
	for i, x := range witnesses {
		uv, err := applyGraphCubicChartMaps(out.ChartMaps, [2]*big.Rat{x, zero}, false, b)
		if err != nil {
			return nil, err
		}
		jets, err := graphCubicOriginalJets(ctx, sources, uv, b)
		if err != nil {
			return nil, err
		}
		s := jets[0].Point[proj.residualAxis].Cmp(jets[1].Point[proj.residualAxis])
		if s != (1-2*i)*law.side {
			return nil, ErrConvergence
		}
		out.Regions[i] = NodalCubicRegion{UV: uv, HeightSide: s}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}

func acnodalCubicLimits(law nodalCubicLaw, gap *big.Rat, b *polynomialBudget) (near, far, tau *big.Rat, err error) {
	near = new(big.Rat).Set(law.center[0])
	far = new(big.Rat).Sub(big.NewRat(1, 1), near)
	if law.direction < 0 {
		near, far = far, near
	}
	margin := new(big.Rat).Sub(big.NewRat(1, 1), law.center[1])
	if margin.Cmp(law.center[1]) > 0 {
		margin.Set(law.center[1])
	}
	if near.Sign() <= 0 || far.Sign() <= 0 || margin.Sign() <= 0 || far.Cmp(gap) == 0 {
		return nil, nil, nil, ErrDomain
	}
	if far.Cmp(gap) < 0 {
		// For x<Gap, a nonnegative y² equals a negative right
		// side except at x=0, y=0. The branch misses the footprint.
		return near, far, nil, nil
	}
	// On x>=Gap, x=Gap+t²/k and |y|=|t|x increase strictly
	// with |t|, so the far endpoint bounds the entire regular arc.
	exit2 := new(big.Rat).Mul(law.scale, new(big.Rat).Sub(far, gap))
	height2 := new(big.Rat).Mul(exit2, new(big.Rat).Mul(far, far))
	margin.Mul(margin, margin)
	for _, v := range []*big.Rat{exit2, height2, margin} {
		if err := b.check(v); err != nil {
			return nil, nil, nil, err
		}
	}
	if height2.Cmp(margin) >= 0 {
		return nil, nil, nil, ErrDomain
	}
	lower, upper, err := rationalSqrtBounds(exit2, 8, b)
	if err != nil {
		return nil, nil, nil, err
	}
	if lower.Cmp(upper) != 0 {
		return nil, nil, nil, ErrUnrepresentable
	}
	return near, far, lower, nil
}
