package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// RepeatedSplineArcTrimOptions bounds exact discovery and trimming of an
// original double-contact graph arc. The event budget includes discovery.
type RepeatedSplineArcTrimOptions struct {
	Contact     geom.GraphPointContactOptions
	Events      geom.ExactCurvePlaneEventOptions
	Loops       trimvalidate.Options
	MaxContacts int
}

type RepeatedSplineArcTrimContact struct {
	Face, Loop, Use, Edge, Span int
	Reversed                    bool
	// Event evaluates the original chart UV contact. ArcParameter uses
	// normalized arc units. CoedgeParameter encloses the original spline
	// knot parameter or the directed [0,1] parameter of a straight segment.
	Event                *geom.ExactComposedCurveEvent
	ArcParameter         *geom.ExactAlgebraicRoot
	ExactCoedgeParameter *geom.ExactAlgebraicRoot
	CoedgeParameter      geom.ExactParameterInterval
	CoedgeEndpointMask   uint8
}

type RepeatedSplineArcTrimNode struct {
	Parameter *geom.ExactAlgebraicRoot
	Contacts  [2][]RepeatedSplineArcTrimContact
	OnFaces   bool
	Isolated  bool
}

type RepeatedSplineArcTrimInterval struct {
	Start, End int
	Locations  [2]trimvalidate.Location
	Retained   bool
}

// RepeatedSplineArcTrimSection retains the original graphs and exact
// continuous model compositions of the full arc. The interval ledger
// identifies which parts are in the closure of both original faces.
type RepeatedSplineArcTrimSection struct {
	Complete                                bool
	Arc                                     geom.SplineGraphRepeatedArc
	Sources                                 [2]geom.Surface
	Model                                   [2]geom.SurfaceCurveCompositionCells
	Nodes                                   []RepeatedSplineArcTrimNode
	Intervals                               []RepeatedSplineArcTrimInterval
	ExactOperations, Subdivisions, LoopWork int
	native                                  *nativeRepeatedSplineArcSection
}

// SectionRepeatedSplineGraphArc proves a rational double-contact arc between
// two original common-XY graph faces. It exhausts supported straight and
// rational spline coedges and classifies intervals between their exact
// contacts. Positive-length overlap requires a separate coincidence proof.
func SectionRepeatedSplineGraphArc(ctx context.Context, faces [2]Face, dependent int,
	o RepeatedSplineArcTrimOptions) (r RepeatedSplineArcTrimSection, err error) {
	if ctx == nil || dependent < 0 || dependent > 1 || o.MaxContacts < 0 || o.MaxContacts > 16384 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxContacts == 0 {
		o.MaxContacts = 4096
	}
	if o.Events.MaxOperations == 0 {
		o.Events.MaxOperations = 8000000
	}
	if o.Events.MaxCoefficientBits == 0 {
		o.Events.MaxCoefficientBits = 16384
	}
	if o.Events.MaxSubdivisions == 0 {
		o.Events.MaxSubdivisions = 8192
	}
	if o.Events.MaxOperations < 1 || o.Events.MaxCoefficientBits < 64 || o.Events.MaxSubdivisions < 1 {
		return r, geom.ErrInvalid
	}
	o.Loops, err = o.Loops.WithDefaults()
	if err != nil {
		return r, err
	}
	defer func() {
		if err != nil {
			r = RepeatedSplineArcTrimSection{ExactOperations: r.ExactOperations, Subdivisions: r.Subdivisions, LoopWork: r.LoopWork}
		}
	}()
	chargeLoop := func(p trimvalidate.Report) {
		o.Loops.MaxWork -= p.Work
		o.Loops.MaxSubdivisions -= p.Subdivisions
		r.LoopWork += p.Work
	}
	remaining := func() geom.ExactCurvePlaneEventOptions {
		p := o.Events
		p.MaxOperations -= r.ExactOperations
		p.MaxSubdivisions -= r.Subdivisions
		return p
	}
	patchOptions := func() geom.SurfacePatchOptions {
		p := remaining()
		return geom.SurfacePatchOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits}
	}
	var supports [2]*geom.SurfacePatch
	for chart, f := range faces {
		if remaining().MaxOperations < 1 || o.Loops.MaxWork < 1 || o.Loops.MaxSubdivisions < 1 {
			return r, geom.ErrConvergence
		}
		p, e := exactFaceSupportPatch(f, patchOptions())
		if e != nil {
			return r, e
		}
		r.ExactOperations += p.ExactOperations()
		supports[chart], r.Sources[chart] = p, f.Surface
		validated, e := ValidateFaceTrimLoops(ctx, f, o.Loops)
		chargeLoop(validated)
		if e != nil {
			return r, e
		}
		if validated.Status != trimvalidate.Certified {
			return r, geom.ErrConvergence
		}
		d := p.Domain()
		bounds, e := faceUVBounds(f)
		if e != nil {
			return r, e
		}
		if bounds.UMin < d.UMin || bounds.UMax > d.UMax || bounds.VMin < d.VMin || bounds.VMax > d.VMax {
			return r, geom.ErrDomain
		}
	}
	p := o.Contact
	if p.MaxOperations == 0 || p.MaxOperations > remaining().MaxOperations {
		p.MaxOperations = remaining().MaxOperations
	}
	if p.MaxCoefficientBits == 0 || p.MaxCoefficientBits > o.Events.MaxCoefficientBits {
		p.MaxCoefficientBits = o.Events.MaxCoefficientBits
	}
	if p.MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	r.Arc, err = geom.DiscoverSplineGraphRepeatedArc(ctx, supports[0], supports[1], dependent, p)
	r.ExactOperations += r.Arc.ExactOperations
	if err != nil {
		return r, err
	}
	if !r.Arc.Proven || remaining().MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	uv, e := r.Arc.UVCurve.ModelContactCells(ctx, patchOptions(), 1)
	r.ExactOperations += uv.ExactOperations
	if e != nil {
		return r, e
	}
	if !uv.Complete || len(uv.Cells) != 1 {
		return r, geom.ErrConvergence
	}
	cell := uv.Cells[0]
	unit := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	for _, value := range unit {
		if remaining().MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		root, used, e := geom.NewExactRationalRoot(ctx, value, remaining())
		r.ExactOperations += used
		if e != nil {
			return r, e
		}
		r.Nodes = append(r.Nodes, RepeatedSplineArcTrimNode{Parameter: root})
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (geom.ExactCurveEventOrder, error) {
		p := remaining()
		if p.MaxOperations < 1 || p.MaxSubdivisions < 1 {
			return geom.ExactCurveEventOrder{}, geom.ErrConvergence
		}
		q, e := a.Compare(ctx, b, p)
		r.ExactOperations += q.Operations
		r.Subdivisions += q.Subdivisions
		if e == nil && !q.Proven {
			e = geom.ErrConvergence
		}
		return q, e
	}
	insert := func(c RepeatedSplineArcTrimContact) error {
		root := c.ArcParameter
		lo, hi := 0, len(r.Nodes)
		for lo < hi {
			m := (lo + hi) / 2
			q, e := compare(r.Nodes[m].Parameter, root)
			if e != nil {
				return e
			}
			if q.Order < 0 {
				lo = m + 1
			} else {
				hi = m
			}
		}
		if lo == len(r.Nodes) {
			return geom.ErrConvergence
		}
		q, e := compare(r.Nodes[lo].Parameter, root)
		if e != nil {
			return e
		}
		if q.Order != 0 {
			if lo == 0 {
				return geom.ErrConvergence
			}
			r.Nodes = append(r.Nodes, RepeatedSplineArcTrimNode{})
			copy(r.Nodes[lo+1:], r.Nodes[lo:])
			r.Nodes[lo] = RepeatedSplineArcTrimNode{Parameter: root}
		}
		r.Nodes[lo].Contacts[c.Face] = append(r.Nodes[lo].Contacts[c.Face], c)
		return nil
	}
	contacts := 0
	addCurved := func(chart, loop, index int, u Use) error {
		p := patchOptions()
		if p.MaxOperations < 1 {
			return geom.ErrConvergence
		}
		spans, used, e := tubeFaceTrimSpans(ctx, u, SplineTubeTrimSource{Loop: loop, Use: index, Edge: u.Edge, Reversed: u.Reversed}, p)
		r.ExactOperations += used
		if e != nil {
			return e
		}
		for spanIndex, span := range spans {
			p := patchOptions()
			if p.MaxOperations < 1 {
				return geom.ErrConvergence
			}
			curve, e := exactBezierTrimParameterCurve(ctx, span.controls, p)
			if e != nil {
				return e
			}
			r.ExactOperations += curve.ExactOperations()
			p = patchOptions()
			if p.MaxOperations < 1 || remaining().MaxSubdivisions < 1 {
				return geom.ErrConvergence
			}
			q, e := r.Arc.IntersectUVTrim(ctx, curve, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: p,
				MaxEvents: max(2, o.MaxContacts-contacts), MaxSubdivisions: remaining().MaxSubdivisions})
			r.ExactOperations += q.ExactOperations
			r.Subdivisions += q.Subdivisions
			if e != nil {
				return e
			}
			if !q.Complete || len(q.Hits) > o.MaxContacts-contacts {
				return geom.ErrConvergence
			}
			contacts += len(q.Hits)
			for _, hit := range q.Hits {
				original := hit.TrimEvent.Root()
				var endpointMask uint8
				for side, onFirstSpan := range [2]bool{spanIndex == 0, spanIndex == len(spans)-1} {
					if !onFirstSpan {
						continue
					}
					position := 0
					if side == 1 {
						position = len(r.Nodes) - 1
					}
					order, e := compare(original, r.Nodes[position].Parameter)
					if e != nil {
						return e
					}
					if order.Order == 0 {
						endpointMask |= 1 << side
					}
				}
				start, end := span.source.ParameterStart, span.source.ParameterEnd
				if start != 0 || end != 1 {
					p := remaining()
					if p.MaxOperations < 1 || p.MaxSubdivisions < 1 {
						return geom.ErrConvergence
					}
					a, z := ratInset(start), ratInset(end)
					original, used, e = original.AffineMap(ctx, a, new(big.Rat).Sub(z, a), p)
					r.ExactOperations += used
					if e != nil {
						return e
					}
				}
				c := RepeatedSplineArcTrimContact{Face: chart, Loop: loop, Use: index, Edge: u.Edge, Span: span.source.Span, Reversed: u.Reversed,
					Event: hit.TrimEvent, ArcParameter: hit.ArcParameter, ExactCoedgeParameter: original, CoedgeParameter: original.Interval(), CoedgeEndpointMask: endpointMask}
				if e := insert(c); e != nil {
					return e
				}
			}
		}
		return nil
	}
	for chart, f := range faces {
		for i, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for j, u := range loop {
				if u.Curve != nil {
					if e := addCurved(chart, i, j, u); e != nil {
						return r, e
					}
					continue
				}
				p := remaining()
				if p.MaxOperations < 1 || p.MaxSubdivisions < 1 {
					return r, geom.ErrConvergence
				}
				start := [3]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y), new(big.Rat)}
				direction := [3]*big.Rat{new(big.Rat).Sub(ratInset(u.UVEnd.X), start[0]), new(big.Rat).Sub(ratInset(u.UVEnd.Y), start[1]), new(big.Rat)}
				q, e := cell.IntersectRationalFiniteLine(ctx, start, direction, unit, geom.ComposedCurveLineOptions{ExactCurvePlaneEventOptions: p})
				r.ExactOperations += q.Operations
				r.Subdivisions += q.Subdivisions
				if e != nil {
					return r, e
				}
				if !q.Complete || len(q.Portions) != 0 || len(q.Hits) > o.MaxContacts-contacts {
					return r, geom.ErrConvergence
				}
				contacts += len(q.Hits)
				for _, hit := range q.Hits {
					c := RepeatedSplineArcTrimContact{Face: chart, Loop: i, Use: j, Edge: u.Edge, Reversed: u.Reversed,
						Event: hit.Event, ArcParameter: hit.Event.Root(), CoedgeParameter: hit.LineParameter, CoedgeEndpointMask: hit.BoundaryMask}
					if e := insert(c); e != nil {
						return r, e
					}
				}
			}
		}
	}
	for i := 1; i < len(r.Nodes); i++ {
		q, e := compare(r.Nodes[i-1].Parameter, r.Nodes[i].Parameter)
		if e != nil {
			return r, e
		}
		if q.Order != -1 || q.Intervals[0][1].Cmp(q.Intervals[1][0]) >= 0 || remaining().MaxOperations < 1 || remaining().MaxSubdivisions < 1 {
			return r, geom.ErrConvergence
		}
		witness := new(big.Rat).Quo(new(big.Rat).Add(q.Intervals[0][1], q.Intervals[1][0]), big.NewRat(2, 1))
		uv, e := cell.EvaluateRational(ctx, witness, remaining())
		r.ExactOperations += uv.ExactOperations
		r.Subdivisions += uv.Subdivisions
		if e != nil {
			return r, e
		}
		v := [2]*big.Rat{uv.Point[0], uv.Point[1]}
		iv := RepeatedSplineArcTrimInterval{Start: i - 1, End: i}
		for chart, f := range faces {
			if o.Loops.MaxWork < 1 || o.Loops.MaxSubdivisions < 1 {
				return r, geom.ErrConvergence
			}
			where, proof, e := LocateRationalBoxInFaceTrim(ctx, f, v, v, o.Loops)
			chargeLoop(proof)
			if e != nil {
				return r, e
			}
			if proof.Status != trimvalidate.Certified || (where != trimvalidate.Inside && where != trimvalidate.Outside) {
				return r, geom.ErrConvergence
			}
			iv.Locations[chart] = where
		}
		iv.Retained = iv.Locations == [2]trimvalidate.Location{trimvalidate.Inside, trimvalidate.Inside}
		r.Intervals = append(r.Intervals, iv)
	}
	for i := range r.Nodes {
		n := &r.Nodes[i]
		n.OnFaces = true
		seen := r.Intervals[min(i, len(r.Intervals)-1)]
		for chart := range faces {
			n.OnFaces = n.OnFaces && (len(n.Contacts[chart]) != 0 || seen.Locations[chart] == trimvalidate.Inside)
		}
		n.Isolated = n.OnFaces && (i == 0 || !r.Intervals[i-1].Retained) && (i == len(r.Intervals) || !r.Intervals[i].Retained)
	}
	for i, p := range supports {
		o := remaining()
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return r, geom.ErrConvergence
		}
		q, e := p.BuildSurfaceCurveCompositionCells(ctx, r.Arc.UVCurve, geom.SurfaceCurveAgreementOptions{
			SurfacePatchOptions: patchOptions(), MaxSubdivisions: o.MaxSubdivisions})
		r.ExactOperations += q.ExactOperations
		r.Subdivisions += q.Subdivisions
		if e != nil {
			return r, e
		}
		if !q.Complete {
			return r, geom.ErrConvergence
		}
		r.Model[i] = q
	}
	r.Complete = true
	r.native = encodeRepeatedSplineArcSection(faces, dependent)
	return r, nil
}
