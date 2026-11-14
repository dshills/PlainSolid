package geom

import (
	"context"
	"math/big"
)

// RationalIsoTubeDiscoveryOptions sets independent limits on exhaustive iso
// classification, local box search, and the final original-support proof.
// MaxOperations in Proof caps each of the preliminary classification and
// final source proof; Search controls a separate box search budget.
type RationalIsoTubeDiscoveryOptions struct {
	Search SurfaceArcTubeDiscoveryOptions
	Proof  RationalIsoTubeChainOptions
}

// DiscoverRationalIsoTubeChain finds the original fixed source parameter,
// clips the complete source iso to an embedded planar trapezoid, and searches
// for a finite sequence of exact restricted tube pieces. The admitted pair
// has one transverse iso and one nonzero rational retained interval.
// The returned cuts and boxes can reproduce the same proof. Unresolved pairs
// never return a partial recipe or chain.
func DiscoverRationalIsoTubeChain(ctx context.Context, a, b *SurfacePatch, o RationalIsoTubeDiscoveryOptions) (int, float64, []*big.Rat, []RationalIsoTubePiece, *RationalIsoTubeChain, error) {
	axis, fixed, cuts, pieces, chain, err := discoverRationalIsoTubeChain(ctx, a, b, o)
	if err != nil {
		return 0, 0, nil, nil, nil, err
	}
	return axis, fixed, cuts, pieces, chain, nil
}

func discoverRationalIsoTubeChain(ctx context.Context, a, b *SurfacePatch, o RationalIsoTubeDiscoveryOptions) (axis int, fixed float64, cuts []*big.Rat, pieces []RationalIsoTubePiece, chain *RationalIsoTubeChain, err error) {
	if ctx == nil || a == nil || b == nil || o.Proof.MaxPieces < 0 || o.Proof.MaxPieces > 64 || o.Proof.MaxCells < 0 || o.Proof.MaxCells > 4096 {
		err = ErrInvalid
		return
	}
	if err = ctx.Err(); err != nil {
		return
	}
	opts, err := patchOptions(o.Proof.SurfacePatchOptions)
	if err != nil {
		return
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	remaining := func() (SurfacePatchOptions, error) {
		child := opts
		child.MaxOperations -= budget.operations
		if child.MaxOperations < 1 {
			return child, ErrConvergence
		}
		return child, ctx.Err()
	}
	chartOptions := AffineSurfaceChartOptions{MaxCells: o.Proof.MaxCells, MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}
	chart, err := NewTrapezoidSurfaceChart(ctx, b, chartOptions)
	if err != nil {
		return
	}
	budget.operations += chart.ExactOperations()
	child, err := remaining()
	if err != nil {
		return
	}
	plane, err := chart.RawSupportPlane(child)
	if err != nil {
		return
	}
	budget.operations += plane.ExactOperations()
	coeff := plane.Coefficients()
	normal := [3]*big.Rat{coeff[0], coeff[1], coeff[2]}
	d := a.Domain()
	lo := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
	hi := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.VMax)}
	var val [3]*big.Rat
	for i, uv := range [3][2]*big.Rat{lo, {hi[0], lo[1]}, {lo[0], hi[1]}} {
		child, e := remaining()
		if e != nil {
			err = e
			return
		}
		p, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
		if e != nil {
			err = e
			return
		}
		budget.operations += p.ExactOperations
		val[i] = new(big.Rat)
		for j := range normal {
			val[i].Add(val[i], new(big.Rat).Mul(normal[j], p.Point[j]))
			if err = budget.check(val[i]); err != nil {
				return
			}
		}
	}
	var slope [2]*big.Rat
	offset := new(big.Rat).Set(val[0])
	for i := range slope {
		slope[i] = new(big.Rat).Quo(new(big.Rat).Sub(val[i+1], val[0]), new(big.Rat).Sub(hi[i], lo[i]))
		offset.Sub(offset, new(big.Rat).Mul(slope[i], lo[i]))
		if err = budget.check(slope[i]); err != nil {
			return
		}
		if err = budget.check(offset); err != nil {
			return
		}
	}
	if (slope[0].Sign() == 0) == (slope[1].Sign() == 0) {
		err = ErrUnrepresentable
		return
	}
	if slope[0].Sign() == 0 {
		axis = 1
	}
	fixedRat := new(big.Rat).Quo(new(big.Rat).Neg(new(big.Rat).Add(offset, coeff[3])), slope[axis])
	if err = budget.check(fixedRat); err != nil {
		return
	}
	if fixedRat.Cmp(lo[axis]) <= 0 || fixedRat.Cmp(hi[axis]) >= 0 {
		err = ErrUnrepresentable
		return
	}
	fixed, err = RepresentedExactScalar(fixedRat)
	if err != nil {
		return
	}
	child, err = remaining()
	if err != nil {
		return
	}
	form, err := CertifySurfaceAffineForm(ctx, a, normal, slope, offset, SurfaceAffineCoordinateOptions{MaxCells: o.Proof.MaxCells, SurfacePatchOptions: child})
	if err != nil {
		return
	}
	if !form.Proven {
		err = ErrUnrepresentable
		return
	}
	budget.operations += form.ExactOperations
	child, err = remaining()
	if err != nil {
		return
	}
	curve, err := a.IsoCurve(axis, fixed, child)
	if err != nil {
		return
	}
	budget.operations += curve.ExactOperations()
	child, err = remaining()
	if err != nil {
		return
	}
	clip, err := ClipIsoCurveToTrapezoidChart(ctx, curve, chart, SplinePlaneOptions{MaxOperations: child.MaxOperations, MaxCoefficientBits: child.MaxCoefficientBits})
	if err != nil {
		return
	}
	if !clip.Complete || len(clip.Spans) != 1 || len(clip.Contacts) != 0 || len(clip.CoincidentBoundary) != 0 ||
		clip.Spans[0].Start.TangentBoundary || clip.Spans[0].End.TangentBoundary {
		err = ErrUnrepresentable
		return
	}
	from, until := clip.Spans[0].Start.Parameter, clip.Spans[0].End.Parameter
	search, err := discoverSurfaceIsoPlaneRationalTubes(ctx, a, b, axis, fixed, from, until, o.Search, true, true, true)
	if err != nil {
		return
	}
	maxPieces := o.Proof.MaxPieces
	if maxPieces == 0 {
		maxPieces = 16
	}
	if !search.Complete || len(search.Excluded) != 0 || len(search.Tubes) == 0 || len(search.Tubes) > maxPieces {
		err = ErrConvergence
		return
	}
	cuts = []*big.Rat{new(big.Rat).Set(from)}
	for _, t := range search.Tubes {
		if t.Boundary != nil || t.ExactSourceSpan[0].Cmp(cuts[len(cuts)-1]) != 0 {
			err = ErrConvergence
			return
		}
		pieces = append(pieces, RationalIsoTubePiece{Box: t.Box, FreeAxis: t.FreeAxis})
		cuts = append(cuts, new(big.Rat).Set(t.ExactSourceSpan[1]))
	}
	if cuts[len(cuts)-1].Cmp(until) != 0 {
		err = ErrConvergence
		return
	}
	chain, err = CertifyRationalIsoTubeChain(ctx, a, b, axis, fixed, cuts, pieces, o.Proof)
	return
}
