package geom

import (
	"context"
	"math/big"
	"sort"
)

// Shared internal collector: source restriction, cumulative quotas and exact
// four-occurrence knot witnesses are independent of the selected graph proof.
type graphPieceDescription struct {
	power             []*big.Rat
	controls          [][2]*big.Rat
	sign, cells, work int
}
type piecewiseGraphProof[T any] struct {
	Proven                 bool
	Domain                 ParameterRectangle
	FreeAxis, PositiveSide int
	Pieces                 []T
	Joins                  []ParabolaKnotJoin
	ExactOperations        int
}

func discoverPiecewiseGraph[T any](ctx context.Context, a, b *SurfacePatch, o PiecewiseParabolaOptions, discover func(context.Context, *SurfacePatch, *SurfacePatch, int, GraphPointContactOptions) (T, error), describe func(T) graphPieceDescription) (r piecewiseGraphProof[T], err error) {
	used := 0
	defer func() {
		if err != nil {
			r = piecewiseGraphProof[T]{}
		}
		r.ExactOperations = used
	}()
	if ctx == nil || a == nil || b == nil || o.FreeAxis < 0 || o.FreeAxis > 1 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 8
	}
	if o.MaxPieces < 2 || o.MaxPieces > 32 {
		return r, ErrInvalid
	}
	p := o.Proof
	if p.MaxOperations == 0 {
		p.MaxOperations = 8000000
	}
	if p.MaxCoefficientBits == 0 {
		p.MaxCoefficientBits = 16384
	}
	if p.MaxCells == 0 {
		p.MaxCells = 4096
	}
	if p.MaxOperations < 1 || p.MaxCoefficientBits < 64 || p.MaxCells < 1 || p.MaxDepth < 0 {
		return r, ErrInvalid
	}
	if a.Domain() != b.Domain() {
		return r, ErrDomain
	}
	var sources [2]*SurfacePatch
	for i, source := range []*SurfacePatch{a, b} {
		if used >= p.MaxOperations {
			return r, ErrConvergence
		}
		q, e := source.WithLimits(SurfacePatchOptions{MaxOperations: p.MaxOperations - used, MaxCoefficientBits: p.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		used += q.ExactOperations()
		sources[i] = q
	}
	a, b = sources[0], sources[1]
	d := a.Domain()
	lo, hi := d.UMin, d.UMax
	ak, bk := a.knotsU, b.knotsU
	if o.FreeAxis == 1 {
		lo, hi = d.VMin, d.VMax
		ak, bk = a.knotsV, b.knotsV
	}
	stations := []float64{lo, hi}
	seen := map[float64]bool{lo: true, hi: true}
	for _, knots := range [][]float64{ak, bk} {
		for _, k := range knots {
			if k > lo && k < hi && !seen[k] {
				seen[k] = true
				stations = append(stations, k)
				if len(stations)-1 > o.MaxPieces {
					return r, ErrConvergence
				}
			}
		}
	}
	sort.Float64s(stations)
	if len(stations) < 3 {
		return r, ErrConvergence
	}
	var owners [][2]*SurfacePatch
	cells := 0
	for i := 0; i+1 < len(stations); i++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		slab := d
		if o.FreeAxis == 0 {
			slab.UMin, slab.UMax = stations[i], stations[i+1]
		} else {
			slab.VMin, slab.VMax = stations[i], stations[i+1]
		}
		var pair [2]*SurfacePatch
		for j, source := range []*SurfacePatch{a, b} {
			remaining := p.MaxOperations - used
			if remaining < 1 {
				return r, ErrConvergence
			}
			var work int
			pair[j], work, err = source.TrimWithWorkLimit(slab, remaining)
			used += work
			if err != nil {
				return r, err
			}
		}
		proof := p
		proof.MaxOperations -= used
		proof.MaxCells -= cells
		if proof.MaxOperations < 1 || proof.MaxCells < 1 {
			return r, ErrConvergence
		}
		piece, e := discover(ctx, pair[0], pair[1], o.FreeAxis, proof)
		description := describe(piece)
		used += description.work
		if e != nil {
			return r, e
		}
		cells += description.cells
		if cells > p.MaxCells {
			return r, ErrConvergence
		}
		if i > 0 && description.sign != describe(r.Pieces[0]).sign {
			return r, ErrConvergence
		}
		r.Pieces = append(r.Pieces, piece)
		owners = append(owners, pair)
	}
	budget := &polynomialBudget{maxOperations: p.MaxOperations, maxBits: p.MaxCoefficientBits, operations: used}
	defer func() { used = budget.operations }()
	for i := 1; i < len(r.Pieces); i++ {
		previous, nextPiece := describe(r.Pieces[i-1]), describe(r.Pieces[i])
		uv := previous.controls[len(previous.controls)-1]
		next := nextPiece.controls[0]
		for axis := 0; axis < 2; axis++ {
			if uv[axis].Cmp(next[axis]) != 0 {
				return r, ErrConvergence
			}
		}
		join := ParabolaKnotJoin{UV: [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}}
		for side := 0; side < 2; side++ {
			piece := describe(r.Pieces[i-1+side])
			derivative := polynomialDerivative(piece.power)
			for _, x := range derivative {
				if e := budget.check(x); e != nil {
					return r, e
				}
			}
			slope, e := polynomialValue(derivative, uv[o.FreeAxis], budget)
			if e != nil {
				return r, e
			}
			for owner := 0; owner < 2; owner++ {
				remaining := p.MaxOperations - budget.operations
				if remaining < 1 {
					return r, ErrConvergence
				}
				jet, e := owners[i-1+side][owner].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: p.MaxCoefficientBits})
				budget.operations += jet.ExactOperations
				if e != nil {
					return r, e
				}
				for axis := 0; axis < 3; axis++ {
					if side == 0 && owner == 0 {
						join.Point[axis] = new(big.Rat).Set(jet.Point[axis])
					} else if join.Point[axis].Cmp(jet.Point[axis]) != 0 {
						return r, ErrConvergence
					}
					free, dependent := jet.Du[axis], jet.Dv[axis]
					if o.FreeAxis == 1 {
						free, dependent = dependent, free
					}
					tangent := new(big.Rat).Add(free, new(big.Rat).Mul(dependent, slope))
					if e := budget.check(tangent); e != nil {
						return r, e
					}
					if owner == 0 {
						join.Tangents[side][axis] = tangent
					} else if join.Tangents[side][axis].Cmp(tangent) != 0 {
						return r, ErrConvergence
					}
				}
			}
		}
		r.Joins = append(r.Joins, join)
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	r.Proven = true
	r.Domain = d
	r.FreeAxis = o.FreeAxis
	r.PositiveSide = describe(r.Pieces[0]).sign
	return r, nil
}
