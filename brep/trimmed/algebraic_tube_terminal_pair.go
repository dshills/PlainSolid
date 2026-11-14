package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicSplineBoundaryOwner identifies an original rectangular chart edge.
// Parameter is its original varying UV coordinate. EdgeParameter increases
// from the chart's lower to upper varying bound; it is not a B-rep coedge
// traversal parameter. ModelEvent independently binds that root to the support.
type AlgebraicSplineBoundaryOwner struct {
	Operand                  int
	Side                     geom.SurfaceEdge
	Parameter, EdgeParameter *geom.ExactAlgebraicRoot
	ModelEvent               *geom.ExactComposedCurveEvent
}

// AlgebraicSplineTerminalPair proves one noncorner edge occurrence on each
// original chart at the same immutable tube endpoint. It does not select a
// single owner or imply how adjoining shell faces continue through that event.
type AlgebraicSplineTerminalPair struct {
	owners [2]AlgebraicSplineBoundaryOwner
	event  *geom.ExactComposedCurveEvent
	work   int
}

func (p *AlgebraicSplineTerminalPair) Owners() [2]AlgebraicSplineBoundaryOwner {
	if p == nil {
		return [2]AlgebraicSplineBoundaryOwner{}
	}
	return p.owners
}
func (p *AlgebraicSplineTerminalPair) ModelEvent() *geom.ExactComposedCurveEvent {
	if p == nil {
		return nil
	}
	return p.event
}
func (p *AlgebraicSplineTerminalPair) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.work
}

type pairedTerminalBudget struct {
	remaining, used int
	options         geom.TubeAlgebraicGraphEndpointOptions
}

func newPairedTerminalBudget(o geom.TubeAlgebraicGraphEndpointOptions) (*pairedTerminalBudget, error) {
	n := o.MaxOperations
	if n == 0 {
		n = 128000000
	}
	if n < 1 || n > 1000000000 || o.MaxCoefficientBits < 0 || o.MaxSubdivisions < 0 {
		return nil, geom.ErrInvalid
	}
	return &pairedTerminalBudget{remaining: n, options: o}, nil
}
func (b *pairedTerminalBudget) debit(n int) error {
	if n < 0 || n > b.remaining {
		return geom.ErrConvergence
	}
	b.remaining -= n
	b.used += n
	return nil
}
func (b *pairedTerminalBudget) rootOptions() (geom.ExactCurvePlaneEventOptions, error) {
	if b.remaining < 1 {
		return geom.ExactCurvePlaneEventOptions{}, geom.ErrConvergence
	}
	return geom.ExactCurvePlaneEventOptions{MaxOperations: b.remaining, MaxCoefficientBits: b.options.MaxCoefficientBits, MaxSubdivisions: b.options.MaxSubdivisions}, nil
}
func (b *pairedTerminalBudget) endpointOptions() (geom.TubeAlgebraicGraphEndpointOptions, error) {
	if b.remaining < 1 {
		return geom.TubeAlgebraicGraphEndpointOptions{}, geom.ErrConvergence
	}
	o := b.options
	o.MaxOperations = b.remaining
	return o, nil
}
func (b *pairedTerminalBudget) compareRational(ctx context.Context, r *geom.ExactAlgebraicRoot, x *big.Rat) (int, error) {
	if r == nil {
		return 0, geom.ErrInvalid
	}
	o, e := b.rootOptions()
	if e != nil {
		return 0, e
	}
	bound, n, e := geom.NewExactRationalRoot(ctx, x, o)
	if e != nil {
		return 0, e
	}
	if e = b.debit(n); e != nil {
		return 0, e
	}
	o, e = b.rootOptions()
	if e != nil {
		return 0, e
	}
	c, e := r.Compare(ctx, bound, o)
	if e != nil {
		return 0, e
	}
	if e = b.debit(c.Operations); e != nil {
		return 0, e
	}
	if !c.Proven {
		return 0, geom.ErrConvergence
	}
	return c.Order, nil
}

// DiscoverAlgebraicSplineTerminalPair is additive to the strict single-owner
// terminal API. Both charts must have exactly one boundary coordinate, with
// the varying coordinate strictly interior. Corner and one-owner cases remain
// unsupported. Work is cumulative, including both source-edge comparisons.
func DiscoverAlgebraicSplineTerminalPair(ctx context.Context, segment intersect.TubeSegment, end int, a, b intersect.Patch, o geom.TubeAlgebraicGraphEndpointOptions) (*AlgebraicSplineTerminalPair, error) {
	if ctx == nil || end < 0 || end > 1 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if e := checkAlgebraicTerminalCharts(segment, a, b); e != nil {
		return nil, e
	}
	budget, e := newPairedTerminalBudget(o)
	if e != nil {
		return nil, e
	}
	child, e := budget.endpointOptions()
	if e != nil {
		return nil, e
	}
	q, e := discoverExactTubeSectionEndpoint(ctx, segment, end, a, b, child)
	if e != nil {
		return nil, e
	}
	if e = budget.debit(q.ExactOperations); e != nil {
		return nil, e
	}
	result, e := certifyPairedTerminal(ctx, q, a, b, budget)
	if e == nil {
		result.work = budget.used
	}
	return result, e
}
func certifyPairedTerminal(ctx context.Context, q geom.TubeAlgebraicGraphEndpointCertificate, a, b intersect.Patch, budget *pairedTerminalBudget) (*AlgebraicSplineTerminalPair, error) {
	if !q.Proven || !q.SourceIsoTransverse || q.ModelEvent == nil {
		return nil, geom.ErrConvergence
	}
	startWork := budget.used
	result := &AlgebraicSplineTerminalPair{event: q.ModelEvent}
	for operand, p := range [2]intersect.Patch{a, b} {
		bounds := [2][2]float64{{p.Min.X, p.Max.X}, {p.Min.Y, p.Max.Y}}
		side := -1
		for axis := 0; axis < 2; axis++ {
			for end, value := range bounds[axis] {
				order, e := budget.compareRational(ctx, q.Parameters[2*operand+axis], ratInset(value))
				if e != nil {
					return nil, e
				}
				if order == 0 {
					if side >= 0 {
						return nil, geom.ErrConvergence
					}
					side = 2*axis + end
				} else if (end == 0 && order < 0) || (end == 1 && order > 0) {
					return nil, geom.ErrDomain
				}
			}
		}
		if side < 0 {
			return nil, geom.ErrConvergence
		}
		axis := side / 2
		vary := 1 - axis
		r := q.Parameters[2*operand+vary]
		lo, hi := ratInset(bounds[vary][0]), ratInset(bounds[vary][1])
		width := new(big.Rat).Sub(hi, lo)
		opts, e := budget.rootOptions()
		if e != nil {
			return nil, e
		}
		normalized, n, e := r.AffineMap(ctx, new(big.Rat).Neg(new(big.Rat).Quo(lo, width)), new(big.Rat).Inv(width), opts)
		if e != nil {
			return nil, e
		}
		if e = budget.debit(n); e != nil {
			return nil, e
		}
		child, e := budget.endpointOptions()
		if e != nil {
			return nil, e
		}
		patch, e := exactSplineSupportPatch(p.Surface, child.SurfacePatchOptions)
		if e != nil {
			return nil, e
		}
		if e = budget.debit(patch.ExactOperations()); e != nil {
			return nil, e
		}
		var controls [][4]*big.Rat
		for _, v := range []*big.Rat{lo, hi} {
			uv := [2]*big.Rat{}
			uv[axis] = ratInset(bounds[axis][side%2])
			uv[vary] = v
			controls = append(controls, [4]*big.Rat{uv[0], uv[1], new(big.Rat), big.NewRat(1, 1)})
		}
		child, e = budget.endpointOptions()
		if e != nil {
			return nil, e
		}
		curve, e := geom.NewExactHomogeneousBezier(ctx, controls, child.SurfacePatchOptions)
		if e != nil {
			return nil, e
		}
		if e = budget.debit(curve.ExactOperations()); e != nil {
			return nil, e
		}
		child, e = budget.endpointOptions()
		if e != nil {
			return nil, e
		}
		cells, e := patch.BuildSurfaceCurveCompositionCells(ctx, curve, child.SurfaceCurveAgreementOptions)
		if e != nil {
			return nil, e
		}
		if e = budget.debit(cells.ExactOperations); e != nil {
			return nil, e
		}
		if !cells.Complete {
			return nil, geom.ErrConvergence
		}
		var event *geom.ExactComposedCurveEvent
		for _, cell := range cells.Cells {
			opts, e = budget.rootOptions()
			if e != nil {
				return nil, e
			}
			// BindRoot does not publish its operation count. Reserve its complete
			// bounded allowance even when this cell excludes the parameter.
			if opts.MaxOperations > 100000 {
				opts.MaxOperations = 100000
			}
			if e = budget.debit(opts.MaxOperations); e != nil {
				return nil, e
			}
			candidate, err := cell.BindRoot(ctx, normalized, opts)
			if errors.Is(err, geom.ErrDomain) {
				continue
			}
			if err != nil {
				return nil, err
			}
			opts, e = budget.rootOptions()
			if e != nil {
				return nil, e
			}
			equal, n, err := candidate.CompareModelPoint(ctx, q.ModelEvent, opts)
			if err != nil {
				return nil, err
			}
			if e = budget.debit(n); e != nil {
				return nil, e
			}
			if !equal {
				return nil, geom.ErrConvergence
			}
			event = candidate
		}
		if event == nil {
			return nil, geom.ErrConvergence
		}
		result.owners[operand] = AlgebraicSplineBoundaryOwner{Operand: operand, Side: geom.SurfaceEdge(side), Parameter: r, EdgeParameter: normalized, ModelEvent: event}
	}
	result.work = budget.used - startWork
	return result, nil
}

// AlgebraicSplineOwnerBranch retains two independently bound original edge
// occurrences at each end, and exact equality of every internal junction.
type AlgebraicSplineOwnerBranch struct {
	Ends      [2]*AlgebraicSplineTerminalPair
	Junctions []intersect.TubeAlgebraicEndpointComparison
	Segments  int
}

// AlgebraicSplineOwnerSection is a complete face-pair ledger, not a shell
// imprint or material selection. Closed branches retain the trace proof; all
// open branches require the paired noncorner ownership domain above.
type AlgebraicSplineOwnerSection struct {
	Trace           intersect.SplinePairTrace
	Open            []AlgebraicSplineOwnerBranch
	Closed          int
	Complete        bool
	ExactOperations int
}

// SectionSplinePairAlgebraicOwners independently certifies original-pair
// coverage, all junctions and both original edge occurrences at each open end.
// Endpoint proof work is cumulative across the entire ledger. Trace discovery
// and coverage retain their own explicit stage limits. Failure is atomic.
func SectionSplinePairAlgebraicOwners(ctx context.Context, a, b intersect.Patch, seeds *intersect.SplineSeedReport, trace intersect.SplinePairTraceOptions, o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplineOwnerSection, error) {
	empty := AlgebraicSplineOwnerSection{}
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	budget, e := newPairedTerminalBudget(o)
	if e != nil {
		return empty, e
	}
	var result AlgebraicSplineOwnerSection
	if seeds == nil {
		result.Trace, e = intersect.TraceSplinePair(ctx, a, b, trace)
	} else {
		result.Trace, e = intersect.CertifySplinePairBranches(ctx, a, b, *seeds, trace)
	}
	if e != nil {
		return empty, e
	}
	if !result.Trace.Complete || len(result.Trace.Curves) != len(result.Trace.BranchGraph.Branches) || len(result.Trace.Curves) > 256 {
		return empty, geom.ErrConvergence
	}
	total := 0
	for _, curve := range result.Trace.Curves {
		total += len(curve)
		if total > 4096 {
			return empty, geom.ErrConvergence
		}
	}
	for branch, segments := range result.Trace.Curves {
		if len(segments) == 0 {
			return empty, geom.ErrConvergence
		}
		if result.Trace.BranchGraph.Branches[branch].Closed {
			result.Closed++
			continue
		}
		var endpoints [][2]geom.TubeAlgebraicGraphEndpointCertificate
		for _, seg := range segments {
			if e = checkAlgebraicTerminalCharts(seg, a, b); e != nil {
				return empty, e
			}
			var ends [2]geom.TubeAlgebraicGraphEndpointCertificate
			for end := range ends {
				child, err := budget.endpointOptions()
				if err != nil {
					return empty, err
				}
				q, err := discoverExactTubeSectionEndpoint(ctx, seg, end, a, b, child)
				if err != nil {
					return empty, err
				}
				if !q.Proven {
					return empty, geom.ErrConvergence
				}
				if e = budget.debit(q.ExactOperations); e != nil {
					return empty, e
				}
				ends[end] = q
			}
			endpoints = append(endpoints, ends)
		}
		row := AlgebraicSplineOwnerBranch{Segments: len(segments)}
		for end, q := range []geom.TubeAlgebraicGraphEndpointCertificate{endpoints[0][0], endpoints[len(endpoints)-1][1]} {
			row.Ends[end], e = certifyPairedTerminal(ctx, q, a, b, budget)
			if e != nil {
				return empty, e
			}
		}
		for i := 1; i < len(endpoints); i++ {
			x, y := endpoints[i-1][1], endpoints[i][0]
			junction := intersect.TubeAlgebraicEndpointComparison{SupportsEqual: true, Left: x, Right: y}
			for axis := range x.Parameters {
				opts, err := budget.rootOptions()
				if err != nil {
					return empty, err
				}
				c, err := x.Parameters[axis].Compare(ctx, y.Parameters[axis], opts)
				if err != nil {
					return empty, err
				}
				if e = budget.debit(c.Operations); e != nil {
					return empty, e
				}
				if !c.Proven || c.Order != 0 {
					return empty, geom.ErrConvergence
				}
				junction.ExactOperations += c.Operations
			}
			junction.ProvenEqual = true
			row.Junctions = append(row.Junctions, junction)
		}
		result.Open = append(result.Open, row)
	}
	result.ExactOperations = budget.used
	result.Complete = true
	return result, nil
}
