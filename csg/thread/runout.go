package thread

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// CompareRunoutQuarter classifies both parts' retained axial material on a
// common quarter-phase ray at a radius strictly inside their common band.
// NutShift translates the nut and its cutters together. Positive axial
// overlap provides an interference witness for this ray; an empty report is
// not a complete 3D clearance claim. Endpoints retain cap and cutter ancestry.
func (p Pair) CompareRunoutQuarter(ctx context.Context, radius, phase, nutShift *big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalRetainedPair, error) {
	if ctx == nil || radius == nil || p.radialRange[0] == nil || radius.Cmp(p.radialRange[0]) <= 0 ||
		radius.Cmp(p.radialRange[1]) >= 0 {
		return geom.HelicalRetainedPair{}, geom.ErrInvalid
	}
	return geom.CompareHelicalRetainedQuarter(ctx, p.shaftStarts, p.nutStarts, radius, phase, nutShift, p.stockBands[0], p.stockBands[1], o)
}

// CompareRunoutPhaseWindow tests an entire closed phase interval at one
// radius strictly inside the common stock band. It certifies no positive
// overlap over this interval or returns a rational phase witness. It does
// not prove coverage at other radii or allow an unqualified 3D fit claim.
func (p Pair) CompareRunoutPhaseWindow(ctx context.Context, radius, nutShift *big.Rat, phase [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalPhaseWindowComparison, error) {
	if ctx == nil || radius == nil || p.radialRange[0] == nil || radius.Cmp(p.radialRange[0]) <= 0 ||
		radius.Cmp(p.radialRange[1]) >= 0 {
		return geom.HelicalPhaseWindowComparison{}, geom.ErrInvalid
	}
	return geom.CompareHelicalRetainedPhaseWindow(ctx, p.shaftStarts, p.nutStarts, radius, nutShift, phase, p.stockBands[0], p.stockBands[1], o)
}

// CompareRunoutRadialBand tests a closed band of radii at one exact helical
// phase. It proves no positive overlap or returns a rational collision radius.
// The endpoints must lie strictly inside the common stock radius band.
func (p Pair) CompareRunoutRadialBand(ctx context.Context, phase, nutShift *big.Rat, radiusBand [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalRadialBandComparison, error) {
	if ctx == nil || radiusBand[0] == nil || radiusBand[1] == nil || p.radialRange[0] == nil ||
		radiusBand[0].Cmp(p.radialRange[0]) <= 0 || radiusBand[1].Cmp(p.radialRange[1]) >= 0 {
		return geom.HelicalRadialBandComparison{}, geom.ErrInvalid
	}
	return geom.CompareHelicalRetainedRadialBand(ctx, p.shaftStarts, p.nutStarts, phase, nutShift, radiusBand, p.stockBands[0], p.stockBands[1], o)
}

// CompareRunoutJointWindow exhausts the closed radius/phase box for a shifted
// nut. A complete clear result excludes positive axial overlap within that
// box. It does not assert minimum Euclidean clearance or exclude contacts
// outside the selected radius/phase region.
func (p Pair) CompareRunoutJointWindow(ctx context.Context, nutShift *big.Rat, radii, phases [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalJointWindowComparison, error) {
	if ctx == nil || radii[0] == nil || radii[1] == nil || p.radialRange[0] == nil ||
		radii[0].Cmp(p.radialRange[0]) <= 0 || radii[1].Cmp(p.radialRange[1]) >= 0 {
		return geom.HelicalJointWindowComparison{}, geom.ErrInvalid
	}
	return geom.CompareHelicalRetainedJointWindow(ctx, p.shaftStarts, p.nutStarts, nutShift, radii, phases, p.stockBands[0], p.stockBands[1], o)
}

// CompareRunoutFullTurn exhausts a radial band over the closed full phase turn
// [0,1]. Both seam representations are included with reindexed turn ancestry.
// The radial band must be strictly within the common stock radii.
func (p Pair) CompareRunoutFullTurn(ctx context.Context, nutShift *big.Rat, radii [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalJointWindowComparison, error) {
	if ctx == nil || radii[0] == nil || radii[1] == nil || p.radialRange[0] == nil ||
		radii[0].Cmp(p.radialRange[0]) <= 0 || radii[1].Cmp(p.radialRange[1]) >= 0 {
		return geom.HelicalJointWindowComparison{}, geom.ErrInvalid
	}
	return geom.CompareHelicalRetainedFullTurn(ctx, p.shaftStarts, p.nutStarts, nutShift, radii, p.stockBands[0], p.stockBands[1], o)
}
