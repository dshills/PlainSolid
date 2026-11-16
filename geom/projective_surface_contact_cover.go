package geom

import (
	"context"
	"errors"
	"math/big"
)

// ProjectiveSurfaceContactCoverOptions bounds common height construction
// and local contact isolation. MaxExactOperations is shared across the
// whole query, defaulting to sixteen million exact operations.
type ProjectiveSurfaceContactCoverOptions struct {
	Atlas              GeneralSurfaceHeightOptions
	Arcs               SurfacePlaneArcOptions
	MaxExactOperations int
	OriginalCuts       ProjectiveSurfaceOriginalCuts
	// OriginalHoles excludes the open interiors of verified rectangles
	// from each owner's raw chart. Relevant holes must be strictly inside
	// the common projection and their closed images must be disjoint.
	OriginalHoles [2][]ExactSurfaceRectangle
	// IsolateQuadraticSaddles may replace regular isolation in cells with a
	// complete four-ray saddle proof. All other cells still use Arcs limits.
	IsolateQuadraticSaddles bool
}

// ProjectiveSurfaceContactCover covers a positive-area contact projection
// with local exact arcs, tangent points and coincident cells. Isolations
// correspond to Atlas.Cells. Each uses that residual cell's unit square;
// Common and OriginalCells map the answer back to both original owners.
// If a corresponding Saddles entry is proven, it replaces that cell's
// ordinary isolation. Saddles is nil unless the option was requested.
// Complete means all local cells are covered, without claiming global joins.
type ProjectiveSurfaceContactCover struct {
	Atlas           ProjectiveSurfaceHeightDifference
	Isolations      []SurfacePlaneArcs
	Saddles         []ProjectiveQuadraticSaddle
	ExcludedCells   []int
	CommonHoles     []ExactSurfaceRectangle
	IncompleteCells []int
	Complete        bool
	ExactOperations int
}

// BoundProjectiveSurfaceContacts discovers the complete local zero set of
// the original graph height differences wherever the existing plane-arc
// proof succeeds. Resource and unsupported singular cases retain both the
// exact atlas and the list of incomplete cells.
func BoundProjectiveSurfaceContacts(ctx context.Context, a, b *SurfacePatch, axes [2]int,
	o ProjectiveSurfaceContactCoverOptions) (r ProjectiveSurfaceContactCover, err error) {
	if ctx == nil || o.MaxExactOperations < 0 || o.Atlas.MaxOperations < 0 || o.Arcs.MaxOperations < 0 {
		return r, ErrInvalid
	}
	if o.MaxExactOperations == 0 {
		o.MaxExactOperations = 16_000_000
	}
	if o.Atlas.MaxOperations == 0 {
		o.Atlas.MaxOperations = 8_000_000
	}
	if o.Arcs.MaxOperations == 0 {
		o.Arcs.MaxOperations = 2_000_000
	}
	o.Atlas.MaxOperations = min(o.Atlas.MaxOperations, o.MaxExactOperations)
	cuts, err := projectiveHoleCuts(o.OriginalCuts, o.OriginalHoles)
	if err != nil {
		return r, err
	}
	r.Atlas, err = BuildProjectiveSurfaceHeightDifferenceWithOriginalCuts(ctx, a, b, axes, cuts, o.Atlas)
	r.ExactOperations = r.Atlas.ExactOperations
	if err != nil {
		return r, err
	}
	var excluded []bool
	if len(o.OriginalHoles[0])+len(o.OriginalHoles[1]) != 0 {
		bits := o.Arcs.MaxCoefficientBits
		if bits == 0 {
			bits = 16384
		}
		budget := &polynomialBudget{maxOperations: o.MaxExactOperations - r.ExactOperations, maxBits: bits}
		holes, mask, skip, err := projectiveContactHoleMask(ctx, r.Atlas, [2]*SurfacePatch{a, b}, o.OriginalHoles, budget)
		r.ExactOperations += budget.operations
		if err != nil {
			return r, err
		}
		r.CommonHoles, r.ExcludedCells, excluded = holes, skip, mask
	}
	r.Isolations = make([]SurfacePlaneArcs, len(r.Atlas.Cells))
	var saddleLimits ExactCurvePlaneEventOptions
	if o.IsolateQuadraticSaddles {
		saddleLimits, err = graphRootOptions(ExactCurvePlaneEventOptions{MaxOperations: o.Arcs.MaxOperations,
			MaxCoefficientBits: o.Arcs.MaxCoefficientBits, MaxSubdivisions: o.Arcs.MaxRootSubdivisions})
		if err != nil {
			return r, err
		}
		r.Saddles = make([]ProjectiveQuadraticSaddle, len(r.Atlas.Cells))
	}
	defer func() {
		for i, cell := range r.Isolations {
			if !cell.Complete && (excluded == nil || !excluded[i]) && (r.Saddles == nil || !r.Saddles[i].Proven) {
				r.IncompleteCells = append(r.IncompleteCells, i)
			}
		}
	}()
	plane, err := NewPlane(Vec3{}, Vec3{Z: 1})
	if err != nil {
		return r, err
	}
	projected := projectiveContactProjectionCertificate(ctx)
	all := true
	for i, cell := range r.Atlas.Cells {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if excluded != nil && excluded[i] {
			continue
		}
		left := o.MaxExactOperations - r.ExactOperations
		if left < 1 {
			return r, ErrConvergence
		}
		if o.IsolateQuadraticSaddles {
			limits := saddleLimits
			limits.MaxOperations = min(left, limits.MaxOperations)
			r.Saddles[i], err = certifyProjectiveQuadraticSaddleCell(ctx, cell, r.Atlas.Overlap, limits)
			r.ExactOperations += r.Saddles[i].Operations
			if err == nil && r.Saddles[i].Proven {
				continue
			}
			if err != nil && !errors.Is(err, ErrConvergence) {
				return r, err
			}
			left = o.MaxExactOperations - r.ExactOperations
			if left < 1 {
				return r, ErrConvergence
			}
		}
		arc := o.Arcs
		arc.MaxOperations = min(left, arc.MaxOperations)
		r.Isolations[i], err = certifySurfacePlaneArcsContext(ctx, cell.Difference, plane, arc, projected)
		r.ExactOperations += r.Isolations[i].ExactOperations
		if err != nil && !errors.Is(err, ErrConvergence) {
			return r, err
		}
		all = all && err == nil && r.Isolations[i].Complete
	}
	if !all {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}

// Where the interval quotient rule loses the sign to weight variation,
// prove the free projected coordinate across the entire original local
// patch. It is independent of the fixed axis, so its ordinary derivative
// is also its derivative along the implicit contact graph.
func projectiveContactProjectionCertificate(ctx context.Context) surfaceArcProjectionCertificate {
	return func(p *SurfacePatch, data surfaceArcData, fixed int, b *polynomialBudget) (SurfaceArcCertificate, bool, error) {
		normal := [2]exactInterval{data.du[4], data.dv[4]}[fixed]
		if normal.sign() == 0 {
			return SurfaceArcCertificate{}, false, nil
		}
		// The height atlas stores its two selected projections in the
		// residual patch's X and Y coordinates, for every source axis pair.
		proof, err := certifyProjectiveGraphProjection(ctx, p, [2]int{0, 1}, p.options.MaxPoles, b)
		if errors.Is(err, ErrDomain) {
			return SurfaceArcCertificate{}, false, nil
		}
		if err != nil {
			return SurfaceArcCertificate{}, false, err
		}
		free := 1 - fixed
		index := 0
		if proof[index].raw != free {
			index = 1
		}
		chart := proof[index]
		d := projectiveParameterIntervals(p.Domain())[free]
		a := boundedRatArithmetic{b: b}
		span := a.sub(chart.ends[1], chart.ends[0])
		first := a.quo(a.mul(span, chart.ratio), a.sub(d.hi, d.lo))
		last := a.quo(first, a.mul(chart.ratio, chart.ratio))
		if a.err != nil {
			return SurfaceArcCertificate{}, false, a.err
		}
		lo, hi := orderedProjectiveEnds([2]*big.Rat{first, last})
		return SurfaceArcCertificate{fixed, index, certifiedRange(normal), certifiedRange(exactInterval{lo, hi})}, true, nil
	}
}
