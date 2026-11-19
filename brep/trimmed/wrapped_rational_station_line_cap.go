//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// WrappedRationalBoundarySpan is a closed coincident boundary interval. Faces owns
// its relative interior; StartFaces/EndFaces additionally include an original
// adjacent face exactly when that endpoint is on an original rim. Algebraic brackets identify
// endpoints, not rounded parameters. These spans are never material interior.
type WrappedRationalBoundarySpan struct {
	Start, End                  geom.ToroidLineRoot
	Faces, StartFaces, EndFaces []int
}

// WrappedRationalStationCapLine separates cap coincidence from material. Hits
// includes rim contacts and an admitted singleton cap clip; the latter uses
// the existing endpoint multiplicity convention, not an isolated plane root.
type WrappedRationalStationCapLine struct {
	Hits     []WrappedLoftLineHit
	Boundary []WrappedRationalBoundarySpan
}

// ClipWrappedRationalStationLoftCapLineRational accepts an exact horizontal
// rational line on either original terminal cap, including minT==maxT. It
// preserves the original full weighted-profile cap and rim owners. Internal
// station planes and nonhorizontal generators are outside this API. Geometry
// retains its existing bounded conic/overlap proof; MaxEvents bounds returned
// contacts plus spans. MaxPointOperations is unused (there is no gap classifier).
// All failures are atomic; the isolated-contact API remains unchanged.
func (s Solid) ClipWrappedRationalStationLoftCapLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalStationCapLine, error) {
	empty := WrappedRationalStationCapLine{}
	if o.MaxEvents == 0 {
		o.MaxEvents = 128
	}
	if ctx == nil || o.MaxEvents < 1 || o.MaxEvents > 256 || !finite(o.Geometry.ParameterTolerance) || o.Geometry.ParameterTolerance <= 0 || o.Geometry.MaxSubdivisions < 0 || o.Geometry.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	bits := o.Geometry.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 || bits > 65536 {
		return empty, geom.ErrInvalid
	}
	valid := func(q *big.Rat) bool { return q != nil && q.Num().BitLen() <= bits && q.Denom().BitLen() <= bits }
	if !valid(minT) || !valid(maxT) || minT.Cmp(maxT) > 0 {
		return empty, geom.ErrInvalid
	}
	for i := 0; i < 3; i++ {
		if !valid(origin[i]) || !valid(direction[i]) {
			return empty, geom.ErrInvalid
		}
	}
	if direction[2].Sign() != 0 || direction[0].Sign() == 0 && direction[1].Sign() == 0 {
		return empty, geom.ErrInvalid
	}
	proof, bands, e := s.validateWrappedProfileStationLoft(ctx, o.Geometry.Surface, false)
	if e != nil {
		return empty, e
	}
	cap, band := -1, -1
	if origin[2].Cmp(proof.Heights[0]) == 0 {
		cap, band = 0, 0
	}
	if origin[2].Cmp(proof.Heights[len(proof.Heights)-1]) == 0 {
		cap, band = 1, len(bands)-1
	}
	if cap < 0 {
		return empty, geom.ErrDomain
	}
	report, e := geom.IntersectWrappedFrustumLineRational(ctx, bands[band], origin, direction, minT, maxT, o.Geometry)
	if e != nil {
		return empty, e
	}
	if len(report.Hits) > 2 || len(report.Overlaps) > 1 || len(report.Hits)+len(report.Overlaps) > o.MaxEvents {
		return empty, geom.ErrConvergence
	}
	part := geom.CylinderLowerCap
	if cap == 1 {
		part = geom.CylinderUpperCap
	}
	capFace, wallFace := proof.History.CapFaces[cap], proof.History.WallFaces[band]
	clone := func(q geom.ToroidLineRoot) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(q.Min), Max: new(big.Rat).Set(q.Max), Multiplicity: q.Multiplicity}
	}
	validRoot := func(q geom.ToroidLineRoot) bool {
		return valid(q.Min) && valid(q.Max) && q.Min.Cmp(q.Max) <= 0 && q.Min.Cmp(minT) >= 0 && q.Max.Cmp(maxT) <= 0
	}
	result := WrappedRationalStationCapLine{}
	for _, hit := range report.Hits {
		if !validRoot(hit.Parameter) || hit.Parts&part == 0 || hit.Parts & ^(part|geom.CylinderLateral) != 0 {
			return empty, geom.ErrConvergence
		}
		faces := []int{capFace}
		if hit.Parts&geom.CylinderLateral != 0 {
			faces = append(faces, wallFace)
		}
		sort.Ints(faces)
		if n := len(result.Hits); n > 0 && result.Hits[n-1].Parameter.Max.Cmp(hit.Parameter.Min) >= 0 {
			return empty, geom.ErrConvergence
		}
		result.Hits = append(result.Hits, WrappedLoftLineHit{Parameter: clone(hit.Parameter), Faces: faces, Parts: hit.Parts})
	}
	endpointOwners := func(q geom.ToroidLineRoot) ([]int, error) {
		faces := []int{capFace}
		for _, hit := range result.Hits {
			h := hit.Parameter
			if h.Min.Cmp(q.Min) == 0 && h.Max.Cmp(q.Max) == 0 {
				return append([]int(nil), hit.Faces...), nil
			}
			if h.Max.Cmp(q.Min) >= 0 && q.Max.Cmp(h.Min) >= 0 {
				return nil, geom.ErrConvergence
			}
		}
		return faces, nil
	}
	for _, span := range report.Overlaps {
		if span.Part != part || !validRoot(span.Start) || !validRoot(span.End) || span.Start.Max.Cmp(span.End.Min) >= 0 {
			return empty, geom.ErrConvergence
		}
		first, e := endpointOwners(span.Start)
		if e != nil {
			return empty, e
		}
		last, e := endpointOwners(span.End)
		if e != nil {
			return empty, e
		}
		result.Boundary = append(result.Boundary, WrappedRationalBoundarySpan{Start: clone(span.Start), End: clone(span.End), Faces: []int{capFace}, StartFaces: first, EndFaces: last})
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
