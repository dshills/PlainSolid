package geom

import "context"

// SurfacePairParameterBox is a closed four-parameter product rectangle.
type SurfacePairParameterBox struct{ A, B ParameterRectangle }

// SurfaceTubeCoverageOptions selects the verified tube's free axis and bounds
// proof work. Zero limits select depth 12, 4096 outside cells, 4M exact operations
// and 16384 coefficient bits. Exact trimming and separation charge this budget.
type SurfaceTubeCoverageOptions struct{ FreeAxis, MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int }

// SurfaceTubeCoverage proves containment of CONTACT, not necessarily containment
// of the candidate parameter box. Covered requires a verified branch tube and
// exclusion of all candidate portions outside its box. It does not imply that
// the candidate actually contains contact. Unresolved/Pending preserve outside
// boxes for which contact absence remains unproved. OutsideExcluded boxes may
// overlap at their boundaries; exclusions remain exact there as well.
type SurfaceTubeCoverage struct {
	Tube SurfaceTubeCertificate
	// BoundaryCovered boxes may contain contact, all proved inside the verified tube.
	BoundaryCovered                      []SurfacePairParameterBox
	OutsideExcluded, Unresolved, Pending []SurfacePairParameterBox
	Covered                              bool
	VisitedCells, ExactOperations        int
}

func pairParameterRanges(p SurfacePairParameterBox) [4][2]float64 {
	return [4][2]float64{{p.A.UMin, p.A.UMax}, {p.A.VMin, p.A.VMax}, {p.B.UMin, p.B.UMax}, {p.B.VMin, p.B.VMax}}
}
func rangesParameterPair(r [4][2]float64) SurfacePairParameterBox {
	return SurfacePairParameterBox{ParameterRectangle{r[0][0], r[0][1], r[1][0], r[1][1]}, ParameterRectangle{r[2][0], r[2][1], r[3][0], r[3][1]}}
}

// outsideTubeSlabs covers the complement by at most eight closed slabs. Shared
// tube-boundary points are intentionally included, making exclusion conservative.
func outsideTubeSlabs(candidate, tube SurfacePairParameterBox) []SurfacePairParameterBox {
	current, target := pairParameterRanges(candidate), pairParameterRanges(tube)
	var intersection [4][2]float64
	for axis := 0; axis < 4; axis++ {
		intersection[axis] = [2]float64{max(current[axis][0], target[axis][0]), min(current[axis][1], target[axis][1])}
		if intersection[axis][0] >= intersection[axis][1] {
			return []SurfacePairParameterBox{candidate}
		}
	}
	out := []SurfacePairParameterBox{}
	for axis := 0; axis < 4; axis++ {
		if current[axis][0] < intersection[axis][0] {
			slab := current
			slab[axis][1] = intersection[axis][0]
			out = append(out, rangesParameterPair(slab))
			current[axis][0] = intersection[axis][0]
		}
		if current[axis][1] > intersection[axis][1] {
			slab := current
			slab[axis][0] = intersection[axis][1]
			out = append(out, rangesParameterPair(slab))
			current[axis][1] = intersection[axis][1]
		}
	}
	return out
}

// CertifySurfacePairCellInTube revalidates a tube from shared original surfaces,
// then excludes every outside candidate slab using exact separating control-hull
// projections with knot-aware subdivision. A nonseparating hull never implies
// contact or coverage. Closed slabs retain tube boundaries; exact contact confinement may prove those
// contacts belong to the verified tube. Unproved boundary contacts remain open.
func CertifySurfacePairCellInTube(a, b *SurfacePatch, candidate, tube SurfacePairParameterBox, o SurfaceTubeCoverageOptions) (result SurfaceTubeCoverage, err error) {
	if a == nil || b == nil || !candidate.A.Valid() || !candidate.B.Valid() || !tube.A.Valid() || !tube.B.Valid() || o.FreeAxis < 0 || o.FreeAxis > 3 {
		return result, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	for _, item := range []struct {
		p *SurfacePatch
		r ParameterRectangle
	}{{a, candidate.A}, {b, candidate.B}, {a, tube.A}, {b, tube.B}} {
		d := item.p.Domain()
		r := item.r
		if r.UMin < d.UMin || r.UMax > d.UMax || r.VMin < d.VMin || r.VMax > d.VMax {
			return result, ErrDomain
		}
	}
	defer func() {
		if err != nil && !result.Covered && len(result.Pending) == 0 && len(result.Unresolved) == 0 {
			result.Pending = append(result.Pending, candidate)
		}
	}()
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	ta, e := trimPatchBudget(a, tube.A, budget)
	result.ExactOperations = budget.operations
	if e != nil {
		return result, e
	}
	tb, e := trimPatchBudget(b, tube.B, budget)
	result.ExactOperations = budget.operations
	if e != nil {
		return result, e
	}
	if result.ExactOperations >= o.MaxOperations {
		return result, ErrConvergence
	}
	result.Tube, e = CertifySurfacePairTube(ta, tb, SurfacePairTubeOptions{FreeAxis: o.FreeAxis, MaxOperations: o.MaxOperations - result.ExactOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	result.ExactOperations += result.Tube.Test.ExactOperations
	if e != nil {
		return result, e
	}
	if !result.Tube.ProvenBranch {
		return result, ErrConvergence
	}
	err = excludeSurfaceTubeOutsideCovered(context.Background(), a, b, &result, outsideTubeSlabs(candidate, tube), o, []SurfacePairTubeRegion{{Box: tube, FreeAxis: o.FreeAxis}})
	return result, err
}

func excludeSurfaceTubeOutside(ctx context.Context, a, b *SurfacePatch, result *SurfaceTubeCoverage, slabs []SurfacePairParameterBox, o SurfaceTubeCoverageOptions) error {
	return excludeSurfaceTubeOutsideCovered(ctx, a, b, result, slabs, o, nil)
}
func excludeSurfaceTubeOutsideCovered(ctx context.Context, a, b *SurfacePatch, result *SurfaceTubeCoverage, slabs []SurfacePairParameterBox, o SurfaceTubeCoverageOptions, tubes []SurfacePairTubeRegion) error {
	type node struct {
		box   SurfacePairParameterBox
		depth int
	}
	stack := []node{}
	for _, slab := range slabs {
		stack = append(stack, node{slab, 0})
	}
	pending := func(current *node) {
		if current != nil {
			result.Pending = append(result.Pending, current.box)
		}
		for _, n := range stack {
			result.Pending = append(result.Pending, n.box)
		}
	}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			pending(nil)
			return e
		}
		if result.VisitedCells >= o.MaxCells || result.ExactOperations >= o.MaxOperations {
			pending(nil)
			return ErrConvergence
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result.VisitedCells++
		budget := &polynomialBudget{maxOperations: o.MaxOperations - result.ExactOperations, maxBits: o.MaxCoefficientBits}
		pa, e := trimPatchBudget(a, n.box.A, budget)
		if e != nil {
			result.ExactOperations += budget.operations
			pending(&n)
			return e
		}
		pb, e := trimPatchBudget(b, n.box.B, budget)
		result.ExactOperations += budget.operations
		if e != nil {
			pending(&n)
			return e
		}
		if result.ExactOperations >= o.MaxOperations {
			pending(&n)
			return ErrConvergence
		}
		separation, e := SeparateSurfacePatches(pa, pb, SurfaceSeparationOptions{MaxOperations: o.MaxOperations - result.ExactOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		result.ExactOperations += separation.ExactOperations
		if e != nil {
			pending(&n)
			return e
		}
		if separation.Separated {
			result.OutsideExcluded = append(result.OutsideExcluded, n.box)
			continue
		}
		if len(tubes) > 0 {
			coverage, excluded, contracted, changed, work, e := contractSurfaceContactToTubes(ctx, pa, pb, n.box, tubes, o.MaxOperations-result.ExactOperations, o.MaxCoefficientBits)
			result.ExactOperations += work
			if e != nil {
				pending(&n)
				return e
			}
			if coverage {
				result.BoundaryCovered = append(result.BoundaryCovered, n.box)
				continue
			}
			if excluded {
				result.OutsideExcluded = append(result.OutsideExcluded, n.box)
				continue
			}
			if changed && n.depth < o.MaxDepth {
				stack = append(stack, node{contracted, n.depth + 1})
				continue
			}
		}
		if n.depth >= o.MaxDepth {
			result.Unresolved = append(result.Unresolved, n.box)
			continue
		}
		ta, ua, ka := patchSplitChoice(pa)
		tb, ub, kb := patchSplitChoice(pb)
		splitA := ka || (!kb && patchParameterWidth(pa) >= patchParameterWidth(pb))
		axis, t := 0, ta
		if !ua {
			axis = 1
		}
		if !splitA {
			axis, t = 2, tb
			if !ub {
				axis = 3
			}
		}
		r := pairParameterRanges(n.box)
		if t == r[axis][0] || t == r[axis][1] {
			result.Unresolved = append(result.Unresolved, n.box)
			continue
		}
		left, right := r, r
		left[axis][1] = t
		right[axis][0] = t
		stack = append(stack, node{rangesParameterPair(right), n.depth + 1}, node{rangesParameterPair(left), n.depth + 1})
	}
	if len(result.Unresolved) > 0 {
		return ErrConvergence
	}
	result.Covered = true
	return nil
}
