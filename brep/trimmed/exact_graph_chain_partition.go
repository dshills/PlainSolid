//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// ExactGraphChainPartitionOptions bounds the aggregate RAW input parameter
// count (both owners, all pieces), and therefore the union count. Defaults are
// 16384 parameters and4096 coefficient bits; hard maxima are65536 and65536.
type ExactGraphChainPartitionOptions struct{ MaxParameters, MaxCoefficientBits int }

// ExactGraphPiecePartition owns its exact normalized parameter union. OriginalA
// and OriginalB map each original vertex occurrence to the union index. Separate
// piece endpoints remain separate occurrences: this planner does not weld them.
type ExactGraphPiecePartition struct {
	Parameters           []*big.Rat
	OriginalA, OriginalB []int
}

// ExactGraphChainPartition is a proposed source-parameter refinement only. It
// proves no curve agreement, topology, chord error or face coverage. Consumers
// must establish source identity and recertify all new chords and fills.
type ExactGraphChainPartition struct {
	Pieces                      []ExactGraphPiecePartition
	InputParameters, Parameters int
}

// PlanExactGraphChainPartition merges corresponding exact normalized ordered
// partitions for1..64 pieces. Every partition starts at0, ends at1 and is strictly
// increasing. All rationals are copied; errors return no partial plan.
func PlanExactGraphChainPartition(ctx context.Context, a, b [][]*big.Rat, o ExactGraphChainPartitionOptions) (ExactGraphChainPartition, error) {
	var empty ExactGraphChainPartition
	if ctx == nil || len(a) == 0 || len(a) > 64 || len(a) != len(b) {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxParameters == 0 {
		o.MaxParameters = 16384
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxParameters < 1 || o.MaxParameters > 65536 || o.MaxCoefficientBits < 1 || o.MaxCoefficientBits > 65536 {
		return empty, geom.ErrInvalid
	}
	count := 0
	for i := range a {
		for _, p := range [][]*big.Rat{a[i], b[i]} {
			if len(p) < 2 {
				return empty, geom.ErrInvalid
			}
			if len(p) > o.MaxParameters-count {
				return empty, geom.ErrConvergence
			}
			count += len(p)
		}
	}
	for i := range a {
		for _, p := range [][]*big.Rat{a[i], b[i]} {
			for j, t := range p {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				if t == nil {
					return empty, geom.ErrInvalid
				}
				if t.Num().BitLen() > o.MaxCoefficientBits || t.Denom().BitLen() > o.MaxCoefficientBits {
					return empty, geom.ErrConvergence
				}
				if j == 0 && t.Sign() != 0 || j == len(p)-1 && t.Cmp(big.NewRat(1, 1)) != 0 || j > 0 && t.Cmp(p[j-1]) <= 0 {
					return empty, geom.ErrInvalid
				}
			}
		}
	}
	r := ExactGraphChainPartition{InputParameters: count, Pieces: make([]ExactGraphPiecePartition, len(a))}
	for k := range a {
		p := ExactGraphPiecePartition{OriginalA: make([]int, len(a[k])), OriginalB: make([]int, len(b[k]))}
		i, j := 0, 0
		for i < len(a[k]) || j < len(b[k]) {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			index := len(p.Parameters)
			takeA, takeB := false, false
			if i == len(a[k]) {
				takeB = true
			} else if j == len(b[k]) {
				takeA = true
			} else {
				c := a[k][i].Cmp(b[k][j])
				takeA = c <= 0
				takeB = c >= 0
			}
			var q *big.Rat
			if takeA {
				q = a[k][i]
				p.OriginalA[i] = index
				i++
			}
			if takeB {
				q = b[k][j]
				p.OriginalB[j] = index
				j++
			}
			p.Parameters = append(p.Parameters, new(big.Rat).Set(q))
		}
		r.Parameters += len(p.Parameters)
		r.Pieces[k] = p
	}
	return r, nil
}
