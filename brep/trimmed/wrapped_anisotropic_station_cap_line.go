//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// WrappedAnisotropicCapLine contains only closed original cap boundary spans
// and rim/singleton contacts. It never reports material-volume interior.
type WrappedAnisotropicCapLine struct {
	Hits            []WrappedAnisotropicLineHit
	Boundary        []WrappedRationalBoundarySpan
	ExactOperations int
}

// ClipWrappedAnisotropicStationCapLineRational accepts a rational horizontal
// line exactly on an exposed original terminal cap, including singleton clips.
// Original cap, wall, rim and vertex owners are retained. Internal station
// planes, uncertain quadrant ownership and nonhorizontal lines fail atomically.
// The cumulative query and separate native authentication quotas are those of
// WrappedAnisotropicMultiBandLineOptions; MaxEvents bounds hits plus spans.
func (s Solid) ClipWrappedAnisotropicStationCapLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicMultiBandLineOptions) (WrappedAnisotropicCapLine, error) {
	empty := WrappedAnisotropicCapLine{}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 512
	}
	bits := o.Geometry.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if ctx == nil || o.MaxOperations < 1 || o.MaxOperations > 8000000 || o.MaxEvents < 1 || o.MaxEvents > 512 || bits < 64 || bits > 65536 || !finite(o.Geometry.ParameterTolerance) || o.Geometry.ParameterTolerance <= 0 || o.Geometry.MaxSubdivisions < 0 || o.Geometry.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: bits}
	for _, q := range append(append([]*big.Rat{minT, maxT}, origin[:]...), direction[:]...) {
		if q == nil {
			return empty, geom.ErrInvalid
		}
		b.check(q)
	}
	if b.err != nil {
		return empty, b.err
	}
	if minT.Cmp(maxT) > 0 || direction[2].Sign() != 0 || (direction[0].Sign() == 0 && direction[1].Sign() == 0) {
		return empty, geom.ErrInvalid
	}
	proof, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, o.Geometry.Surface)
	if e != nil {
		return empty, e
	}
	clone := func(q geom.ToroidLineRoot) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(q.Min), Max: new(big.Rat).Set(q.Max), Multiplicity: q.Multiplicity}
	}
	exact := func(t *big.Rat) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(t), Max: new(big.Rat).Set(t), Multiplicity: 1}
	}
	point := func(t *big.Rat) (WrappedAnisotropicStationPoint, error) {
		var q [3]*big.Rat
		for a := range q {
			q[a] = b.add(origin[a], b.mul(direction[a], t))
		}
		if b.err != nil {
			return WrappedAnisotropicStationPoint{}, b.err
		}
		remaining := b.limit - b.used
		if remaining < 1 {
			return WrappedAnisotropicStationPoint{}, geom.ErrConvergence
		}
		v, e := s.locatePreparedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{Source: o.Geometry.Surface, MaxPointOperations: min(remaining, 1000000), MaxCoefficientBits: bits}, &proof, bands)
		if e != nil {
			return WrappedAnisotropicStationPoint{}, e
		}
		b.used += v.ExactOperations
		if b.used > b.limit {
			return WrappedAnisotropicStationPoint{}, geom.ErrConvergence
		}
		return v, nil
	}
	cap, band, station := -1, -1, -1
	if origin[2].Cmp(proof.Heights[0]) == 0 {
		cap = 0
		band = 0
		station = 0
	}
	if origin[2].Cmp(proof.Heights[len(proof.Heights)-1]) == 0 {
		cap = 1
		band = len(bands) - 1
		station = len(proof.Heights) - 1
	}
	if cap < 0 {
		return empty, geom.ErrInvalid
	}
	capFace := proof.History.CapFaces[cap]
	capPart := geom.CylinderLowerCap
	if cap == 1 {
		capPart = geom.CylinderUpperCap
	}
	hitAt := func(t *big.Rat) (WrappedAnisotropicLineHit, bool, error) {
		q, e := point(t)
		if e != nil {
			return WrappedAnisotropicLineHit{}, false, e
		}
		if q.Location == parametric.Outside {
			return WrappedAnisotropicLineHit{}, false, nil
		}
		if q.Location != parametric.Boundary {
			return WrappedAnisotropicLineHit{}, false, geom.ErrConvergence
		}
		parts := capPart
		if len(q.RimEdges) > 0 {
			parts |= geom.CylinderLateral
		}
		return WrappedAnisotropicLineHit{exact(t), parts, q.Faces, q.Edges, q.RimEdges, q.Vertices}, true, nil
	}
	if minT.Cmp(maxT) == 0 {
		h, ok, e := hitAt(minT)
		if e != nil {
			return empty, e
		}
		q := WrappedAnisotropicCapLine{ExactOperations: b.used}
		if ok {
			q.Hits = []WrappedAnisotropicLineHit{h}
		}
		if e = ctx.Err(); e != nil {
			return empty, e
		}
		return q, nil
	}
	remaining := b.limit - b.used
	if remaining < 1 {
		return empty, geom.ErrConvergence
	}
	goOptions := o.Geometry
	goOptions.Surface.MaxOperations = remaining
	goOptions.Surface.MaxCoefficientBits = bits
	local, e := geom.IntersectWrappedAnisotropicClosedBandLineRational(ctx, bands[band], origin, direction, minT, maxT, goOptions)
	if e != nil {
		return empty, e
	}
	b.used += local.ExactOperations
	if b.used > b.limit || len(local.Intersections.Overlaps) > 0 || len(local.Intersections.Hits) > o.MaxEvents {
		return empty, geom.ErrConvergence
	}
	result := WrappedAnisotropicCapLine{}
	for _, h := range local.Intersections.Hits {
		root := h.Parameter
		if h.Parts != geom.CylinderLateral {
			return empty, geom.ErrConvergence
		}
		if root.Min.Cmp(root.Max) == 0 {
			hit, ok, e := hitAt(root.Min)
			if e != nil {
				return empty, e
			}
			if !ok || len(hit.RimEdges) == 0 {
				return empty, geom.ErrConvergence
			}
			hit.Parameter.Multiplicity = root.Multiplicity
			result.Hits = append(result.Hits, hit)
			continue
		}
		// Affine coordinate signs at both enclosure endpoints prove the unique
		// original quadrant over the entire root interval. No rounded azimuth.
		var signs [2]int
		for axis := 0; axis < 2; axis++ {
			lo := b.sub(b.add(origin[axis], b.mul(direction[axis], root.Min)), proof.Center[axis])
			hi := b.sub(b.add(origin[axis], b.mul(direction[axis], root.Max)), proof.Center[axis])
			if b.err != nil {
				return empty, b.err
			}
			if lo.Sign() == 0 || lo.Sign() != hi.Sign() {
				return empty, geom.ErrConvergence
			}
			signs[axis] = lo.Sign()
		}
		quadrant := 0
		if signs[0] < 0 {
			if signs[1] > 0 {
				quadrant = 1
			} else {
				quadrant = 2
			}
		} else if signs[1] < 0 {
			quadrant = 3
		}
		edge := proof.History.StationRims[station][quadrant]
		faces := []int{capFace, proof.History.WallFaces[band]}
		sort.Ints(faces)
		result.Hits = append(result.Hits, WrappedAnisotropicLineHit{Parameter: clone(root), Parts: capPart | geom.CylinderLateral, Faces: faces, Edges: []int{edge}, RimEdges: []int{edge}})
	}
	ends := []geom.ToroidLineRoot{exact(minT)}
	owners := [][]int{{capFace}}
	for _, h := range result.Hits {
		ends = append(ends, clone(h.Parameter))
		owners = append(owners, append([]int(nil), h.Faces...))
	}
	ends = append(ends, exact(maxT))
	owners = append(owners, []int{capFace})
	for i := 1; i < len(ends); i++ {
		a, c := ends[i-1], ends[i]
		if a.Max.Cmp(c.Min) >= 0 {
			if a.Min.Cmp(a.Max) == 0 && c.Min.Cmp(c.Max) == 0 && a.Min.Cmp(c.Min) == 0 {
				continue
			}
			return empty, geom.ErrConvergence
		}
		t := b.div(b.add(a.Max, c.Min), big.NewRat(2, 1))
		if b.err != nil {
			return empty, b.err
		}
		q, e := point(t)
		if e != nil {
			return empty, e
		}
		if q.Location == parametric.Outside {
			continue
		}
		if q.Location != parametric.Boundary || len(q.Faces) != 1 || q.Faces[0] != capFace || len(q.RimEdges) != 0 {
			return empty, geom.ErrConvergence
		}
		if len(result.Hits)+len(result.Boundary) >= o.MaxEvents {
			return empty, geom.ErrConvergence
		}
		result.Boundary = append(result.Boundary, WrappedRationalBoundarySpan{Start: clone(a), End: clone(c), Faces: []int{capFace}, StartFaces: append([]int(nil), owners[i-1]...), EndFaces: append([]int(nil), owners[i]...)})
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	result.ExactOperations = b.used
	return result, nil
}
