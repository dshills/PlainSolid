package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAnisotropicGeneratorOptions has no geometric tolerance. MaxOperations
// (default1M, maximum8M) covers the affine proof, clipping and prepared point
// classifications. Native source authentication retains Source's separate
// bounded per-pass quotas and the original sixteen-band source limit.
type WrappedAnisotropicGeneratorOptions struct {
	Source        geom.SurfacePatchOptions
	MaxOperations int
}

// WrappedAnisotropicGeneratorSpan is a closed selected-wall boundary interval.
// Faces/Edges own its relative interior; Start/End retain all original endpoint
// owners, including the other wall at a crease and terminal caps/rim vertices.
type WrappedAnisotropicGeneratorSpan struct {
	Start, End   WrappedAnisotropicLineHit
	Faces, Edges []int
}
type WrappedAnisotropicGeneratorLine struct {
	WallFace        int
	Boundary        []WrappedAnisotropicGeneratorSpan
	Hits            []WrappedAnisotropicLineHit // singleton clip only
	ExactOperations int
}

// ClipWrappedAnisotropicStationGeneratorRational proves that the authored line
// is a generator of the explicitly selected original wall, then clips to that
// wall's closed height interval. Exact affine identities prove constant normalized
// profile coordinates throughout the band; one exact retained profile membership
// is sufficient only after those identities. No assertion covers the line outside
// the selected wall. Nongenerators and horizontal lines fail, not approximate.
func (s Solid) ClipWrappedAnisotropicStationGeneratorRational(ctx context.Context, wallFace int, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicGeneratorOptions) (WrappedAnisotropicGeneratorLine, error) {
	empty := WrappedAnisotropicGeneratorLine{}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	bits := o.Source.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if ctx == nil || o.MaxOperations < 1 || o.MaxOperations > 8000000 || bits < 64 || bits > 65536 {
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
	if minT.Cmp(maxT) > 0 || direction[2].Sign() == 0 {
		return empty, geom.ErrInvalid
	}
	proof, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, o.Source)
	if e != nil {
		return empty, e
	}
	band := -1
	for i, id := range proof.History.WallFaces {
		if id == wallFace {
			band = i
			break
		}
	}
	if band < 0 {
		return empty, geom.ErrInvalid
	}
	z0, z1 := proof.Heights[band], proof.Heights[band+1]
	for axis := 0; axis < 2; axis++ {
		slope := b.div(b.sub(proof.Scales[band+1][axis], proof.Scales[band][axis]), b.sub(z1, z0))
		s0 := b.add(proof.Scales[band][axis], b.mul(slope, b.sub(origin[2], z0)))
		s1 := b.mul(slope, direction[2])
		x0 := b.sub(origin[axis], proof.Center[axis])
		determinant := b.sub(b.mul(direction[axis], s0), b.mul(x0, s1))
		if b.err != nil {
			return empty, b.err
		}
		if determinant.Sign() != 0 {
			return empty, geom.ErrDomain
		}
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
		v, e := s.locatePreparedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{Source: o.Source, MaxPointOperations: min(remaining, 1000000), MaxCoefficientBits: bits}, &proof, bands)
		if e != nil {
			return WrappedAnisotropicStationPoint{}, e
		}
		b.used += v.ExactOperations
		if b.used > b.limit {
			return WrappedAnisotropicStationPoint{}, geom.ErrConvergence
		}
		return v, nil
	}
	middle := b.div(b.sub(b.div(b.add(z0, z1), big.NewRat(2, 1)), origin[2]), direction[2])
	if b.err != nil {
		return empty, b.err
	}
	mid, e := point(middle)
	if e != nil {
		return empty, e
	}
	if mid.Location != parametric.Boundary || len(mid.Faces) != 1 || mid.Faces[0] != wallFace || len(mid.RimEdges) != 0 || len(mid.Vertices) != 0 {
		return empty, geom.ErrDomain
	}
	lo := b.div(b.sub(z0, origin[2]), direction[2])
	hi := b.div(b.sub(z1, origin[2]), direction[2])
	if b.err != nil {
		return empty, b.err
	}
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	if lo.Cmp(minT) < 0 {
		lo = minT
	}
	if hi.Cmp(maxT) > 0 {
		hi = maxT
	}
	result := WrappedAnisotropicGeneratorLine{WallFace: wallFace}
	finish := func() (WrappedAnisotropicGeneratorLine, error) {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		result.ExactOperations = b.used
		return result, nil
	}
	if lo.Cmp(hi) > 0 {
		return finish()
	}
	contact := func(t *big.Rat) (WrappedAnisotropicLineHit, error) {
		q, e := point(t)
		if e != nil {
			return WrappedAnisotropicLineHit{}, e
		}
		owns := false
		for _, id := range q.Faces {
			owns = owns || id == wallFace
		}
		if q.Location != parametric.Boundary || !owns {
			return WrappedAnisotropicLineHit{}, geom.ErrConvergence
		}
		parts := geom.CylinderLateral
		for _, id := range q.Faces {
			if id == proof.History.CapFaces[0] {
				parts |= geom.CylinderLowerCap
			}
			if id == proof.History.CapFaces[1] {
				parts |= geom.CylinderUpperCap
			}
		}
		return WrappedAnisotropicLineHit{exact(t), parts, q.Faces, q.Edges, q.RimEdges, q.Vertices}, nil
	}
	start, e := contact(lo)
	if e != nil {
		return empty, e
	}
	if lo.Cmp(hi) == 0 {
		result.Hits = []WrappedAnisotropicLineHit{start}
		return finish()
	}
	end, e := contact(hi)
	if e != nil {
		return empty, e
	}
	result.Boundary = []WrappedAnisotropicGeneratorSpan{{Start: start, End: end, Faces: []int{wallFace}, Edges: append([]int(nil), mid.Edges...)}}
	return finish()
}
