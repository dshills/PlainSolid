package intersect

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// AffineChartDiscoveryOptions enables bounded exact lattice-guided proposals.
// It is not a coverage certificate. Defaults: adaptive 4..32 slabs, 16M exact operations shared across retries.
type AffineChartDiscoveryOptions struct {
	Enabled                                      bool
	Divisions, MaxOperations, MaxCoefficientBits int
	// LatticeDivisions selects exact i/n chart sites, default2, bounded2..8.
	// Nonbinary sites are permitted; native proposal boxes still must be lossless.
	LatticeDivisions int
	// ConstantPaddingDivisor controls proposal widths only (default8, power of2).
	ConstantPaddingDivisor int
}

// DiscoverAffineChartPairTubes finds common model points at each original
// chart's bounded rational lattice, requires one affine four-parameter site line and
// proposes exact represented subslabs. Every slab is uniformly re-proved and
// its complete paired affine path is certified. Other branches can only be
// excluded by the caller's unchanged independent original-domain coverage.
// Exact source recovery uses p at separately bounded stages; lattice arithmetic,
// evaluation, uniform proofs and path identity share o.MaxOperations across retries.
func DiscoverAffineChartPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o AffineChartDiscoveryOptions) (SplineSeedReport, error) {
	if o.Divisions != 0 {
		r, e := discoverAffineChartPairTubes(ctx, a, b, p, o, nil)
		return r, affineChartDiscoveryError(e)
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	remaining := o.MaxOperations
	for _, divisions := range []int{4, 8, 16, 32} {
		local := o
		local.Divisions = divisions
		local.MaxOperations = remaining
		spent := 0
		result, e := discoverAffineChartPairTubes(ctx, a, b, p, local, &spent)
		remaining -= spent
		e = affineChartDiscoveryError(e)
		if e == nil {
			return result, nil
		}
		if !errors.Is(e, ErrUnresolved) && !errors.Is(e, geom.ErrConvergence) {
			return SplineSeedReport{}, e
		}
		if remaining < 1 {
			return SplineSeedReport{}, geom.ErrConvergence
		}
	}
	return SplineSeedReport{}, ErrUnresolved
}
func affineChartDiscoveryError(e error) error {
	if errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrUnrepresentable) || errors.Is(e, geom.ErrDegenerate) {
		return fmt.Errorf("affine chart discovery: unsupported proof domain (%v): %w", e, ErrUnresolved)
	}
	return e
}
func discoverAffineChartPairTubes(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o AffineChartDiscoveryOptions, spent *int) (SplineSeedReport, error) {
	empty := SplineSeedReport{}
	if ctx == nil || !a.valid() || !b.valid() {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.LatticeDivisions == 0 {
		o.LatticeDivisions = 2
	}
	if o.LatticeDivisions < 2 || o.LatticeDivisions > 8 {
		return empty, geom.ErrInvalid
	}
	if o.ConstantPaddingDivisor == 0 {
		o.ConstantPaddingDivisor = 8
	}
	if o.ConstantPaddingDivisor < 8 || o.ConstantPaddingDivisor > 256 || o.ConstantPaddingDivisor&(o.ConstantPaddingDivisor-1) != 0 {
		return empty, geom.ErrInvalid
	}
	if o.Divisions == 0 {
		o.Divisions = 4
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.Divisions < 2 || o.Divisions > 32 || o.Divisions&(o.Divisions-1) != 0 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 {
		return empty, geom.ErrInvalid
	}
	remaining := o.MaxOperations
	defer func() {
		if spent != nil {
			*spent = o.MaxOperations - remaining
		}
	}()
	check := func(q *big.Rat) error {
		remaining--
		if remaining < 0 || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
	}
	patches := [2]*geom.SurfacePatch{}
	var bounds [4][2]*big.Rat
	for i, owner := range [2]Patch{a, b} {
		var e error
		patches[i], e = exactSplinePatch(owner, p)
		if e != nil {
			return empty, e
		}
		bounds[2*i] = [2]*big.Rat{new(big.Rat).SetFloat64(owner.Min.X), new(big.Rat).SetFloat64(owner.Max.X)}
		bounds[2*i+1] = [2]*big.Rat{new(big.Rat).SetFloat64(owner.Min.Y), new(big.Rat).SetFloat64(owner.Max.Y)}
	}
	type site struct {
		uv    [2]*big.Rat
		point [3]*big.Rat
	}
	var sites [2][]site
	for owner, patch := range patches {
		for i := 0; i <= o.LatticeDivisions; i++ {
			for j := 0; j <= o.LatticeDivisions; j++ {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				var uv [2]*big.Rat
				for axis, n := range [2]int{i, j} {
					r := bounds[2*owner+axis]
					uv[axis] = new(big.Rat).Add(r[0], new(big.Rat).Mul(new(big.Rat).Sub(r[1], r[0]), big.NewRat(int64(n), int64(o.LatticeDivisions))))
					if e := check(uv[axis]); e != nil {
						return empty, e
					}
				}
				if remaining < 1 {
					return empty, geom.ErrConvergence
				}
				jet, e := patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
				remaining -= jet.ExactOperations
				if e != nil {
					return empty, e
				}
				sites[owner] = append(sites[owner], site{uv, jet.Point})
			}
		}
	}
	var matches [][4]*big.Rat
	for _, x := range sites[0] {
		for _, y := range sites[1] {
			same := true
			for axis := 0; axis < 3; axis++ {
				remaining--
				if remaining < 0 {
					return empty, geom.ErrConvergence
				}
				if x.point[axis].Cmp(y.point[axis]) != 0 {
					same = false
					break
				}
			}
			if same {
				matches = append(matches, [4]*big.Rat{x.uv[0], x.uv[1], y.uv[0], y.uv[1]})
			}
		}
	}
	if len(matches) < 2 {
		return empty, fmt.Errorf("affine chart discovery: insufficient exact lattice sites: %w", ErrUnresolved)
	}
	axis := -1
	for i := 0; i < 4; i++ {
		for _, q := range matches[1:] {
			if q[i].Cmp(matches[0][i]) != 0 {
				axis = i
				break
			}
		}
		if axis >= 0 {
			break
		}
	}
	if axis < 0 {
		return empty, fmt.Errorf("affine chart discovery: constant site tuple: %w", ErrUnresolved)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i][axis].Cmp(matches[j][axis]) < 0 })
	start, end := matches[0], matches[len(matches)-1]
	var delta [4]*big.Rat
	for i := range delta {
		delta[i] = new(big.Rat).Sub(end[i], start[i])
		if e := check(delta[i]); e != nil {
			return empty, e
		}
	}
	for _, q := range matches {
		for i := 0; i < 4; i++ {
			left := new(big.Rat).Mul(new(big.Rat).Sub(q[i], start[i]), delta[axis])
			right := new(big.Rat).Mul(new(big.Rat).Sub(q[axis], start[axis]), delta[i])
			if e := check(left); e != nil {
				return empty, e
			}
			if e := check(right); e != nil {
				return empty, e
			}
			if left.Cmp(right) != 0 {
				return empty, fmt.Errorf("affine chart discovery: nonaffine paired sites: %w", ErrUnresolved)
			}
		}
	}
	endpointAxis := func(q [4]*big.Rat) int {
		found := -1
		for i := 0; i < 4; i++ {
			if q[i].Cmp(bounds[i][0]) == 0 || q[i].Cmp(bounds[i][1]) == 0 {
				if delta[i].Sign() == 0 || found >= 0 {
					return -1
				}
				found = i
			}
		}
		return found
	}
	first, last := endpointAxis(start), endpointAxis(end)
	if first < 0 || last < 0 {
		return empty, fmt.Errorf("affine chart discovery: endpoint owner is ambiguous or absent: %w", ErrUnresolved)
	}
	out := SplineSeedReport{}
	for piece := 0; piece < o.Divisions; piece++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		free := axis
		if piece < o.Divisions/2 {
			free = first
		} else {
			free = last
		}
		var lo, hi [4]*big.Rat
		var boxes [4][2]float64
		for i := 0; i < 4; i++ {
			lo[i] = new(big.Rat).Add(start[i], new(big.Rat).Mul(delta[i], big.NewRat(int64(piece), int64(o.Divisions))))
			hi[i] = new(big.Rat).Add(start[i], new(big.Rat).Mul(delta[i], big.NewRat(int64(piece+1), int64(o.Divisions))))
			l, h := new(big.Rat).Set(lo[i]), new(big.Rat).Set(hi[i])
			if l.Cmp(h) > 0 {
				l, h = h, l
			}
			if i != free {
				divisor := int64(32)
				if delta[i].Sign() == 0 {
					divisor = int64(o.ConstantPaddingDivisor)
				}
				padding := new(big.Rat).Quo(new(big.Rat).Sub(bounds[i][1], bounds[i][0]), big.NewRat(divisor, 1))
				l.Sub(l, padding)
				h.Add(h, padding)
				if l.Cmp(bounds[i][0]) < 0 {
					l.Set(bounds[i][0])
				}
				if h.Cmp(bounds[i][1]) > 0 {
					h.Set(bounds[i][1])
				}
			}
			for j, q := range [2]*big.Rat{l, h} {
				if e := check(q); e != nil {
					return empty, e
				}
				f, e := geom.RepresentedExactScalar(q)
				if e != nil {
					return empty, fmt.Errorf("affine chart discovery: unrepresentable proposal box: %w", ErrUnresolved)
				}
				boxes[i][j] = f
			}
		}
		domains := [2]geom.ParameterRectangle{}
		var cell [2]*geom.SurfacePatch
		for i := 0; i < 2; i++ {
			domains[i] = geom.ParameterRectangle{UMin: boxes[2*i][0], UMax: boxes[2*i][1], VMin: boxes[2*i+1][0], VMax: boxes[2*i+1][1]}
			var e error
			cell[i], e = patches[i].Trim(domains[i])
			if e != nil {
				return empty, e
			}
		}
		if remaining < 1 {
			return empty, geom.ErrConvergence
		}
		opts := geom.SurfacePairTubeOptions{FreeAxis: free, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits}
		tube, proof, e := geom.NewVerifiedSurfaceTubeWithCertificate(cell[0], cell[1], opts)
		remaining -= proof.Test.ExactOperations
		if e != nil {
			return empty, e
		}
		if remaining < 1 {
			return empty, geom.ErrConvergence
		}
		path, e := tube.CertifyAffineParameterPath(ctx, lo, hi, geom.TubeAffineLineOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		remaining -= path.ExactOperations
		if e != nil {
			return empty, e
		}
		if !path.Proven {
			return empty, fmt.Errorf("affine chart discovery: unproved complete chart path: %w", ErrUnresolved)
		}
		out.Attempts = append(out.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: domains[0], DomainB: domains[1]}, Tube: &proof})
	}
	return out, nil
}
