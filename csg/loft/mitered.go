package loft

import (
	"context"
	"math/big"
	"plainsolid/brep/support"
	"plainsolid/geom"
)

// MiteredStations constructs piecewise convex cells bounded by corresponding
// planar rings that may have different normals. Every ring plane must strictly
// separate ALL earlier and later original ring vertices. Neighboring oriented
// ring normals must have positive dot product. Each cell must be convex with
// exactly planar side quads; final shell embedding is independently checked.
// This is a separated miter-cell construction, not an arbitrary path sweep.
// BandResult retains original miter-loop edge/vertex identities. Each cell and
// final shell have separate proof budgets; all-ring separation uses one shared
// Support.MaxWork budget, in addition to the global station/vertex count limits.
func MiteredStations(ctx context.Context, rings [][]geom.Vec3, o StationOptions) (BandResult, error) {
	empty := BandResult{}
	s, e := stationShell(ctx, rings, o, false, false)
	if e != nil {
		return empty, e
	}
	reports := make([]support.Report, 0, len(rings)-1)
	volumes := make([]*big.Rat, 0, len(rings)-1)
	sum := new(big.Rat)
	bits := o.Support.MaxBits
	if bits == 0 {
		bits = 16384
	}
	for i := 0; i+1 < len(rings); i++ {
		r, e := planarPair(ctx, rings[i], rings[i+1], o.Options, false)
		if e != nil {
			return empty, e
		}
		reports = append(reports, r.Supports)
		volumes = append(volumes, new(big.Rat).Set(r.ExactVolume))
		sum.Add(sum, r.ExactVolume)
		if sum.Num().BitLen() > bits || sum.Denom().BitLen() > bits {
			return empty, ErrLimit
		}
	}
	if sum.Cmp(s.ExactVolume) != 0 {
		return empty, ErrUnsupported
	}
	return BandResult{StationResult: s, BandSupports: reports, BandVolumes: volumes}, nil
}
