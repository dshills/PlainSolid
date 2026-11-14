package intersect

import (
	"context"
	"errors"
	"sort"

	"plainsolid/geom"
)

// TubeChainOptions bounds pairwise endpoint checks. Zero MaxJunctions selects
// 4096. The arithmetic limits apply independently to each proof.
type TubeChainOptions struct {
	MaxJunctions int
	Patches      geom.SurfacePatchOptions
	Junction     geom.SurfaceTubeJunctionOptions
}

// TubeChain is an increasing-free-parameter chain of local branch
// certificates. Cells are indices in the original seed report's Attempts.
// Different free axes are never joined by this operation.
type TubeChain struct {
	FreeAxis int
	Cells    []int
}

type TubeJunction struct {
	Left, Right int
	Certificate geom.SurfaceTubeJunction
}

// TubeChainReport records matched and unresolved junctions. Truncated marks a
// work limit. Chains cover only successful local tube proofs, never all
// intersections unless a separate exhaustive global argument is available.
type TubeChainReport struct {
	Chains    []TubeChain
	Junctions []TubeJunction
	Truncated bool
}

// TubeOverlap records a local same-branch proof. It is not an endpoint
// junction; callers must choose how to trim overlapping ranges before use.
type TubeOverlap struct {
	First, Second int
	Certificate   geom.SurfaceTubeOverlap
}

// FindSplineTubeOverlaps checks intersecting four-parameter interiors of
// certified tube candidates, including differently parameterized tubes.
// MaxJunctions bounds the number of pair comparisons, including rejected
// broad-phase pairs. The returned boolean indicates truncation.
func FindSplineTubeOverlaps(ctx context.Context, a, b Patch, seeds SplineSeedReport, o TubeChainOptions) ([]TubeOverlap, bool, error) {
	if o.MaxJunctions == 0 {
		o.MaxJunctions = 4096
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxJunctions < 1 || len(seeds.Attempts) > 32768 {
		return nil, false, geom.ErrInvalid
	}
	pa, e := exactSplinePatch(a, o.Patches)
	if e != nil {
		return nil, false, e
	}
	pb, e := exactSplinePatch(b, o.Patches)
	if e != nil {
		return nil, false, e
	}
	ranges := func(c geom.SurfaceSurfaceCell) [4][2]float64 {
		x, y := c.DomainA, c.DomainB
		return [4][2]float64{{x.UMin, x.UMax}, {x.VMin, x.VMax}, {y.UMin, y.UMax}, {y.VMin, y.VMax}}
	}
	var r []TubeOverlap
	n := 0
	for i, l := range seeds.Attempts {
		if e := ctx.Err(); e != nil {
			return r, false, e
		}
		if l.Tube == nil || !l.Tube.ProvenBranch {
			continue
		}
		if l.Tube.FreeAxis < 0 || l.Tube.FreeAxis > 3 {
			return nil, false, geom.ErrInvalid
		}
		for j := i + 1; j < len(seeds.Attempts); j++ {
			if e := ctx.Err(); e != nil {
				return r, false, e
			}
			x := seeds.Attempts[j]
			if x.Tube == nil || !x.Tube.ProvenBranch {
				continue
			}
			if x.Tube.FreeAxis < 0 || x.Tube.FreeAxis > 3 {
				return nil, false, geom.ErrInvalid
			}
			if n == o.MaxJunctions {
				return r, true, nil
			}
			n++
			u, v := ranges(l.Cell), ranges(x.Cell)
			intersection := [4][2]float64{}
			valid := true
			for k := range 4 {
				lo, hi := max(u[k][0], v[k][0]), min(u[k][1], v[k][1])
				intersection[k] = [2]float64{lo, hi}
				valid = valid && lo < hi
			}
			if !valid {
				continue
			}
			for k, axis := range [2]int{l.Tube.FreeAxis, x.Tube.FreeAxis} {
				if k == 1 && axis == l.Tube.FreeAxis {
					break
				}
				lo, hi := intersection[axis][0], intersection[axis][1]
				m := lo/2 + hi/2
				if m <= lo || m >= hi {
					continue
				}
				p, e := geom.CertifySurfaceTubeOverlap(pa, pb, l.Cell.DomainA, l.Cell.DomainB, x.Cell.DomainA, x.Cell.DomainB, geom.SurfaceTubeOverlapOptions{
					LeftFreeAxis: l.Tube.FreeAxis, RightFreeAxis: x.Tube.FreeAxis, WitnessAxis: axis, WitnessParameter: m,
					MaxOperations: o.Junction.MaxOperations, MaxCoefficientBits: o.Junction.MaxCoefficientBits,
				})
				if e != nil {
					if errors.Is(e, geom.ErrConvergence) {
						continue
					}
					return r, false, e
				}
				if p.SameLocalBranch {
					r = append(r, TubeOverlap{i, j, p})
					break
				}
			}
		}
	}
	return r, false, nil
}

// JoinSplineTubes connects abutting local tube certificates on the same
// ordered pair of spline surfaces. It uses exact section proofs, not spatial
// proximity. Cells with ambiguous continuations are left unjoined.
func JoinSplineTubes(ctx context.Context, a, b Patch, seeds SplineSeedReport, o TubeChainOptions) (TubeChainReport, error) {
	var out TubeChainReport
	if o.MaxJunctions == 0 {
		o.MaxJunctions = 4096
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxJunctions < 1 || len(seeds.Attempts) > 32768 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	pa, e := exactSplinePatch(a, o.Patches)
	if e != nil {
		return out, e
	}
	pb, e := exactSplinePatch(b, o.Patches)
	if e != nil {
		return out, e
	}
	type boundary struct {
		axis int
		t    float64
	}
	starts := map[boundary][]int{}
	var ids []int
	for i, x := range seeds.Attempts {
		t := x.Tube
		if t == nil || !t.ProvenBranch {
			continue
		}
		if t.FreeAxis < 0 || t.FreeAxis > 3 || !finite(t.FreeParameter.Min) || !finite(t.FreeParameter.Max) || t.FreeParameter.Min >= t.FreeParameter.Max {
			return out, geom.ErrInvalid
		}
		ids = append(ids, i)
		k := boundary{t.FreeAxis, t.FreeParameter.Min}
		starts[k] = append(starts[k], i)
	}
	// Check each reported certificate against the supplied surfaces. In
	// particular, a standalone chain may have no neighboring junction to
	// revalidate it. The seed report is caller-owned and can be modified.
	for _, i := range ids {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		x := seeds.Attempts[i]
		a, e := pa.Trim(x.Cell.DomainA)
		if e != nil {
			return out, e
		}
		b, e := pb.Trim(x.Cell.DomainB)
		if e != nil {
			return out, e
		}
		c, e := certifySplineTubeCell(ctx, a, b, geom.SurfacePairTubeOptions{FreeAxis: x.Tube.FreeAxis, MaxOperations: o.Junction.MaxOperations, MaxCoefficientBits: o.Junction.MaxCoefficientBits}, x.BoundaryAnchor)
		if e != nil {
			return out, e
		}
		if !c.ProvenBranch || c.FreeParameter != x.Tube.FreeParameter {
			return out, geom.ErrInvalid
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := seeds.Attempts[ids[i]].Tube, seeds.Attempts[ids[j]].Tube
		if a.FreeAxis != b.FreeAxis {
			return a.FreeAxis < b.FreeAxis
		}
		if a.FreeParameter.Min != b.FreeParameter.Min {
			return a.FreeParameter.Min < b.FreeParameter.Min
		}
		return ids[i] < ids[j]
	})
	next, previous := map[int][]int{}, map[int][]int{}
	blockNext, blockPrevious := map[int]bool{}, map[int]bool{}
search:
	for _, i := range ids {
		l := seeds.Attempts[i]
		k := boundary{l.Tube.FreeAxis, l.Tube.FreeParameter.Max}
		for _, j := range starts[k] {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if len(out.Junctions) >= o.MaxJunctions {
				out.Truncated = true
				break search
			}
			r := seeds.Attempts[j]
			op := o.Junction
			op.FreeAxis = k.axis
			c, e := geom.CertifySurfaceTubeJunction(pa, pb, l.Cell.DomainA, l.Cell.DomainB, r.Cell.DomainA, r.Cell.DomainB, op)
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return out, e
			}
			out.Junctions = append(out.Junctions, TubeJunction{i, j, c})
			switch {
			case c.Connected:
				next[i] = append(next[i], j)
				previous[j] = append(previous[j], i)
			case !c.Excluded:
				blockNext[i], blockPrevious[j] = true, true
			}
		}
	}
	// On truncation emit only local chains: an unvisited competitor can
	// make an otherwise valid junction ambiguous.
	seen := map[int]bool{}
	for _, i := range ids {
		if seen[i] {
			continue
		}
		chain := TubeChain{FreeAxis: seeds.Attempts[i].Tube.FreeAxis}
		for {
			chain.Cells = append(chain.Cells, i)
			seen[i] = true
			if out.Truncated || blockNext[i] || len(next[i]) != 1 {
				break
			}
			j := next[i][0]
			if blockPrevious[j] || len(previous[j]) != 1 || seen[j] {
				break
			}
			i = j
		}
		out.Chains = append(out.Chains, chain)
	}
	return out, nil
}
