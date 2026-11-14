package geom

import (
	"context"
	"math/big"
)

type projectedExactRectangle [2][2]*big.Rat

// CertifyCommonXYGraphCellInTubes proves complete contact coverage for original
// rational identity-XY graphs without assuming radial or factored heights.
// Supports must be single positive-weight Bezier patches of degree<=8 per axis.
// Every target is freshly certified on the original pair. Exact projected
// vertices, open edges and open rectangles cover the closed candidate; an
// uncovered stratum is excluded only by an exact strict relative-interior
// Bernstein sign. Mixed-sign/zero residuals remain unresolved. This proves
// coverage only, not connectivity, embedding or branch enumeration.
func CertifyCommonXYGraphCellInTubes(ctx context.Context, a, c *SurfacePatch, candidate SurfacePairParameterBox, targets []SurfacePairTubeRegion, o SurfaceTubeUnionCoverageOptions) (out SurfaceTubeUnionCoverage, err error) {
	if ctx == nil || a == nil || c == nil {
		return out, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 8192
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.MaxPieces < 1 || o.MaxPieces > 1048576 || o.MaxCells < 1 || o.MaxCells > 1048576 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return out, ErrInvalid
	}
	if len(targets) > 512 {
		return out, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil && !out.Covered && len(out.Unresolved) == 0 {
			out.Pending = []SurfacePairParameterBox{candidate}
		}
	}()
	for _, p := range []*SurfacePatch{a, c} {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if p.degreeU > 8 || p.degreeV > 8 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return out, ErrConvergence
		}
		_, inv, e := normalizeAffineXYGraph(ctx, p, b)
		if e != nil {
			return out, e
		}
		for i, row := range inv {
			for j, q := range row {
				want := 0
				if i == j {
					want = 1
				}
				if q.Cmp(big.NewRat(int64(want), 1)) != 0 {
					return out, ErrConvergence
				}
			}
		}
	}
	project := func(pair SurfacePairParameterBox) (projectedExactRectangle, bool, error) {
		var out projectedExactRectangle
		for i, item := range []struct {
			p *SurfacePatch
			d ParameterRectangle
		}{{a, pair.A}, {c, pair.B}} {
			d, full := item.d, item.p.Domain()
			if !d.Valid() || d.UMin < full.UMin || d.UMax > full.UMax || d.VMin < full.VMin || d.VMax > full.VMax {
				return out, false, ErrDomain
			}
			for axis, range2 := range [][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
				lo, hi := new(big.Rat).SetFloat64(range2[0]), new(big.Rat).SetFloat64(range2[1])
				if e := b.check(lo); e != nil {
					return out, false, e
				}
				if e := b.check(hi); e != nil {
					return out, false, e
				}
				if i == 0 {
					out[axis] = [2]*big.Rat{lo, hi}
				} else {
					if lo.Cmp(out[axis][0]) > 0 {
						out[axis][0] = lo
					}
					if hi.Cmp(out[axis][1]) < 0 {
						out[axis][1] = hi
					}
				}
			}
		}
		return out, out[0][0].Cmp(out[0][1]) <= 0 && out[1][0].Cmp(out[1][1]) <= 0, nil
	}
	box, present, e := project(candidate)
	if e != nil {
		return out, e
	}
	var rectangles []projectedExactRectangle
	for _, target := range targets {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if target.Boundary != nil {
			return out, ErrConvergence
		}
		rect, nonempty, e := project(target.Box)
		if e != nil {
			return out, e
		}
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return out, ErrConvergence
		}
		// The exact target-as-candidate union contains no leftover pieces. This
		// reuses authentic original tube proof, never public diagnostic flags.
		proof, e := CertifySurfacePairCellInTubesContext(ctx, a, c, target.Box, []SurfacePairTubeRegion{target}, SurfaceTubeUnionCoverageOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxCells: o.MaxCells, MaxPieces: o.MaxPieces})
		b.operations += proof.ExactOperations
		if e != nil {
			return out, e
		}
		if !proof.Covered {
			return out, ErrConvergence
		}
		out.Tubes = append(out.Tubes, proof.Tubes...)
		if nonempty {
			rectangles = append(rectangles, rect)
		}
	}
	if !present {
		out.Covered = true
		out.OutsideExcluded = []SurfacePairParameterBox{candidate}
		return out, nil
	}
	covered, e := projectedRectangleStrataCovered(ctx, box, rectangles, o, b, &out, func(d projectedExactRectangle) (bool, error) { return commonGraphStratumNonzero(ctx, a, c, d, b) })
	if e != nil {
		return out, e
	}
	if !covered {
		out.Unresolved = []SurfacePairParameterBox{candidate}
		return out, ErrConvergence
	}
	out.Covered = true
	return out, nil
}

func commonGraphStratumNonzero(ctx context.Context, a, c *SurfacePatch, rect projectedExactRectangle, b *polynomialBudget) (bool, error) {
	var patches [2]*SurfacePatch
	for k, p := range []*SurfacePatch{a, c} {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		d := p.Domain()
		ends := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		var local [2][2]*big.Rat
		for axis := range 2 {
			origin := new(big.Rat).SetFloat64(ends[axis][0])
			width := new(big.Rat).Sub(new(big.Rat).SetFloat64(ends[axis][1]), origin)
			if e := b.check(width); e != nil {
				return false, e
			}
			for j := range 2 {
				q := new(big.Rat).Sub(rect[axis][j], origin)
				if e := b.check(q); e != nil {
					return false, e
				}
				q.Quo(q, width)
				if e := b.check(q); e != nil {
					return false, e
				}
				local[axis][j] = q
			}
		}
		net, e := restrictComparisonTensor(comparisonTensor{net: p.net, nu: p.countU, nv: p.countV}, local[0][0], local[0][1], local[1][0], local[1][1], b)
		if e != nil {
			return false, e
		}
		q := *p
		q.net = net
		patches[k] = &q
	}
	// Equal-degree polynomial heights need no degree-doubling homogeneous
	// product. Prove both weight nets constant before this exact reduction.
	if patches[0].degreeU == patches[1].degreeU && patches[0].degreeV == patches[1].degreeV {
		constant := true
		for _, p := range patches {
			for _, h := range p.net {
				if e := b.check(h[3]); e != nil {
					return false, e
				}
				if h[3].Cmp(p.net[0][3]) != 0 {
					constant = false
				}
			}
		}
		if constant {
			sign := 0
			for i, h := range patches[0].net {
				x := new(big.Rat).Quo(h[2], h[3])
				if e := b.check(x); e != nil {
					return false, e
				}
				z := patches[1].net[i]
				y := new(big.Rat).Quo(z[2], z[3])
				if e := b.check(y); e != nil {
					return false, e
				}
				x.Sub(x, y)
				if e := b.check(x); e != nil {
					return false, e
				}
				if x.Sign() != 0 {
					if sign != 0 && sign != x.Sign() {
						return false, nil
					}
					sign = x.Sign()
				}
			}
			return sign != 0, nil
		}
	}
	left, e := tensorHomogeneousBernsteinProduct(patches[0], 2, patches[1], 3, b)
	if e != nil {
		return false, e
	}
	right, e := tensorHomogeneousBernsteinProduct(patches[0], 3, patches[1], 2, b)
	if e != nil {
		return false, e
	}
	sign := 0
	for i, q := range left {
		q.Sub(q, right[i])
		if e := b.check(q); e != nil {
			return false, e
		}
		s := q.Sign()
		if s != 0 {
			if sign != 0 && sign != s {
				return false, nil
			}
			sign = s
		}
	}
	// On every relative interior all retained Bernstein basis functions are
	// positive. Collapsed axes were evaluated exactly, so the same rule covers
	// points and open edges. Identically zero never proves exclusion.
	return sign != 0, nil
}

func projectedRectangleStrataCovered(ctx context.Context, box projectedExactRectangle, targets []projectedExactRectangle, o SurfaceTubeUnionCoverageOptions, b *polynomialBudget, out *SurfaceTubeUnionCoverage, exclude func(projectedExactRectangle) (bool, error)) (bool, error) {
	var cuts [2][]*big.Rat
	for axis := 0; axis < 2; axis++ {
		insert := func(q *big.Rat) error {
			if q.Cmp(box[axis][0]) < 0 || q.Cmp(box[axis][1]) > 0 {
				return nil
			}
			for i, x := range cuts[axis] {
				if e := b.check(q); e != nil {
					return e
				}
				cmp := q.Cmp(x)
				if cmp == 0 {
					return nil
				}
				if cmp < 0 {
					cuts[axis] = append(cuts[axis], nil)
					copy(cuts[axis][i+1:], cuts[axis][i:])
					cuts[axis][i] = q
					return nil
				}
			}
			cuts[axis] = append(cuts[axis], q)
			return nil
		}
		if e := insert(box[axis][0]); e != nil {
			return false, e
		}
		if e := insert(box[axis][1]); e != nil {
			return false, e
		}
		for _, target := range targets {
			for _, q := range target[axis] {
				if e := insert(q); e != nil {
					return false, e
				}
			}
		}
	}
	nx, ny := 2*len(cuts[0])-1, 2*len(cuts[1])-1
	if nx > o.MaxPieces/ny {
		return false, ErrConvergence
	}
	out.PartitionPieces = nx * ny
	bounds := func(axis, index int) (*big.Rat, *big.Rat, *big.Rat) {
		lo := cuts[axis][index/2]
		hi := lo
		if index%2 != 0 {
			hi = cuts[axis][index/2+1]
		}
		return lo, hi, new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
	}
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			x0, x1, x := bounds(0, i)
			y0, y1, y := bounds(1, j)
			if e := b.check(x); e != nil {
				return false, e
			}
			if e := b.check(y); e != nil {
				return false, e
			}
			covered := false
			for _, target := range targets {
				if e := b.check(x); e != nil {
					return false, e
				}
				if x.Cmp(target[0][0]) >= 0 && x.Cmp(target[0][1]) <= 0 && y.Cmp(target[1][0]) >= 0 && y.Cmp(target[1][1]) <= 0 {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			if out.VisitedCells >= o.MaxCells {
				return false, ErrConvergence
			}
			out.VisitedCells++
			nonzero, e := exclude(projectedExactRectangle{{x0, x1}, {y0, y1}})
			if e != nil {
				return false, e
			}
			if !nonzero {
				return false, nil
			}
		}
	}
	return true, nil
}
