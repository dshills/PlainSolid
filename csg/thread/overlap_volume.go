package thread

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CertifyOverlapVolumeBox proves an explicit radius/phase/axial box lies in
// both retained parts and returns its exact volume coefficient (times π).
// Its radii must be strictly inside the common stock band. This is a
// lower-volume witness; an unsuccessful proof does not establish separation.
func (p Pair) CertifyOverlapVolumeBox(ctx context.Context, nutShift *big.Rat, radii, phases, axial [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalOverlapVolumeBox, error) {
	if !p.validInteriorRadiusBand(radii) {
		return geom.HelicalOverlapVolumeBox{}, geom.ErrInvalid
	}
	return geom.CertifyHelicalOverlapVolumeBox(ctx, p.shaftStarts, p.nutStarts, nutShift, radii, phases, axial, p.stockBands[0], p.stockBands[1], o)
}

// CertifyOverlapVolumeBox uses the finished relieved stock. A box spanning a
// relief radius is tested against the intersection of the neighboring axial
// stock bands, so success certifies the entire box. A failed request can be
// retried as smaller boxes on either side of the step.
func (p RelievedPair) CertifyOverlapVolumeBox(ctx context.Context, nutShift *big.Rat, radii, phases, axial [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalOverlapVolumeBox, error) {
	q := p.proofPair
	if !q.validInteriorRadiusBand(radii) {
		return geom.HelicalOverlapVolumeBox{}, geom.ErrInvalid
	}
	// Shaft and nut axial regimes each vary monotonically for their own
	// reliefs; taking the two endpoint-band intersection is sufficient.
	bands := p.stockBandsAtRadius(radii[0])
	end := p.stockBandsAtRadius(radii[1])
	for i := range bands {
		if end[i][0].Cmp(bands[i][0]) > 0 {
			bands[i][0] = end[i][0]
		}
		if end[i][1].Cmp(bands[i][1]) < 0 {
			bands[i][1] = end[i][1]
		}
	}
	return geom.CertifyHelicalOverlapVolumeBox(ctx, q.shaftStarts, q.nutStarts, nutShift, radii, phases, axial, bands[0], bands[1], o)
}

func (p Pair) validInteriorRadiusBand(radii [2]*big.Rat) bool {
	return radii[0] != nil && radii[1] != nil && p.radialRange[0] != nil &&
		radii[0].Cmp(radii[1]) < 0 && radii[0].Cmp(p.radialRange[0]) > 0 && radii[1].Cmp(p.radialRange[1]) < 0
}

// IntegrateOverlapCell returns the exact common volume (π times the
// coefficient) of a radial/phase cell where the cutter slices are constant
// across the radii and the axial overlap law is affine in phase. The geometry
// proof rejects interior events and radial variation.
func (p Pair) IntegrateOverlapCell(ctx context.Context, nutShift *big.Rat, radii, phases [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalAffineOverlapIntegral, error) {
	if !p.validInteriorRadiusBand(radii) {
		return geom.HelicalAffineOverlapIntegral{}, geom.ErrInvalid
	}
	return geom.IntegrateHelicalConstantSliceCell(ctx, p.shaftStarts, p.nutStarts, nutShift, radii, phases, p.stockBands[0], p.stockBands[1], o)
}

// IntegrateOverlapCell applies the same exact integral to a finished pair.
// A cell may touch a relief radius at its boundary, which has zero volume;
// a cell crossing a relief radius must be split by the caller.
func (p RelievedPair) IntegrateOverlapCell(ctx context.Context, nutShift *big.Rat, radii, phases [2]*big.Rat, o geom.HelicalStartsLineOptions) (geom.HelicalAffineOverlapIntegral, error) {
	q := p.proofPair
	if !q.validInteriorRadiusBand(radii) {
		return geom.HelicalAffineOverlapIntegral{}, geom.ErrInvalid
	}
	for _, spec := range []trimmed.HelicalReliefSpec{p.proofReliefs.ShaftTip, p.proofReliefs.NutLower, p.proofReliefs.NutUpper} {
		if spec.Depth == 0 {
			continue
		}
		r := exact(spec.Radius)
		if r.Cmp(radii[0]) > 0 && r.Cmp(radii[1]) < 0 {
			return geom.HelicalAffineOverlapIntegral{}, geom.ErrConvergence
		}
	}
	mid := new(big.Rat).Quo(new(big.Rat).Add(radii[0], radii[1]), big.NewRat(2, 1))
	bands := p.stockBandsAtRadius(mid)
	return geom.IntegrateHelicalConstantSliceCell(ctx, q.shaftStarts, q.nutStarts, nutShift, radii, phases, bands[0], bands[1], o)
}
