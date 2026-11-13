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
