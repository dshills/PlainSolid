package geom

import (
	"context"
	"math/big"
	"sort"
)

// PiecewiseParabolaOptions bounds the complete union of original knot slabs.
// MaxPieces defaults to 8 and cannot exceed 32. Proof quotas are cumulative,
// including restriction, full slab proofs and exact one-sided join evaluation.
// MaxCells cumulatively counts difference cells and injectivity cell visits.
type PiecewiseParabolaOptions struct {
	Proof               GraphPointContactOptions
	FreeAxis, MaxPieces int
}

// ParabolaKnotJoin retains a shared original UV/model point and separate
// one-sided model tangents in increasing original free-coordinate units.
// Tangents need not agree across a C0 knot. Both owners independently agree
// with each listed side. No smooth continuation across the knot is asserted.
type ParabolaKnotJoin struct {
	UV       [2]*big.Rat
	Point    [3]*big.Rat
	Tangents [2][3]*big.Rat
}

// PiecewiseSplineGraphParabola is a complete contact report, not a native
// edge recipe. Each piece retains its original domain and knot-cell evidence.
// Internal terminal masks denote slab edges, not original outer boundaries.
// PositiveSide is the common sign toward increasing dependent coordinate.
type PiecewiseSplineGraphParabola struct {
	Proven                 bool
	Domain                 ParameterRectangle
	FreeAxis, PositiveSide int
	Pieces                 []SplineGraphParabola
	Joins                  []ParabolaKnotJoin
	ExactOperations        int
}

// DiscoverPiecewiseSplineGraphParabola proves a continuous open contact whose
// quadratic formula may change at original knots. Every original free-axis
// knot is retained, including redundant refinement knots. At least two slabs
// are required. All slabs cover the entire dependent range, have strict
// Bernstein boundary clearance, and independently prove all original cells.
// Failed, canceled or exhausted discovery publishes no partial contact.
func DiscoverPiecewiseSplineGraphParabola(ctx context.Context, a, b *SurfacePatch, o PiecewiseParabolaOptions) (r PiecewiseSplineGraphParabola, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = PiecewiseSplineGraphParabola{}
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
		piece, e := DiscoverSplineGraphParabolaOnAxis(ctx, pair[0], pair[1], o.FreeAxis, proof)
		used += piece.ExactOperations
		if e != nil {
			return r, e
		}
		cells += len(piece.CertifiedCells) + piece.Graph.VisitedCells
		if cells > p.MaxCells {
			return r, ErrConvergence
		}
		if i > 0 && piece.PositiveSide != r.Pieces[0].PositiveSide {
			return r, ErrConvergence
		}
		r.Pieces = append(r.Pieces, piece)
		owners = append(owners, pair)
	}
	budget := &polynomialBudget{maxOperations: p.MaxOperations, maxBits: p.MaxCoefficientBits, operations: used}
	defer func() { used = budget.operations }()
	for i := 1; i < len(r.Pieces); i++ {
		uv := r.Pieces[i-1].Controls[2]
		next := r.Pieces[i].Controls[0]
		for axis := 0; axis < 2; axis++ {
			if uv[axis].Cmp(next[axis]) != 0 {
				return r, ErrConvergence
			}
		}
		join := ParabolaKnotJoin{UV: [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}}
		for side := 0; side < 2; side++ {
			piece := r.Pieces[i-1+side]
			slope := new(big.Rat).Add(piece.Power[1], new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(piece.Power[2], uv[o.FreeAxis])))
			if e := budget.check(slope); e != nil {
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
	r.PositiveSide = r.Pieces[0].PositiveSide
	return r, nil
}
