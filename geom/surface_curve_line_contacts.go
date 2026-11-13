package geom

import (
	"context"
	"math/big"
)

// ComposedLineContactsOptions bounds cells/events and each exact proof stage.
// LineOptions limits each isolation, comparison and coordinate-image proof.
type ComposedLineContactsOptions struct {
	LineOptions         ComposedCurveLineOptions
	MaxCells, MaxEvents int
}

// ComposedLineContact retains both exact parameter identities. LineRoot is in
// origin+t*direction units, not curve units. Cells lists incident original cells.
// BoundaryMask 1/2 denotes exact equality to the lower/upper finite line limit.
type ComposedLineContact struct {
	Event        *ExactComposedPlaneEvent
	LineRoot     *ExactAlgebraicRoot
	LineBounds   ExactParameterInterval
	BoundaryMask uint8
	Cells        []int
}

// ComposedLineContactTable covers exactly Domain and CellCount supplied cells,
// not an omitted parent curve or a periodic quotient. Contacts are ordered by
// original curve parameter, not line parameter; both identities are retained.
type ComposedLineContactTable struct {
	Complete  bool
	Domain    [2]*ExactAlgebraicRoot
	CellCount int
	Contacts  []ComposedLineContact
}

// DiscoverLineContacts proves isolated finite line contacts with exact seam
// identity. Supporting-line-coincident cells reject atomically, even if their
// finite overlap is empty. CoordinateRoot's bounded algebraic image domain
// (currently defining root degree <=4) applies; unsupported images never become
// approximate identities. No tangent continuity or manifold status is inferred.
func (c SurfaceCurveCompositionCells) DiscoverLineContacts(ctx context.Context, origin, direction [3]*big.Rat, limits [2]*big.Rat, o ComposedLineContactsOptions) (ComposedLineContactTable, error) {
	var zero ComposedLineContactTable
	if ctx == nil || !c.Complete || len(c.Cells) == 0 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxCells == 0 {
		o.MaxCells = 64
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 || o.MaxEvents < 1 || o.MaxEvents > 65536 {
		return zero, ErrInvalid
	}
	if len(c.Cells) > o.MaxCells {
		return zero, ErrConvergence
	}
	opts, e := graphRootOptions(o.LineOptions.ExactCurvePlaneEventOptions)
	if e != nil {
		return zero, e
	}
	pivot := -1
	for i := 0; i < 3; i++ {
		if origin[i] == nil || direction[i] == nil {
			return zero, ErrInvalid
		}
		if direction[i].Sign() != 0 {
			pivot = i
		}
	}
	if pivot < 0 {
		return zero, ErrDegenerate
	}
	if limits[0] == nil || limits[1] == nil || limits[0].Cmp(limits[1]) > 0 {
		return zero, ErrInvalid
	}
	inputBudget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, values := range [][]*big.Rat{origin[:], direction[:], limits[:]} {
		for _, v := range values {
			if e := inputBudget.check(v); e != nil {
				return zero, e
			}
		}
	}
	if e = c.validateContactSeams(ctx, opts); e != nil {
		return zero, e
	}
	offset := new(big.Rat).Quo(new(big.Rat).Neg(origin[pivot]), direction[pivot])
	scale := new(big.Rat).Inv(direction[pivot])
	out := ComposedLineContactTable{}
	count := 0
	for i, cell := range c.Cells {
		hits, e := cell.IntersectRationalFiniteLine(ctx, origin, direction, limits, o.LineOptions)
		if e != nil {
			return zero, e
		}
		if !hits.Complete || hits.Coincident || len(hits.Portions) != 0 {
			return zero, ErrConvergence
		}
		count += len(hits.Hits)
		if count > o.MaxEvents {
			return zero, ErrConvergence
		}
		for _, hit := range hits.Hits {
			coordinate, _, e := hit.Event.CoordinateRoot(ctx, pivot, opts)
			if e != nil {
				return zero, e
			}
			lineRoot, _, e := coordinate.AffineMap(ctx, offset, scale, opts)
			if e != nil {
				return zero, e
			}
			if len(out.Contacts) > 0 {
				prev := &out.Contacts[len(out.Contacts)-1]
				q, e := prev.Event.CompareParameter(ctx, hit.Event, opts)
				if e != nil {
					return zero, e
				}
				if !q.Proven || q.Order > 0 {
					return zero, ErrConvergence
				}
				if q.Order == 0 {
					if len(prev.Cells) != 1 || prev.Cells[0] != i-1 || prev.Event.EndpointMask() != 2 || hit.Event.EndpointMask() != 1 || prev.BoundaryMask != hit.BoundaryMask {
						return zero, ErrDomain
					}
					same, e := prev.LineRoot.Compare(ctx, lineRoot, opts)
					if e != nil {
						return zero, e
					}
					if !same.Proven || same.Order != 0 {
						return zero, ErrDomain
					}
					prev.Cells = append(prev.Cells, i)
					continue
				}
			}
			out.Contacts = append(out.Contacts, ComposedLineContact{Event: hit.Event, LineRoot: lineRoot, LineBounds: hit.LineParameter, BoundaryMask: hit.BoundaryMask, Cells: []int{i}})
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Domain[0], _ = c.Cells[0].EndpointRoot(0)
	out.Domain[1], _ = c.Cells[len(c.Cells)-1].EndpointRoot(1)
	out.CellCount = len(c.Cells)
	out.Complete = true
	return out, nil
}

// DiscoverLineContacts exposes this exact finite-line table for a standalone
// rational model edge. Extraction retains original knot parameter units.
func (c *SurfaceIsoCurve) DiscoverLineContacts(ctx context.Context, origin, direction [3]*big.Rat, limits [2]*big.Rat, o ComposedLineContactsOptions) (ComposedLineContactTable, error) {
	var zero ComposedLineContactTable
	opts, e := graphRootOptions(o.LineOptions.ExactCurvePlaneEventOptions)
	if e != nil {
		return zero, e
	}
	cells, e := c.ModelContactCells(ctx, SurfacePatchOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}, o.MaxCells)
	if e != nil {
		return zero, e
	}
	return cells.DiscoverLineContacts(ctx, origin, direction, limits, o)
}
