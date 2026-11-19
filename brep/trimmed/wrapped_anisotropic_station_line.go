//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ClipWrappedAnisotropicStationHorizontalLineRational certifies isolated wall
// hits and open material intervals strictly between original station planes.
// The exact diagonal inverse preserves the authored unnormalized line t.
// Station-plane, nonhorizontal and coincident-boundary lines are unsupported.
// Options and per-stage source/gap quotas match WrappedRationalStationLineOptions.
func (s Solid) ClipWrappedAnisotropicStationHorizontalLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalStationLine, error) {
	empty := WrappedRationalStationLine{}
	if o.MaxEvents == 0 {
		o.MaxEvents = 128
	}
	if o.MaxPointOperations == 0 {
		o.MaxPointOperations = 65536
	}
	if ctx == nil || o.MaxEvents < 1 || o.MaxEvents > 256 || o.MaxPointOperations < 1 || o.MaxPointOperations > 1000000 || !finite(o.Geometry.ParameterTolerance) || o.Geometry.ParameterTolerance <= 0 || o.Geometry.MaxSubdivisions < 0 || o.Geometry.MaxSubdivisions > 65536 {
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
	if !valid(minT) || !valid(maxT) || minT.Cmp(maxT) >= 0 {
		return empty, geom.ErrInvalid
	}
	nonzero := false
	for i := 0; i < 3; i++ {
		if !valid(origin[i]) || !valid(direction[i]) {
			return empty, geom.ErrInvalid
		}
		nonzero = nonzero || direction[i].Sign() != 0
	}
	if !nonzero {
		return empty, geom.ErrInvalid
	}
	p, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, o.Geometry.Surface)
	if e != nil {
		return empty, e
	}
	if direction[2].Sign() != 0 {
		return empty, geom.ErrInvalid
	}
	band := -1
	for i, h := range p.Heights {
		if origin[2].Cmp(h) == 0 {
			return empty, geom.ErrInvalid
		}
		if origin[2].Cmp(h) < 0 {
			band = i - 1
			break
		}
	}
	if band < 0 {
		return empty, nil
	}
	cloneRoot := func(q geom.ToroidLineRoot) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(q.Min), Max: new(big.Rat).Set(q.Max), Multiplicity: q.Multiplicity}
	}
	report, e := geom.IntersectWrappedAnisotropicHorizontalLineRational(ctx, bands[band], origin, direction, minT, maxT, o.Geometry)
	if e != nil {
		return empty, e
	}
	if len(report.Overlaps) != 0 || len(report.Hits) > o.MaxEvents {
		return empty, geom.ErrConvergence
	}
	var hits []WrappedLoftLineHit
	for _, hit := range report.Hits {
		q := hit.Parameter
		if hit.Parts != geom.CylinderLateral || !valid(q.Min) || !valid(q.Max) || q.Min.Cmp(q.Max) > 0 || q.Min.Cmp(minT) < 0 || q.Max.Cmp(maxT) > 0 {
			return empty, geom.ErrConvergence
		}
		hits = append(hits, WrappedLoftLineHit{Parameter: cloneRoot(q), Parts: geom.CylinderLateral, Faces: []int{p.History.WallFaces[band]}})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Parameter.Min.Cmp(hits[j].Parameter.Min) < 0 })
	merged := make([]WrappedLoftLineHit, 0, len(hits))
	for _, hit := range hits {
		if len(merged) > 0 {
			previous := &merged[len(merged)-1]
			a, b := previous.Parameter, hit.Parameter
			if a.Max.Cmp(b.Min) >= 0 {
				if a.Min.Cmp(a.Max) != 0 || b.Min.Cmp(b.Max) != 0 || a.Min.Cmp(b.Min) != 0 {
					return empty, geom.ErrConvergence
				}
				for _, face := range hit.Faces {
					previous.Faces = appendUniqueFace(previous.Faces, face)
				}
				previous.Parts |= hit.Parts
				previous.Parameter.Multiplicity = max(a.Multiplicity, b.Multiplicity)
				continue
			}
		}
		merged = append(merged, hit)
	}
	for i := range merged {
		sort.Ints(merged[i].Faces)
	}
	exact := func(q *big.Rat) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(q), Max: new(big.Rat).Set(q)}
	}
	ends := []geom.ToroidLineRoot{exact(minT)}
	for _, hit := range merged {
		ends = append(ends, cloneRoot(hit.Parameter))
	}
	ends = append(ends, exact(maxT))
	allowance := o.MaxPointOperations / (len(ends) - 1)
	if allowance < 1 {
		return empty, geom.ErrConvergence
	}
	result := WrappedRationalStationLine{Hits: merged}
	for i := 1; i < len(ends); i++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		left, right := ends[i-1], ends[i]
		cmp := left.Max.Cmp(right.Min)
		if cmp >= 0 {
			if cmp == 0 && left.Min.Cmp(left.Max) == 0 && right.Min.Cmp(right.Max) == 0 {
				continue
			}
			return empty, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Add(left.Max, right.Min), big.NewRat(2, 1))
		if !valid(t) {
			return empty, geom.ErrConvergence
		}
		var point [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			v := new(big.Rat).Mul(t, direction[axis])
			if !valid(v) {
				return empty, geom.ErrConvergence
			}
			point[axis] = new(big.Rat).Add(origin[axis], v)
			if !valid(point[axis]) {
				return empty, geom.ErrConvergence
			}
		}
		classification, e := s.LocateWrappedAnisotropicStationRational(ctx, point, WrappedRationalStationPointOptions{Source: o.Geometry.Surface, MaxPointOperations: allowance, MaxCoefficientBits: bits})
		if e != nil {
			return empty, e
		}
		switch classification.Location {
		case parametric.Inside:
			result.Interior = append(result.Interior, WrappedRationalLineInterior{cloneRoot(left), cloneRoot(right)})
		case parametric.Outside:
		case parametric.Boundary:
			return empty, geom.ErrConvergence
		default:
			return empty, geom.ErrConvergence
		}
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
