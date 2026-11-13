package geom

import (
	"context"
	"math/big"
)

// SaddleSectorBoundary traverses saddle -> terminal StartRay -> rectangle
// boundary in ParameterDirection (+1 CCW UV, -1 CW UV) -> terminal EndRay ->
// saddle. Positive/negative refer to model XY orientation, not height sign.
type SaddleSectorBoundary struct{ StartRay, EndRay, ParameterDirection int }
type GraphSaddleSector struct {
	HeightSide                         int
	Witness                            [2]*big.Rat
	PositiveBoundary, NegativeBoundary SaddleSectorBoundary
}

// QuadraticSaddleSectors orders the four sectors positively in model XY.
// Private proof data make ClassifyRational independent of caller edits to the
// display graph, sectors, or input rational values.
type QuadraticSaddleSectors struct {
	Complete        bool
	Section         QuadraticGraphSection
	RayCycle        [4]int
	Sectors         [4]GraphSaddleSector
	ExactOperations int
	proof           *saddleSectorProof
}
type saddleSectorProof struct {
	center      [2]*big.Rat
	rectangle   [2][2]*big.Rat
	h           [2][2]*big.Rat
	cycle       [4]int
	orientation int
	vertical    bool
}

// SaddleSectorPoint identifies one open sector, a contact ray, or the saddle.
// Sector/Ray are -1 when inapplicable. Boundary means on a rectangle side;
// it does not mean the query lies on the zero-height contact set.
type SaddleSectorPoint struct {
	Outside, Boundary, Saddle                bool
	Sector, Ray, HeightSide, ExactOperations int
}

// CertifyQuadraticGraphSaddleSectors builds exact oriented sector boundaries
// from the clipped quadratic graph. Rational interior witnesses prove each
// sector's height sign; the factorization makes that sign constant throughout.
func CertifyQuadraticGraphSaddleSectors(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, rectangle [2][2]*big.Rat, o GraphPointContactOptions) (r QuadraticSaddleSectors, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	section, e := ClipQuadraticGraphSaddleSection(ctx, a, c, uv, rectangle, o)
	r.Section = section
	r.ExactOperations = section.ExactOperations
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations - section.ExactOperations, maxBits: o.MaxCoefficientBits}
	if b.maxOperations < 1 {
		return r, ErrConvergence
	}
	defer func() { r.ExactOperations = section.ExactOperations + b.operations }()
	p := &saddleSectorProof{orientation: section.Certificate.ChartOrientation}
	for i := 0; i < 2; i++ {
		p.center[i] = new(big.Rat).Set(uv[i])
		for j := 0; j < 2; j++ {
			p.rectangle[i][j] = new(big.Rat).Set(rectangle[i][j])
			p.h[i][j] = new(big.Rat).Set(section.Certificate.Critical.Hessian[i][j])
		}
	}
	p.vertical = p.h[1][1].Sign() == 0
	canonical := [4]int{1, 3, 0, 2}
	var directions [4][2]*big.Rat
	if !p.vertical {
		mid := new(big.Rat).Quo(new(big.Rat).Add(section.Certificate.Rays[0].SlopeUpper, section.Certificate.Rays[2].SlopeLower), big.NewRat(2, 1))
		directions = [4][2]*big.Rat{{big.NewRat(1, 1), mid}, {new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 1), new(big.Rat).Neg(mid)}, {new(big.Rat), big.NewRat(-1, 1)}}
	} else {
		canonical = [4]int{3, 1, 2, 0}
		k := section.Certificate.Rays[2].SlopeLower
		directions = [4][2]*big.Rat{{big.NewRat(1, 1), new(big.Rat).Add(k, big.NewRat(1, 1))}, {big.NewRat(-1, 1), new(big.Rat).Sub(big.NewRat(1, 1), k)}, {big.NewRat(-1, 1), new(big.Rat).Sub(new(big.Rat).Neg(k), big.NewRat(1, 1))}, {big.NewRat(1, 1), new(big.Rat).Sub(k, big.NewRat(1, 1))}}
	}
	p.cycle = canonical
	r.RayCycle = canonical
	if p.orientation < 0 {
		r.RayCycle = [4]int{canonical[0], canonical[3], canonical[2], canonical[1]}
	}
	for index := 0; index < 4; index++ {
		canonicalSector := index
		if p.orientation < 0 {
			canonicalSector = 3 - index
		}
		v := directions[canonicalSector]
		var scale *big.Rat
		for axis := 0; axis < 2; axis++ {
			left := new(big.Rat).Sub(uv[axis], rectangle[axis][0])
			right := new(big.Rat).Sub(rectangle[axis][1], uv[axis])
			if right.Cmp(left) < 0 {
				left = right
			}
			candidate := new(big.Rat).Quo(left, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Abs(v[axis]))))
			if scale == nil || candidate.Cmp(scale) < 0 {
				scale = candidate
			}
		}
		sector := GraphSaddleSector{}
		for axis := 0; axis < 2; axis++ {
			sector.Witness[axis] = new(big.Rat).Add(uv[axis], new(big.Rat).Mul(scale, v[axis]))
			if e := b.check(sector.Witness[axis]); e != nil {
				return r, e
			}
		}
		sign, e := saddleFormSign(p, v, b)
		if e != nil {
			return r, e
		}
		if sign == 0 {
			return r, ErrConvergence
		}
		sector.HeightSide = sign
		start, end := r.RayCycle[index], r.RayCycle[(index+1)%4]
		sector.PositiveBoundary = SaddleSectorBoundary{start, end, p.orientation}
		sector.NegativeBoundary = SaddleSectorBoundary{end, start, -p.orientation}
		r.Sectors[index] = sector
	}
	r.proof = p
	r.Complete = true
	return r, nil
}
func saddleFormSign(p *saddleSectorProof, v [2]*big.Rat, b *polynomialBudget) (int, error) {
	x := new(big.Rat).Mul(p.h[0][0], new(big.Rat).Mul(v[0], v[0]))
	x.Add(x, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(p.h[0][1], new(big.Rat).Mul(v[0], v[1]))))
	x.Add(x, new(big.Rat).Mul(p.h[1][1], new(big.Rat).Mul(v[1], v[1])))
	if e := b.check(x); e != nil {
		return 0, e
	}
	return x.Sign(), nil
}

// ClassifyRational classifies a rational original-UV point against the immutable
// certified sector proof. No slope approximation or rounded angle is used.
func (r QuadraticSaddleSectors) ClassifyRational(ctx context.Context, query [2]*big.Rat, o SurfacePatchEvaluationOptions) (result SaddleSectorPoint, err error) {
	result.Sector, result.Ray = -1, -1
	if ctx == nil || r.proof == nil || query[0] == nil || query[1] == nil {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 10000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	p := r.proof
	var v [2]*big.Rat
	for axis, x := range query {
		if e := b.check(x); e != nil {
			return result, e
		}
		if x.Cmp(p.rectangle[axis][0]) < 0 || x.Cmp(p.rectangle[axis][1]) > 0 {
			result.Outside = true
			return result, nil
		}
		result.Boundary = result.Boundary || x.Cmp(p.rectangle[axis][0]) == 0 || x.Cmp(p.rectangle[axis][1]) == 0
		v[axis] = new(big.Rat).Sub(x, p.center[axis])
	}
	if v[0].Sign() == 0 && v[1].Sign() == 0 {
		result.Saddle = true
		return result, nil
	}
	sign, e := saddleFormSign(p, v, b)
	if e != nil {
		return result, e
	}
	result.HeightSide = sign
	sector := -1
	rayPosition := -1
	if p.vertical {
		if v[0].Sign() == 0 {
			rayPosition = 1
			if v[1].Sign() < 0 {
				rayPosition = 3
			}
		} else {
			slope := new(big.Rat).Quo(v[1], v[0])
			k := new(big.Rat).Quo(new(big.Rat).Neg(p.h[0][0]), new(big.Rat).Mul(big.NewRat(2, 1), p.h[0][1]))
			comparison := slope.Cmp(k)
			if comparison == 0 {
				rayPosition = 0
				if v[0].Sign() < 0 {
					rayPosition = 2
				}
			} else if v[0].Sign() > 0 {
				sector = 3
				if comparison > 0 {
					sector = 0
				}
			} else {
				sector = 1
				if comparison > 0 {
					sector = 2
				}
			}
		}
	} else if v[0].Sign() == 0 {
		sector = 1
		if v[1].Sign() < 0 {
			sector = 3
		}
	} else {
		slope := new(big.Rat).Quo(v[1], v[0])
		vertex := new(big.Rat).Quo(new(big.Rat).Neg(p.h[0][1]), p.h[1][1])
		above := slope.Cmp(vertex) > 0
		if sign == 0 {
			rayPosition = 0
			if above {
				rayPosition = 1
			}
			if v[0].Sign() < 0 {
				rayPosition += 2
			}
		} else if sign*p.h[1][1].Sign() < 0 {
			sector = 0
			if v[0].Sign() < 0 {
				sector = 2
			}
		} else {
			sector = 3
			if above == (v[0].Sign() > 0) {
				sector = 1
			}
		}
	}
	if rayPosition >= 0 {
		result.Ray = p.cycle[rayPosition]
	} else {
		if p.orientation < 0 {
			sector = 3 - sector
		}
		result.Sector = sector
	}
	return result, nil
}
