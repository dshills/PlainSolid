package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// WrappedAnisotropicStationPoint reports all incident original native entities.
// Faces, Edges, RimEdges and Vertices are empty away from material boundary.
// Edges also includes the original periodic wall seam at positive X; RimEdges
// is only its station-rim subset. Cardinal rim vertices own two adjacent rim
// edges, never a falsely unique curve parameter. Every slice is owned.
type WrappedAnisotropicStationPoint struct {
	Location                         parametric.Location
	Faces, Edges, RimEdges, Vertices []int
	ExactOperations                  int
}

// LocateWrappedAnisotropicStationRational proves source topology before one
// exact query. Query arithmetic and the guarded original conic call share
// MaxPointOperations; native source proof has the separately documented Source
// quotas. Artificial internal band disks are material interior, not caps.
func (s Solid) LocateWrappedAnisotropicStationRational(ctx context.Context, point [3]*big.Rat, o WrappedRationalStationPointOptions) (WrappedAnisotropicStationPoint, error) {
	return s.locatePreparedAnisotropicStationRational(ctx, point, o, nil, nil)
}

func (s Solid) locatePreparedAnisotropicStationRational(ctx context.Context, point [3]*big.Rat, o WrappedRationalStationPointOptions, prepared *WrappedAnisotropicStationProof, bands []*geom.WrappedSurfacePatch) (WrappedAnisotropicStationPoint, error) {
	empty := WrappedAnisotropicStationPoint{}
	if o.MaxPointOperations == 0 {
		o.MaxPointOperations = 4096
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if ctx == nil || o.MaxPointOperations < 1 || o.MaxPointOperations > 1000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return empty, geom.ErrInvalid
	}
	b := &stationMassBudget{ctx: ctx, limit: o.MaxPointOperations, bits: o.MaxCoefficientBits}
	var q [3]*big.Rat
	for i, v := range point {
		if v == nil {
			return empty, geom.ErrInvalid
		}
		b.check(v)
		if b.err != nil {
			return empty, b.err
		}
		q[i] = new(big.Rat).Set(v)
	}
	var proof WrappedAnisotropicStationProof
	if prepared == nil {
		var e error
		proof, bands, e = s.validateWrappedAnisotropicStationLoft(ctx, o.Source)
		if e != nil {
			return empty, e
		}
	} else {
		proof = *prepared
	}
	band, station := -1, -1
	for i, z := range proof.Heights {
		b.check(z)
		b.check(q[2])
		if b.err != nil {
			return empty, b.err
		}
		cmp := q[2].Cmp(z)
		if cmp == 0 {
			station = i
			band = max(0, i-1)
			break
		}
		if cmp < 0 {
			band = i - 1
			break
		}
	}
	result := WrappedAnisotropicStationPoint{Location: parametric.Outside}
	finish := func() (WrappedAnisotropicStationPoint, error) {
		if b.err != nil {
			return empty, b.err
		}
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		result.ExactOperations = b.used
		return result, nil
	}
	if band < 0 {
		return finish()
	}
	dx, dy := b.sub(q[0], proof.Center[0]), b.sub(q[1], proof.Center[1])
	if b.err != nil {
		return empty, b.err
	}
	if b.used >= b.limit {
		return empty, geom.ErrConvergence
	}
	options := o.Source
	options.MaxOperations = b.limit - b.used
	options.MaxCoefficientBits = o.MaxCoefficientBits
	local, e := geom.LocateWrappedAnisotropicRational(ctx, bands[band], q, options)
	if e != nil {
		return empty, e
	}
	b.used += local.ExactOperations
	if b.used > b.limit {
		return empty, geom.ErrConvergence
	}
	if local.Location == geom.CylinderOutside {
		return finish()
	}
	result.Location = parametric.Inside
	if local.Parts&geom.CylinderLateral != 0 {
		result.Location = parametric.Boundary
		result.Faces = []int{proof.History.WallFaces[band]}
		if station > 0 && station < len(proof.Heights)-1 {
			result.Faces = appendUniqueFace(result.Faces, proof.History.WallFaces[station])
		}
		// The original seam is a topological edge even when geometrically smooth.
		if dy.Sign() == 0 && dx.Sign() > 0 {
			for _, id := range result.Faces {
				counts := map[int]int{}
				for _, use := range s.faces[id].Outer {
					counts[use.Edge]++
				}
				seam := -1
				for edge, count := range counts {
					if count == 2 {
						if seam >= 0 {
							return empty, geom.ErrConvergence
						}
						seam = edge
					}
				}
				if seam < 0 {
					return empty, geom.ErrConvergence
				}
				result.Edges = appendUniqueFace(result.Edges, seam)
			}
		}
		if station >= 0 {
			rim := proof.History.StationRims[station]
			add := func(index int) {
				result.RimEdges = appendUniqueFace(result.RimEdges, rim[index])
				result.Edges = appendUniqueFace(result.Edges, rim[index])
			}
			if dx.Sign() >= 0 && dy.Sign() >= 0 {
				add(0)
			}
			if dx.Sign() <= 0 && dy.Sign() >= 0 {
				add(1)
			}
			if dx.Sign() <= 0 && dy.Sign() <= 0 {
				add(2)
			}
			if dx.Sign() >= 0 && dy.Sign() <= 0 {
				add(3)
			}
			if len(result.RimEdges) == 2 {
				a, c := s.edges[result.RimEdges[0]], s.edges[result.RimEdges[1]]
				for _, v := range []int{a.Start, a.End} {
					if v == c.Start || v == c.End {
						result.Vertices = appendUniqueFace(result.Vertices, v)
					}
				}
				if len(result.Vertices) != 1 {
					return empty, geom.ErrConvergence
				}
			}
		}
	}
	if station == 0 || station == len(proof.Heights)-1 {
		index := 0
		if station != 0 {
			index = 1
		}
		result.Location = parametric.Boundary
		result.Faces = appendUniqueFace(result.Faces, proof.History.CapFaces[index])
	}
	for _, ids := range [][]int{result.Faces, result.Edges, result.RimEdges, result.Vertices} {
		sort.Ints(ids)
	}
	return finish()
}
