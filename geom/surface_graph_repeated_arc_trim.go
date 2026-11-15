package geom

import (
	"context"
	"math/big"
)

type repeatedSplineGraphArcSource struct {
	curve  *ExactSplineCurve
	free   int
	domain [2]float64
}

// RepeatedSplineArcTrimHit carries the exact normalized arc root and an
// independent event in the original trim parameter. SideSigns describe trim
// minus arc in the dependent chart coordinate, before and after the event.
type RepeatedSplineArcTrimHit struct {
	ArcParameter *ExactAlgebraicRoot
	TrimEvent    *ExactComposedCurveEvent
	SideSigns    [2]int
}

type RepeatedSplineArcTrimIntersection struct {
	Complete                      bool
	Hits                          []RepeatedSplineArcTrimHit
	Coincident                    []ComposedCurvePlaneIntersection
	ExactOperations, Subdivisions int
}

// IntersectUVTrim intersects the proved rational arc against a positive-weight
// original UV trim. The trim has zero Z. Chart coordinates and source trim
// parameters are preserved, and no approximate root is used as a cut.
// Positive-length coincidences retain their exact interval and fail closed.
func (r SplineGraphRepeatedArc) IntersectUVTrim(ctx context.Context, trim *ExactSplineCurve,
	o SurfaceCurveAgreementOptions) (out RepeatedSplineArcTrimIntersection, err error) {
	if ctx == nil || r.source == nil || trim == nil || trim.patch == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o.SurfacePatchOptions, err = patchOptions(o.SurfacePatchOptions)
	if err != nil {
		return out, err
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxEvents < 2 || o.MaxSubdivisions < 1 {
		return out, ErrInvalid
	}
	if len(trim.patch.net) > o.MaxPoles {
		return out, ErrConvergence
	}
	for _, h := range trim.patch.net {
		if h[2].Sign() != 0 {
			return out, ErrDomain
		}
	}
	remaining := func() ExactCurvePlaneEventOptions {
		return ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations - out.ExactOperations,
			MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions - out.Subdivisions}
	}
	// Extruding the exact UV graph along Z gives an affine (free,Z)
	// projection with the other UV as the graph height. The trim's Z=0
	// slice then uses the ordinary original-parameter contact proof.
	s := r.source
	curve := s.curve.patch
	controls := make([][4]*big.Rat, 0, 2*curve.countU)
	for i := 0; i < curve.countU; i++ {
		h := curve.net[i*curve.countV]
		controls = append(controls, [4]*big.Rat{h[0], h[1], new(big.Rat).Neg(h[3]), h[3]}, [4]*big.Rat{h[0], h[1], h[3], h[3]})
	}
	graph, e := NewExactHomogeneousBezierSurface(ctx, curve.degreeU, 1, controls, o.SurfacePatchOptions)
	if e != nil {
		return out, e
	}
	out.ExactOperations += graph.ExactOperations()
	p := o.SurfacePatchOptions
	p.MaxOperations = remaining().MaxOperations
	if p.MaxOperations < 1 {
		return out, ErrConvergence
	}
	trimCells, e := trim.ModelContactCells(ctx, p, min(o.MaxEvents, 256))
	out.ExactOperations += trimCells.ExactOperations
	if e != nil {
		return out, e
	}
	if !trimCells.Complete {
		return out, ErrConvergence
	}
	lo, hi := new(big.Rat).SetFloat64(s.domain[0]), new(big.Rat).SetFloat64(s.domain[1])
	scale := new(big.Rat).Inv(new(big.Rat).Sub(hi, lo))
	offset := new(big.Rat).Neg(new(big.Rat).Mul(lo, scale))
	for _, cell := range trimCells.Cells {
		child := o
		child.MaxOperations = remaining().MaxOperations
		child.MaxSubdivisions = remaining().MaxSubdivisions
		child.MaxEvents -= len(out.Hits) + len(out.Coincident)
		if child.MaxOperations < 1 || child.MaxSubdivisions < 1 || child.MaxEvents < 2 {
			return out, ErrConvergence
		}
		contact, e := IntersectExactComposedCurveProjectedGraph(ctx, cell, graph, [2]int{s.free, 2}, child)
		out.ExactOperations += contact.ExactOperations
		out.Subdivisions += contact.Subdivisions
		if e != nil {
			return out, e
		}
		if !contact.Complete {
			return out, ErrConvergence
		}
		for _, piece := range contact.Cells {
			if piece.Intersection.Coincident {
				out.Coincident = append(out.Coincident, piece.Intersection)
				continue
			}
			for i, event := range piece.Intersection.Events {
				p := remaining()
				if p.MaxOperations < 1 || p.MaxSubdivisions < 1 || len(out.Hits)+len(out.Coincident) >= o.MaxEvents {
					return out, ErrConvergence
				}
				coordinate, used, subdivisions, e := event.CoordinateRootWithWork(ctx, s.free, p)
				out.ExactOperations += used
				out.Subdivisions += subdivisions
				if e != nil {
					return out, e
				}
				p = remaining()
				if p.MaxOperations < 1 || p.MaxSubdivisions < 1 {
					return out, ErrConvergence
				}
				parameter, used, e := coordinate.AffineMap(ctx, offset, scale, p)
				out.ExactOperations += used
				if e != nil {
					return out, e
				}
				hit := RepeatedSplineArcTrimHit{ArcParameter: parameter, TrimEvent: event}
				if i < len(piece.EventSigns) {
					hit.SideSigns = piece.EventSigns[i]
				}
				out.Hits = append(out.Hits, hit)
			}
		}
	}
	if len(out.Coincident) != 0 {
		return out, ErrConvergence
	}
	out.Complete = true
	return out, nil
}
