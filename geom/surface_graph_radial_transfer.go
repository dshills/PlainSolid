package geom

import (
	"context"
	"math/big"
	"runtime"
)

type radialExactRectangle [2][2]*big.Rat

// CertifyCellInTubes transfers contact coverage using the complete physical
// radial identity. Every target uniform tube is freshly proved on the original
// supports. Exact affine inverses map its closed original domains to physical
// rectangles; intersecting both rectangles retains precisely its possible
// contact points. A bounded arrangement includes vertices and open edges/cells,
// so an uncovered tangent point cannot disappear into a neighboring open cell.
// No rounded boxes or sampled curve points establish coverage. Diagnostics are
// in original candidate coordinates; PartitionPieces counts all strata.
func (r *AffineRadialContours) CertifyCellInTubes(ctx context.Context, candidate SurfacePairParameterBox, targets []SurfacePairTubeRegion, o SurfaceTubeUnionCoverageOptions) (out SurfaceTubeUnionCoverage, err error) {
	if ctx == nil || r == nil || r.radial == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 8192
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPieces < 1 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return out, ErrInvalid
	}
	if len(targets) > 512 {
		return out, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		out.ExactOperations = budget.operations
		if err != nil && !out.Covered && len(out.Unresolved) == 0 {
			out.Pending = []SurfacePairParameterBox{candidate}
		}
	}()
	mapPair := func(box SurfacePairParameterBox) (radialExactRectangle, bool, error) {
		var both [2]radialExactRectangle
		for owner, d := range []ParameterRectangle{box.A, box.B} {
			full := r.original[owner].Domain()
			if !d.Valid() || d.UMin < full.UMin || d.UMax > full.UMax || d.VMin < full.VMin || d.VMax > full.VMax {
				return radialExactRectangle{}, false, ErrDomain
			}
			uv := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
			for i, row := range r.inverse[owner] {
				axis := 0
				if row[1].Sign() != 0 {
					axis = 1
				}
				lo := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(uv[i][0]), row[2]), row[axis])
				hi := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(uv[i][1]), row[2]), row[axis])
				runtime.KeepAlive(&row) // golang/go#82026: keeps row in the stack map. Remove once the Go fix is in use.
				if lo.Cmp(hi) > 0 {
					lo, hi = hi, lo
				}
				if e := budget.check(lo); e != nil {
					return radialExactRectangle{}, false, e
				}
				if e := budget.check(hi); e != nil {
					return radialExactRectangle{}, false, e
				}
				both[owner][axis] = [2]*big.Rat{lo, hi}
			}
		}
		var result radialExactRectangle
		for axis := 0; axis < 2; axis++ {
			lo, hi := both[0][axis][0], both[0][axis][1]
			if both[1][axis][0].Cmp(lo) > 0 {
				lo = both[1][axis][0]
			}
			if both[1][axis][1].Cmp(hi) < 0 {
				hi = both[1][axis][1]
			}
			if lo.Cmp(hi) > 0 {
				return result, false, nil
			}
			result[axis] = [2]*big.Rat{lo, hi}
		}
		return result, true, nil
	}
	box, nonempty, e := mapPair(candidate)
	if e != nil {
		return out, e
	}
	var rectangles []radialExactRectangle
	for _, target := range targets {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if target.FreeAxis < 0 || target.FreeAxis > 3 || target.Boundary != nil {
			return out, ErrInvalid
		}
		rect, present, e := mapPair(target.Box)
		if e != nil {
			return out, e
		}
		pa, e := trimPatchBudget(r.original[0], target.Box.A, budget)
		if e != nil {
			return out, e
		}
		pb, e := trimPatchBudget(r.original[1], target.Box.B, budget)
		if e != nil {
			return out, e
		}
		remaining := budget.maxOperations - budget.operations
		if remaining < 1 {
			return out, ErrConvergence
		}
		tube, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: target.FreeAxis, MaxOperations: remaining, MaxCoefficientBits: budget.maxBits})
		budget.operations += tube.Test.ExactOperations
		if e != nil {
			return out, e
		}
		if !tube.ProvenBranch {
			return out, ErrConvergence
		}
		out.Tubes = append(out.Tubes, tube)
		if present {
			rectangles = append(rectangles, rect)
		}
	}
	if !nonempty {
		out.Covered = true
		return out, nil
	}
	covered, e := radialRectangleStrataCovered(ctx, box, rectangles, r.radial.center, r.radial.radii, o, budget, &out)
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

// Relative-open strata make rectangle membership constant. Distance squared
// takes an interval of values on each connected stratum; endpoint attainability
// is retained separately (including isolated tangent edge/vertex contacts).
func radialRectangleStrataCovered(ctx context.Context, box radialExactRectangle, targets []radialExactRectangle, center [2]*big.Rat, radii []*big.Rat, o SurfaceTubeUnionCoverageOptions, b *polynomialBudget, out *SurfaceTubeUnionCoverage) (bool, error) {
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
			min, max, minAttained, maxAttained, e := radialStratumDistance([2][2]*big.Rat{{x0, x1}, {y0, y1}}, center, b)
			if e != nil {
				return false, e
			}
			for _, radius := range radii {
				squared := new(big.Rat).Mul(radius, radius)
				if e := b.check(squared); e != nil {
					return false, e
				}
				lo, hi := squared.Cmp(min), squared.Cmp(max)
				if lo > 0 && hi < 0 || lo == 0 && minAttained || hi == 0 && maxAttained {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

func radialStratumDistance(domain [2][2]*big.Rat, center [2]*big.Rat, b *polynomialBudget) (lo, hi *big.Rat, loAttained, hiAttained bool, err error) {
	lo, hi = new(big.Rat), new(big.Rat)
	loAttained, hiAttained = true, true
	for axis, interval := range domain {
		a := new(big.Rat).Sub(interval[0], center[axis])
		z := new(big.Rat).Sub(interval[1], center[axis])
		aa := new(big.Rat).Mul(a, a)
		zz := new(big.Rat).Mul(z, z)
		minimum, maximum := aa, zz
		if aa.Cmp(zz) > 0 {
			minimum, maximum = zz, aa
		}
		point := interval[0].Cmp(interval[1]) == 0
		attained := point
		if !point {
			hiAttained = false
			if a.Sign() < 0 && z.Sign() > 0 {
				minimum = new(big.Rat)
				attained = true
			} else if a.Sign() <= 0 && z.Sign() >= 0 {
				minimum = new(big.Rat)
			}
		}
		loAttained = loAttained && attained
		lo.Add(lo, minimum)
		hi.Add(hi, maximum)
		for _, q := range []*big.Rat{a, z, aa, zz, lo, hi} {
			if e := b.check(q); e != nil {
				return nil, nil, false, false, e
			}
		}
	}
	return lo, hi, loAttained, hiAttained, nil
}
