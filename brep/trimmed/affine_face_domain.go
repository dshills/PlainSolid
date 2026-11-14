package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type faceDomainBudget struct {
	limit, depth, used int
	enabled            bool
}

func newFaceDomainBudget(limit, depth int) (*faceDomainBudget, error) {
	if limit < 0 || depth < 0 || limit > 1000000 || depth > 64 {
		return nil, geom.ErrInvalid
	}
	enabled := limit != 0 || depth != 0
	if enabled {
		if limit == 0 {
			limit = 1024
		}
		if depth == 0 {
			depth = 16
		}
	}
	return &faceDomainBudget{limit: limit, depth: depth, enabled: enabled}, nil
}

// DomainSubdivisions reports the exact support-containment subdivisions used
// at construction. It is independent of trim topology/area refinement work.
func (q *AffineSplineFaceQuery) DomainSubdivisions() int {
	if q == nil {
		return 0
	}
	return q.domainSubdivisions
}

func (b *faceDomainBudget) prove(ctx context.Context, h [][3]*big.Rat, bounds [2][2]*big.Rat, bits int) error {
	type cell struct {
		h     [][3]*big.Rat
		depth int
	}
	stack := []cell{{h: h}}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return e
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(current.h) < 2 || len(current.h) > 5 {
			return geom.ErrInvalid
		}
		inside := true
		for i, p := range current.h {
			for _, v := range p {
				if v == nil {
					return geom.ErrInvalid
				}
				if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
					return geom.ErrConvergence
				}
			}
			if p[2].Sign() <= 0 {
				return geom.ErrInvalid
			}
			for axis := 0; axis < 2; axis++ {
				for side := 0; side < 2; side++ {
					v := new(big.Rat).Mul(bounds[axis][side], p[2])
					if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
						return geom.ErrConvergence
					}
					cmp := p[axis].Cmp(v)
					if side == 0 && cmp < 0 || side == 1 && cmp > 0 {
						inside = false
						// A Bézier endpoint is an actual curve point, including exact dyadic
						// split points. An outside control alone is never an exit proof.
						if i == 0 || i == len(current.h)-1 || !b.enabled {
							return geom.ErrDomain
						}
					}
				}
			}
		}
		if inside {
			continue
		}
		if current.depth >= b.depth || b.used >= b.limit {
			return geom.ErrConvergence
		}
		b.used++
		lo, hi := splitHomogeneousUV(current.h, big.NewRat(1, 2))
		stack = append(stack, cell{hi, current.depth + 1}, cell{lo, current.depth + 1})
	}
	return nil
}
