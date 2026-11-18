//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// WrappedAnisotropicMultiBandLineOptions uses one query operation ledger for
// clipping, all band proofs/root signs and prepared exact point classifications.
// Native source authentication is separately bounded by Geometry.Surface's
// existing per-pass limits and the sixteen-band source limit. MaxOperations
// defaults to 1M (maximum8M); MaxEvents defaults/max512 before merging.
// Root subdivision limits remain per quartic/sign proof as in the band API.
type WrappedAnisotropicMultiBandLineOptions struct {
	Geometry                 geom.ToroidLineOptions
	MaxOperations, MaxEvents int
}
type WrappedAnisotropicLineHit struct {
	Parameter                        geom.ToroidLineRoot
	Parts                            geom.CylinderBoundaryPart
	Faces, Edges, RimEdges, Vertices []int
}
type WrappedAnisotropicMultiBandLine struct {
	Hits            []WrappedAnisotropicLineHit
	Interior        []WrappedRationalLineInterior
	ExactOperations int
}

// ClipWrappedAnisotropicStationLineRational certifies nonhorizontal finite
// lines across the complete original loft. Exact station contacts retain both
// incident walls and original rim/vertex identities; only terminal disks are
// caps. Coincident boundary polynomials and ambiguous root ordering fail closed.
func (s Solid) ClipWrappedAnisotropicStationLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicMultiBandLineOptions) (WrappedAnisotropicMultiBandLine, error) {
	empty := WrappedAnisotropicMultiBandLine{}
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
	if minT.Cmp(maxT) >= 0 || direction[2].Sign() == 0 {
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
	var hits []WrappedAnisotropicLineHit
	appendHit := func(h WrappedAnisotropicLineHit) error {
		if len(hits) >= o.MaxEvents {
			return geom.ErrConvergence
		}
		hits = append(hits, h)
		return nil
	}
	stations := make([]*big.Rat, len(proof.Heights))
	for i, z := range proof.Heights {
		stations[i] = b.div(b.sub(z, origin[2]), direction[2])
		if b.err != nil {
			return empty, b.err
		}
	}
	for i, w := range bands {
		lo, hi := stations[i], stations[i+1]
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if lo.Cmp(minT) < 0 {
			lo = minT
		}
		if hi.Cmp(maxT) > 0 {
			hi = maxT
		}
		if lo.Cmp(hi) >= 0 {
			continue
		}
		remaining := b.limit - b.used
		if remaining < 1 {
			return empty, geom.ErrConvergence
		}
		goOptions := o.Geometry
		goOptions.Surface.MaxOperations = remaining
		goOptions.Surface.MaxCoefficientBits = bits
		q, e := geom.IntersectWrappedAnisotropicClosedBandLineRational(ctx, w, origin, direction, lo, hi, goOptions)
		if e != nil {
			return empty, e
		}
		b.used += q.ExactOperations
		if b.used > b.limit {
			return empty, geom.ErrConvergence
		}
		if len(q.Intersections.Overlaps) != 0 {
			return empty, geom.ErrConvergence
		}
		for _, h := range q.Intersections.Hits {
			if h.Parts != geom.CylinderLateral {
				return empty, geom.ErrConvergence
			}
			if e = appendHit(WrappedAnisotropicLineHit{Parameter: clone(h.Parameter), Parts: h.Parts, Faces: []int{proof.History.WallFaces[i]}}); e != nil {
				return empty, e
			}
		}
	}
	for i, t := range stations {
		if t.Cmp(minT) < 0 || t.Cmp(maxT) > 0 {
			continue
		}
		q, e := point(t)
		if e != nil {
			return empty, e
		}
		if q.Location != parametric.Boundary {
			continue
		}
		parts := geom.CylinderBoundaryPart(0)
		if len(q.RimEdges) > 0 {
			parts |= geom.CylinderLateral
		}
		if i == 0 {
			parts |= geom.CylinderLowerCap
		}
		if i == len(stations)-1 {
			parts |= geom.CylinderUpperCap
		}
		if e = appendHit(WrappedAnisotropicLineHit{exact(t), parts, q.Faces, q.Edges, q.RimEdges, q.Vertices}); e != nil {
			return empty, e
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Parameter.Min.Cmp(hits[j].Parameter.Min) < 0 })
	merged := make([]WrappedAnisotropicLineHit, 0, len(hits))
	union := func(a, b []int) []int {
		for _, v := range b {
			a = appendUniqueFace(a, v)
		}
		sort.Ints(a)
		return a
	}
	for _, h := range hits {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			a, c := last.Parameter, h.Parameter
			if a.Max.Cmp(c.Min) >= 0 {
				if a.Min.Cmp(a.Max) != 0 || c.Min.Cmp(c.Max) != 0 || a.Min.Cmp(c.Min) != 0 {
					return empty, geom.ErrConvergence
				}
				last.Faces = union(last.Faces, h.Faces)
				last.Edges = union(last.Edges, h.Edges)
				last.RimEdges = union(last.RimEdges, h.RimEdges)
				last.Vertices = union(last.Vertices, h.Vertices)
				last.Parts |= h.Parts
				last.Parameter.Multiplicity = max(a.Multiplicity, c.Multiplicity)
				continue
			}
		}
		merged = append(merged, h)
	}
	ends := []geom.ToroidLineRoot{exact(minT)}
	for _, h := range merged {
		ends = append(ends, clone(h.Parameter))
	}
	ends = append(ends, exact(maxT))
	result := WrappedAnisotropicMultiBandLine{Hits: merged}
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
		switch q.Location {
		case parametric.Inside:
			result.Interior = append(result.Interior, WrappedRationalLineInterior{clone(a), clone(c)})
		case parametric.Outside:
		default:
			return empty, geom.ErrConvergence
		}
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	result.ExactOperations = b.used
	return result, nil
}
