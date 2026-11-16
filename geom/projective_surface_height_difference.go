package geom

import (
	"context"
	"errors"
	"math/big"
	"sort"
)

// ProjectiveSurfaceHeightCell holds one original common knot cell. Common
// is in A's normalized overlap clock. OriginalCells retain the preimage
// knot rectangles on both original owners. Difference is a unit-square
// rational patch with coordinates (selected projection 1, selected
// projection 2, residual coordinate of A minus residual coordinate of B).
type ProjectiveSurfaceHeightCell struct {
	Common        ExactSurfaceRectangle
	OriginalCells [2]ParameterRectangle
	Difference    *SurfacePatch
}

// ProjectiveSurfaceHeightDifference is an exact positive-area common graph
// residual atlas, not a contact graph. It retains nonbinary union cuts in
// Common instead of rounding them into a synthetic knot vector.
type ProjectiveSurfaceHeightDifference struct {
	Overlap         ProjectiveSurfaceGraphOverlap
	Cells           []ProjectiveSurfaceHeightCell
	Complete        bool
	ExactOperations int
}

// ProjectiveSurfaceOriginalCuts indexes the owning surface first, then its
// original U or V axis. An exact cut inside the proved overlap is added to
// its common atlas axis. Cuts on the boundary or outside the overlap leave
// the existing grid unchanged, provided they are on their original chart.
type ProjectiveSurfaceOriginalCuts [2][2][]*big.Rat

// BuildProjectiveSurfaceHeightDifference builds the exact residual on both
// original graphs' entire positive-area projected overlap. MaxCells bounds
// both extraction and common rectangles. MaxPoles bounds source/refined nets
// and the combined output controls. No partial atlas is returned on failure.
func BuildProjectiveSurfaceHeightDifference(ctx context.Context, a, c *SurfacePatch, axes [2]int,
	o GeneralSurfaceHeightOptions) (r ProjectiveSurfaceHeightDifference, err error) {
	return BuildProjectiveSurfaceHeightDifferenceWithOriginalCuts(ctx, a, c, axes, ProjectiveSurfaceOriginalCuts{}, o)
}

// BuildProjectiveSurfaceHeightDifferenceWithOriginalCuts inserts additional
// exact stations from both original parameter charts in the common grid.
// The result retains all original knot owners; only the proof tiles split.
// These cuts do not change either original surface or the covered area.
func BuildProjectiveSurfaceHeightDifferenceWithOriginalCuts(ctx context.Context, a, c *SurfacePatch, axes [2]int,
	cuts ProjectiveSurfaceOriginalCuts, o GeneralSurfaceHeightOptions) (r ProjectiveSurfaceHeightDifference, err error) {
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	if o.MaxCells < 1 {
		return r, ErrInvalid
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, err
	}
	overlap, err := CertifyProjectiveSurfaceGraphOverlap(ctx, a, c, axes, limits)
	b := &polynomialBudget{operations: overlap.ExactOperations, maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if err != nil && !errors.Is(err, ErrConvergence) {
		return r, err
	}
	if overlap.NoArea {
		return r, ErrDomain
	}
	// These fields are published after both exact full projections and
	// their clipped correspondence, before comparing the two lifts.
	// Failure of lift equality does not invalidate the shared projection.
	if overlap.Areas[1].V.Upper == nil || b.operations >= b.maxOperations {
		return r, ErrConvergence
	}
	var cells [2][]projectiveHeightOwnerCell
	for i, p := range [2]*SurfacePatch{a, c} {
		cells[i], err = projectiveHeightOwnerCells(ctx, p, overlap, i, limits.MaxPoles, o.MaxCells, b)
		if err != nil {
			return r, err
		}
	}
	var breaks [2][]*big.Rat
	for _, pieces := range cells {
		for _, p := range pieces {
			breaks[0] = append(breaks[0], p.tensor.u0, p.tensor.u1)
			breaks[1] = append(breaks[1], p.tensor.v0, p.tensor.v1)
		}
	}
	if err := appendProjectiveSurfaceOriginalCuts(ctx, &breaks, [2]*SurfacePatch{a, c}, overlap, cuts, o.MaxCells, b); err != nil {
		return r, err
	}
	for i, values := range breaks {
		sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
		unique := values[:1]
		for _, x := range values[1:] {
			if x.Cmp(unique[len(unique)-1]) != 0 {
				unique = append(unique, x)
			}
		}
		breaks[i] = unique
	}
	nu, nv := len(breaks[0])-1, len(breaks[1])-1
	if nu <= 0 || nv <= 0 || nu > o.MaxCells/nv {
		return r, ErrConvergence
	}
	du, dv := cells[0][0].tensor.nu+cells[1][0].tensor.nu-2, cells[0][0].tensor.nv+cells[1][0].tensor.nv-2
	left := limits.MaxPoles / ((du + 1) * (dv + 1))
	if du > 64 || dv > 64 || nu > left/nv {
		return r, ErrConvergence
	}
	out := make([]ProjectiveSurfaceHeightCell, 0, nu*nv)
	for i := 0; i < nu; i++ {
		for j := 0; j < nv; j++ {
			if err := ctx.Err(); err != nil {
				return r, err
			}
			d := [2][2]*big.Rat{{breaks[0][i], breaks[0][i+1]}, {breaks[1][j], breaks[1][j+1]}}
			cell := ProjectiveSurfaceHeightCell{Common: ExactSurfaceRectangle{
				ExactParameterInterval{new(big.Rat).Set(d[0][0]), new(big.Rat).Set(d[0][1])},
				ExactParameterInterval{new(big.Rat).Set(d[1][0]), new(big.Rat).Set(d[1][1])}}}
			var owners [2]*SurfacePatch
			for owner, group := range cells {
				for _, p := range group {
					x := p.tensor
					if x.u0.Cmp(d[0][0]) > 0 || x.u1.Cmp(d[0][1]) < 0 || x.v0.Cmp(d[1][0]) > 0 || x.v1.Cmp(d[1][1]) < 0 {
						continue
					}
					local := func(t, lo, hi *big.Rat) *big.Rat {
						return new(big.Rat).Quo(new(big.Rat).Sub(t, lo), new(big.Rat).Sub(hi, lo))
					}
					net, err := restrictComparisonTensor(x, local(d[0][0], x.u0, x.u1), local(d[0][1], x.u0, x.u1),
						local(d[1][0], x.v0, x.v1), local(d[1][1], x.v0, x.v1), b)
					if err != nil {
						return r, err
					}
					cell.OriginalCells[owner] = p.original
					owners[owner] = &SurfacePatch{degreeU: x.nu - 1, degreeV: x.nv - 1, countU: x.nu, countV: x.nv, net: net}
					break
				}
				if owners[owner] == nil {
					return r, ErrConvergence
				}
			}
			cell.Difference, err = projectiveHeightResidual(owners[0], owners[1], axes, du, dv, limits, b)
			if err != nil {
				return r, err
			}
			out = append(out, cell)
		}
	}
	r.Overlap, r.Cells, r.Complete = overlap, out, true
	return r, nil
}

func appendProjectiveSurfaceOriginalCuts(ctx context.Context, breaks *[2][]*big.Rat, sources [2]*SurfacePatch,
	overlap ProjectiveSurfaceGraphOverlap, cuts ProjectiveSurfaceOriginalCuts, limit int, b *polynomialBudget) error {
	one := big.NewRat(1, 1)
	a := boundedRatArithmetic{b: b}
	for owner, c := range cuts {
		if len(c[0]) == 0 && len(c[1]) == 0 {
			continue
		}
		source := projectiveParameterIntervals(sources[owner].Domain())
		selected := [2]ExactParameterInterval{overlap.Areas[owner].U, overlap.Areas[owner].V}
		for raw, stops := range c {
			if len(stops) > limit {
				return ErrConvergence
			}
			axis := raw
			if owner == 1 && overlap.SwapB {
				axis = 1 - raw
			}
			x := selected[raw]
			for _, q := range stops {
				if err := ctx.Err(); err != nil {
					return err
				}
				if q == nil {
					return ErrInvalid
				}
				if err := b.check(q); err != nil {
					return err
				}
				if q.Cmp(source[raw].lo) < 0 || q.Cmp(source[raw].hi) > 0 {
					return ErrDomain
				}
				if q.Cmp(x.Lower) <= 0 || q.Cmp(x.Upper) >= 0 {
					continue
				}
				t := a.quo(a.sub(q, x.Lower), a.sub(x.Upper, x.Lower))
				if owner == 1 {
					if [2]bool{overlap.ReverseBU, overlap.ReverseBV}[axis] {
						t = a.sub(one, t)
					}
					ratio := overlap.BStationRatios[axis]
					// Invert s=ratio*t/(1+(ratio-1)*t) to get A's
					// common clock. The denominator stays positive.
					t = a.quo(t, a.add(ratio, a.mul(a.sub(one, ratio), t)))
				}
				if a.err != nil {
					return a.err
				}
				if t.Sign() <= 0 || t.Cmp(one) >= 0 {
					return ErrConvergence
				}
				breaks[axis] = append(breaks[axis], t)
			}
		}
	}
	return nil
}

type projectiveHeightOwnerCell struct {
	tensor   comparisonTensor
	original ParameterRectangle
}

func projectiveHeightOwnerCells(ctx context.Context, p *SurfacePatch, mapUV ProjectiveSurfaceGraphOverlap,
	owner, maxPoles, maxCells int, b *polynomialBudget) ([]projectiveHeightOwnerCell, error) {
	root := *p
	root.options.MaxPoles = min(root.options.MaxPoles, maxPoles)
	area := [2]ExactParameterInterval{mapUV.Areas[owner].U, mapUV.Areas[owner].V}
	swap := owner == 1 && mapUV.SwapB
	reverse := [2]bool{owner == 1 && mapUV.ReverseBU, owner == 1 && mapUV.ReverseBV}
	var ratios [2]*big.Rat
	if owner == 1 {
		ratios = mapUV.BStationRatios
	}
	var out []projectiveHeightOwnerCell
	stack := []*SurfacePatch{&root}
	for len(stack) != 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		i := len(stack) - 1
		p := stack[i]
		stack = stack[:i]
		d := p.Domain()
		raw := projectiveParameterIntervals(d)
		u, v := raw[0], raw[1]
		if u.lo.Cmp(area[0].Upper) >= 0 || u.hi.Cmp(area[0].Lower) <= 0 ||
			v.lo.Cmp(area[1].Upper) >= 0 || v.hi.Cmp(area[1].Lower) <= 0 {
			continue
		}
		if k, alongU, ok := patchSplitChoice(p); ok {
			if len(stack)+len(out)+2 > maxCells {
				return nil, ErrConvergence
			}
			l, h, err := p.split(k, alongU, b)
			if err != nil {
				return nil, err
			}
			stack = append(stack, h, l)
			continue
		}
		x := comparisonTensor{nu: p.countU, nv: p.countV, net: make([]exactHomogeneous, len(p.net))}
		intervals, aligned := [2]exactInterval{u, v}, area
		if swap {
			x.nu, x.nv = x.nv, x.nu
			intervals[0], intervals[1], aligned[0], aligned[1] = intervals[1], intervals[0], aligned[1], aligned[0]
		}
		a := boundedRatArithmetic{b: b}
		one := big.NewRat(1, 1)
		for k, q := range intervals {
			width := a.sub(aligned[k].Upper, aligned[k].Lower)
			lo, hi := a.quo(a.sub(q.lo, aligned[k].Lower), width), a.quo(a.sub(q.hi, aligned[k].Lower), width)
			if reverse[k] {
				lo, hi = a.sub(one, hi), a.sub(one, lo)
			}
			intervals[k] = exactInterval{lo, hi}
		}
		if a.err != nil {
			return nil, a.err
		}
		x.u0, x.u1, x.v0, x.v1 = intervals[0].lo, intervals[0].hi, intervals[1].lo, intervals[1].hi
		for i := range x.nu {
			for j := range x.nv {
				u, v := i, j
				if reverse[0] {
					u = x.nu - 1 - u
				}
				if reverse[1] {
					v = x.nv - 1 - v
				}
				if swap {
					u, v = v, u
				}
				x.net[i*x.nv+j] = p.net[u*p.countV+v]
			}
		}
		pulled, ok, err := pullComparisonTensor(x, ratios, b)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, projectiveHeightOwnerCell{pulled, d})
		}
	}
	if len(out) == 0 {
		return nil, ErrConvergence
	}
	return out, nil
}

func projectiveHeightResidual(a, c *SurfacePatch, axes [2]int, du, dv int, limits SurfacePatchOptions,
	b *polynomialBudget) (*SurfacePatch, error) {
	first := b.operations
	x, err := tensorHomogeneousBernsteinProduct(a, axes[0], c, 3, b)
	if err != nil {
		return nil, err
	}
	y, err := tensorHomogeneousBernsteinProduct(a, axes[1], c, 3, b)
	if err != nil {
		return nil, err
	}
	w, err := tensorHomogeneousBernsteinProduct(a, 3, c, 3, b)
	if err != nil {
		return nil, err
	}
	z := 3 - axes[0] - axes[1]
	h0, err := tensorHomogeneousBernsteinProduct(a, z, c, 3, b)
	if err != nil {
		return nil, err
	}
	h1, err := tensorHomogeneousBernsteinProduct(a, 3, c, z, b)
	if err != nil {
		return nil, err
	}
	knots := func(degree int) []float64 {
		k := make([]float64, 2*(degree+1))
		for i := degree + 1; i < len(k); i++ {
			k[i] = 1
		}
		return k
	}
	p := &SurfacePatch{degreeU: du, degreeV: dv, countU: du + 1, countV: dv + 1,
		knotsU: knots(du), knotsV: knots(dv), net: make([]exactHomogeneous, len(w)), options: limits}
	for i := range w {
		if w[i].Sign() <= 0 {
			return nil, ErrConvergence
		}
		delta := new(big.Rat).Sub(h0[i], h1[i])
		if err := b.check(delta); err != nil {
			return nil, err
		}
		p.net[i] = exactHomogeneous{x[i], y[i], delta, w[i]}
	}
	if err := p.computeBounds(b); err != nil {
		return nil, err
	}
	p.operations = b.operations - first
	return p, nil
}
