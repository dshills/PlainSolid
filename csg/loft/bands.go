package loft

import (
	"context"
	"math/big"
	"plainsolid/brep/support"
	"plainsolid/geom"
)

// BandResult is a piecewise convex station loft. Supports in StationResult
// certify exact planarity only; each BandSupports entry proves its whole slab
// convex. BandVolumes are exact, owned and sum to ExactVolume.
type BandResult struct {
	StationResult
	BandSupports []support.Report
	BandVolumes  []*big.Rat
}

// PlanarStationBands permits reentrant station profiles while retaining exact
// planar side quads. The ordered parallel planes separate slab interiors; each
// neighboring slab is independently certified by Planar and they share precisely
// the same original station polygon. Final exact shell embedding is checked.
// Station limits apply globally. Each band gets a separate Options budget and
// the assembled shell a final separate support/embedding pass, so total work is
// bounded by at most MaxStations times the configured per-pass limits.
// PlanarStations remains the stricter globally convex constructor.
func PlanarStationBands(ctx context.Context, rings [][]geom.Vec3, o StationOptions) (BandResult, error) {
	empty := BandResult{}
	assembled, e := planarStations(ctx, rings, o, false)
	if e != nil {
		return empty, e
	}
	reports := make([]support.Report, 0, len(rings)-1)
	volumes := make([]*big.Rat, 0, len(rings)-1)
	sum := new(big.Rat)
	maxBits := o.Support.MaxBits
	if maxBits == 0 {
		maxBits = 16384
	}
	for i := 0; i+1 < len(rings); i++ {
		band, e := Planar(ctx, rings[i], rings[i+1], o.Options)
		if e != nil {
			return empty, e
		}
		reports = append(reports, band.Supports)
		volumes = append(volumes, new(big.Rat).Set(band.ExactVolume))
		sum.Add(sum, band.ExactVolume)
		if sum.Num().BitLen() > maxBits || sum.Denom().BitLen() > maxBits {
			return empty, ErrLimit
		}
	}
	if sum.Cmp(assembled.ExactVolume) != 0 {
		return empty, ErrUnsupported
	}
	return BandResult{StationResult: assembled, BandSupports: reports, BandVolumes: volumes}, nil
}
