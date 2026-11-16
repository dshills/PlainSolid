package geom

import (
	"context"
	"errors"
	"math/big"
)

// PeriodicSurfaceLineOptions bounds both exact knot extraction and regular
// line contact isolation. MaxSearchCells defaults to 4096 and is cumulative
// across the knot cells. MaxDepth defaults to 16 and is measured within each
// knot cell. MaxOperations defaults to 4M across all stages.
type PeriodicSurfaceLineOptions struct {
	PeriodicSurfaceBezierOptions
	MaxDepth, MaxSearchCells int
}

// PeriodicSurfaceLineWindow contains exact physical UV and line parameter
// intervals. All three coordinates refer to the original inputs.
type PeriodicSurfaceLineWindow struct {
	U, V, Parameter ExactParameterInterval
}

// PeriodicSurfaceLineHit encloses exactly one certified contact. Owners are
// the original surface knot incidences. A contact on a shared knot may
// appear under both original owners.
type PeriodicSurfaceLineHit struct {
	U, V      ExactSurfaceKnotSpan
	Isolation PeriodicSurfaceLineWindow
	Bounds    Box
}

// PeriodicSurfaceLinePending encloses an unresolved candidate. When the grid
// could not be built the owners have Span=-1 and the window retains the
// complete original rectangle and line parameter interval.
type PeriodicSurfaceLinePending struct {
	U, V   ExactSurfaceKnotSpan
	Window PeriodicSurfaceLineWindow
	Depth  int
	Reason string
}

// PeriodicSurfaceLineIntersection is exhaustive when Complete. Pending
// retains unprocessed cells and possible tangent, coincident or unresolved
// boundary contacts. Bounds in a result are owned and independently mutable.
type PeriodicSurfaceLineIntersection struct {
	Hits                          []PeriodicSurfaceLineHit
	Pending                       []PeriodicSurfaceLinePending
	VisitedCells, ExactOperations int
	Complete                      bool
}

// IntersectRationalLinePeriodicSurface isolates contacts of the exact
// origin+t*direction in a closed rational parameter window. Surface and line
// loci are kept exact while solving in unit working coordinates.
func IntersectRationalLinePeriodicSurface(ctx context.Context, origin, direction [3]*big.Rat, surface *PeriodicBSplineSurface, u, v, parameter ExactParameterInterval, options PeriodicSurfaceLineOptions) (result PeriodicSurfaceLineIntersection, err error) {
	if surface == nil || surface.surface == nil {
		return result, ErrInvalid
	}
	return intersectRationalLinePeriodicSurface(ctx, origin, direction, surface, u, v, parameter, options)
}

// IntersectRationalLine isolates contacts of origin+t*direction against the
// exact nonuniform tensor. Contacts at an edited station retain distinct
// owners, and unresolved contacts remain in the pending list.
func (s *ExactPeriodicSplineSurface) IntersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat,
	u, v, parameter ExactParameterInterval, options PeriodicSurfaceLineOptions) (result PeriodicSurfaceLineIntersection, err error) {
	if s == nil || len(s.net) == 0 {
		return result, ErrInvalid
	}
	return intersectRationalLinePeriodicSurface(ctx, origin, direction, s, u, v, parameter, options)
}

func intersectRationalLinePeriodicSurface(ctx context.Context, origin, direction [3]*big.Rat, surface exactPeriodicCellSource, u, v, parameter ExactParameterInterval, options PeriodicSurfaceLineOptions) (result PeriodicSurfaceLineIntersection, err error) {
	if ctx == nil || origin[0] == nil || origin[1] == nil || origin[2] == nil || !validRationalLineDirection(direction) || options.MaxDepth < 0 || options.MaxDepth > 128 || options.MaxSearchCells < 0 || options.MaxCells < 0 {
		return result, ErrInvalid
	}
	if options.MaxOperations == 0 {
		options.MaxOperations = 4000000
	}
	if options.MaxSearchCells == 0 {
		options.MaxSearchCells = 4096
	}
	limits, err := patchOptions(options.SurfacePatchOptions)
	if err != nil {
		return result, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	for _, span := range []ExactParameterInterval{u, v, parameter} {
		if span.Lower == nil || span.Upper == nil {
			return result, ErrInvalid
		}
		for _, endpoint := range []*big.Rat{span.Lower, span.Upper} {
			if err := b.check(endpoint); err != nil {
				return result, err
			}
		}
		if span.Lower.Cmp(span.Upper) >= 0 {
			return result, ErrInvalid
		}
	}
	if err = ctx.Err(); err != nil {
		result.Pending = []PeriodicSurfaceLinePending{unprocessedPeriodicLineWindow(u, v, parameter, err.Error())}
		return result, err
	}
	start, delta, err := unitRationalLine(origin, direction, parameter, b)
	if err != nil {
		result.Pending = []PeriodicSurfaceLinePending{unprocessedPeriodicLineWindow(u, v, parameter, err.Error())}
		return result, err
	}
	limits.MaxOperations -= b.operations
	if limits.MaxOperations < 1 {
		result.Pending = []PeriodicSurfaceLinePending{unprocessedPeriodicLineWindow(u, v, parameter, "work limit")}
		return result, ErrConvergence
	}
	grid, work, err := surface.ExactBezierCells(ctx, u, v, PeriodicSurfaceBezierOptions{SurfacePatchOptions: limits, MaxCells: options.MaxCells})
	b.operations += work
	if err != nil {
		if !errors.Is(err, ErrDomain) && !errors.Is(err, ErrInvalid) {
			result.Pending = []PeriodicSurfaceLinePending{unprocessedPeriodicLineWindow(u, v, parameter, err.Error())}
		}
		return result, err
	}
	fail := func(first int, reason error) (PeriodicSurfaceLineIntersection, error) {
		for _, pending := range grid[first:] {
			u, v := cloneSurfaceKnotSpan(pending.U), cloneSurfaceKnotSpan(pending.V)
			result.Pending = append(result.Pending, PeriodicSurfaceLinePending{
				U: u, V: v, Window: copiedPeriodicLineWindow(u.Domain, v.Domain, parameter), Reason: reason.Error(),
			})
		}
		return result, reason
	}
	for i, cell := range grid {
		search := LineSplineSurfaceOptions{
			MinParameter: 0, MaxParameter: 1, MaxDepth: options.MaxDepth,
			MaxCells: options.MaxSearchCells - result.VisitedCells, MaxOperations: b.maxOperations - b.operations,
			MaxCoefficientBits: limits.MaxCoefficientBits,
		}
		if err = ctx.Err(); err != nil {
			return fail(i, err)
		}
		if search.MaxCells < 1 || search.MaxOperations < 1 {
			return fail(i, ErrConvergence)
		}
		limits.MaxOperations = search.MaxOperations
		patch, err := cell.NormalizedPatch(ctx, limits)
		if err != nil {
			return fail(i, err)
		}
		used := patch.ExactOperations()
		b.operations += used
		search.MaxOperations -= used
		if search.MaxOperations < 1 {
			return fail(i, ErrConvergence)
		}
		previousHits, previousPending := len(result.Hits), len(result.Pending)
		contacts, err := IntersectRationalLineSplineSurface(ctx, start, delta, patch, search)
		b.operations += contacts.ExactOperations
		result.VisitedCells += contacts.VisitedCells
		if mapError := mapPeriodicSurfaceLineContacts(&result, contacts, cell, parameter, b); mapError != nil {
			result.Hits = result.Hits[:previousHits]
			result.Pending = result.Pending[:previousPending]
			if err != nil && !errors.Is(err, ErrConvergence) {
				return fail(i, err)
			}
			return fail(i, mapError)
		}
		if err != nil {
			if len(contacts.Pending) == 0 {
				return fail(i, err)
			}
			if !errors.Is(err, ErrConvergence) {
				return fail(i+1, err)
			}
		}
	}
	result.Complete = len(result.Pending) == 0
	if !result.Complete {
		return result, ErrConvergence
	}
	return result, nil
}

func unitRationalLine(origin, direction [3]*big.Rat, span ExactParameterInterval, b *polynomialBudget) (start, delta [3]*big.Rat, err error) {
	width := new(big.Rat).Sub(span.Upper, span.Lower)
	for axis := range origin {
		for _, x := range []*big.Rat{origin[axis], direction[axis]} {
			if err := b.check(x); err != nil {
				return start, delta, err
			}
		}
		start[axis] = new(big.Rat).Add(origin[axis], new(big.Rat).Mul(direction[axis], span.Lower))
		delta[axis] = new(big.Rat).Mul(direction[axis], width)
		for _, x := range []*big.Rat{start[axis], delta[axis]} {
			if err := b.check(x); err != nil {
				return start, delta, err
			}
		}
	}
	return start, delta, nil
}

func unprocessedPeriodicLineWindow(u, v, parameter ExactParameterInterval, reason string) PeriodicSurfaceLinePending {
	window := copiedPeriodicLineWindow(u, v, parameter)
	return PeriodicSurfaceLinePending{
		U: cloneSurfaceKnotSpan(ExactSurfaceKnotSpan{Domain: u, Span: -1}),
		V: cloneSurfaceKnotSpan(ExactSurfaceKnotSpan{Domain: v, Span: -1}), Window: window, Reason: reason,
	}
}

func copiedPeriodicLineWindow(u, v, parameter ExactParameterInterval) PeriodicSurfaceLineWindow {
	copy := func(i ExactParameterInterval) ExactParameterInterval {
		return ExactParameterInterval{Lower: new(big.Rat).Set(i.Lower), Upper: new(big.Rat).Set(i.Upper)}
	}
	return PeriodicSurfaceLineWindow{U: copy(u), V: copy(v), Parameter: copy(parameter)}
}

func mapPeriodicSurfaceLineContacts(result *PeriodicSurfaceLineIntersection, contacts LineSplineSurfaceResult, cell ExactPeriodicSurfaceBezierCell, parameter ExactParameterInterval, b *polynomialBudget) error {
	mapWindow := func(part ParameterRectangle, line ParameterSpan) (PeriodicSurfaceLineWindow, error) {
		var out PeriodicSurfaceLineWindow
		for _, axis := range []struct {
			out      *ExactParameterInterval
			domain   ExactParameterInterval
			min, max float64
		}{
			{&out.U, cell.U.Domain, part.UMin, part.UMax}, {&out.V, cell.V.Domain, part.VMin, part.VMax},
			{&out.Parameter, parameter, line.Start, line.End},
		} {
			width := new(big.Rat).Sub(axis.domain.Upper, axis.domain.Lower)
			low := new(big.Rat).SetFloat64(axis.min)
			high := new(big.Rat).SetFloat64(axis.max)
			if low == nil || high == nil || low.Cmp(high) > 0 {
				return out, ErrConvergence
			}
			axis.out.Lower = low.Add(axis.domain.Lower, low.Mul(low, width))
			axis.out.Upper = high.Add(axis.domain.Lower, high.Mul(high, width))
			for _, x := range []*big.Rat{axis.out.Lower, axis.out.Upper} {
				if err := b.check(x); err != nil {
					return out, err
				}
			}
		}
		return out, nil
	}
	for _, hit := range contacts.Hits {
		uv := ParameterRectangle{UMin: hit.UV.Min.X, UMax: hit.UV.Max.X, VMin: hit.UV.Min.Y, VMax: hit.UV.Max.Y}
		window, err := mapWindow(uv, ParameterSpan{Start: hit.ParameterMin, End: hit.ParameterMax})
		if err != nil {
			return err
		}
		result.Hits = append(result.Hits, PeriodicSurfaceLineHit{U: cloneSurfaceKnotSpan(cell.U), V: cloneSurfaceKnotSpan(cell.V), Isolation: window, Bounds: hit.Bounds})
	}
	for _, pending := range contacts.Pending {
		window, err := mapWindow(pending.Cell, pending.LineRange)
		if err != nil {
			return err
		}
		result.Pending = append(result.Pending, PeriodicSurfaceLinePending{
			U: cloneSurfaceKnotSpan(cell.U), V: cloneSurfaceKnotSpan(cell.V), Window: window, Depth: pending.Depth, Reason: pending.Reason,
		})
	}
	return nil
}
