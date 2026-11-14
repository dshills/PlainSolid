package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofFaceOwner identifies an original input face, not a generated
// Boolean fragment. Operand is 0 for the left input and 1 for the right.
type MixedRoofFaceOwner struct{ Operand, Face int }

// MixedRoofVerticalEvent is an exposed boundary of the Boolean at one exact
// line station. Before and After indicate interior material in the direction
// of increasing line parameter, including beyond the requested clip.
type MixedRoofVerticalEvent struct {
	Parameter     *big.Rat
	Owners        []MixedRoofFaceOwner
	Before, After bool
}

// MixedRoofVerticalBoundarySpan is a clipped portion coincident with one or
// more original side faces. Its open interior has constant side ownership;
// events at the ends can also carry original floor or roof owners.
type MixedRoofVerticalBoundarySpan struct {
	Parameter geom.ExactParameterInterval
	Owners    []MixedRoofFaceOwner
}

// MixedRoofVerticalSection is the exact clipped line section. Spans give the
// closures of the open interior intervals in line parameter order. Events
// include isolated boundary contacts but exclude artificial clip endpoints.
// Start and End classify the clip endpoints, even on a zero-length clip.
// Returned rationals are owned by the caller and do not alias one another.
type MixedRoofVerticalSection struct {
	Events          []MixedRoofVerticalEvent
	Spans           []geom.ExactParameterInterval
	BoundarySpans   []MixedRoofVerticalBoundarySpan
	Start, End      parametric.Location
	ExactOperations int
}

type mixedRoofLineInterval struct {
	active       bool
	lower, upper *big.Rat
	sides        uint8
}

func (v mixedRoofLineInterval) at(t *big.Rat) (before, after, point parametric.Location) {
	if !v.active {
		return
	}
	l, u := t.Cmp(v.lower), t.Cmp(v.upper)
	open := parametric.Inside
	if v.sides != 0 {
		open = parametric.Boundary
	}
	if l > 0 && u <= 0 {
		before = open
	}
	if l >= 0 && u < 0 {
		after = open
	}
	if l >= 0 && u <= 0 {
		point = open
		if l == 0 || u == 0 {
			point = parametric.Boundary
		}
	}
	return
}

// IntersectVerticalRational evaluates the line (x,y,originZ+directionZ*t) for
// minT <= t <= maxT. Lines on a proved footprint edge return exposed
// coincident spans separately from interior spans and isolated contacts.
// The operation budget includes a fixed allowance for sweeping the at most
// four exact stations, plus actual original roof evaluation work.
func (q *MixedRoofMaterialQuery) IntersectVerticalRational(ctx context.Context, xy [2]*big.Rat, originZ, directionZ, minT, maxT *big.Rat, o geom.SurfacePatchEvaluationOptions) (MixedRoofVerticalSection, error) {
	var zero MixedRoofVerticalSection
	o, err := q.evaluationOptions(ctx, o, xy[0], xy[1], originZ, directionZ, minT, maxT)
	if err != nil {
		return zero, err
	}
	if directionZ.Sign() == 0 || minT.Cmp(maxT) > 0 {
		return zero, geom.ErrInvalid
	}
	const sweepAllowance = 160
	if o.MaxOperations < sweepAllowance {
		return zero, geom.ErrConvergence
	}
	out := MixedRoofVerticalSection{ExactOperations: sweepAllowance}
	type station struct {
		t     *big.Rat
		owner MixedRoofFaceOwner
	}
	var intervals [2]mixedRoofLineInterval
	pending := make([]station, 0, 4)
	for i, b := range q.box {
		footprint, sides := b.footprintSides(xy)
		if footprint == parametric.Outside {
			continue
		}
		opt := o
		opt.MaxOperations -= out.ExactOperations
		if opt.MaxOperations < 1 {
			return zero, geom.ErrConvergence
		}
		roof, err := b.roof.EvaluateRational(ctx, xy[0], xy[1], opt)
		if err != nil {
			return zero, err
		}
		out.ExactOperations += roof.ExactOperations
		a, c := station{new(big.Rat), MixedRoofFaceOwner{i, b.floorFace}}, station{new(big.Rat), MixedRoofFaceOwner{i, b.roofFace}}
		for j, h := range [2]*big.Rat{b.floor, roof.Point[2]} {
			v := [2]*big.Rat{a.t, c.t}[j]
			v.Sub(h, originZ)
			if !mixedRoofFits(v, o.MaxCoefficientBits) {
				return zero, geom.ErrConvergence
			}
			v.Quo(v, directionZ)
			if !mixedRoofFits(v, o.MaxCoefficientBits) {
				return zero, geom.ErrConvergence
			}
		}
		if directionZ.Sign() < 0 {
			a, c = c, a
		}
		intervals[i] = mixedRoofLineInterval{true, a.t, c.t, sides}
		pending = append(pending, a, c)
	}
	// Two overlapping coplanar sides would require local 3D coincidence
	// classification. The complete regular source proof does not admit such
	// contacts; retain an explicit guard at the query boundary.
	if a, b := intervals[0], intervals[1]; a.active && b.active && (a.sides&3 != 0 && b.sides&3 != 0 || a.sides&12 != 0 && b.sides&12 != 0) &&
		a.lower.Cmp(b.upper) <= 0 && b.lower.Cmp(a.upper) <= 0 {
		return zero, geom.ErrUnrepresentable
	}
	op := q.op
	type sectionState struct {
		where, before, after parametric.Location
		point, afterPoint    [2]parametric.Location
	}
	state := func(t *big.Rat) (sectionState, error) {
		ab, aa, a := intervals[0].at(t)
		bb, ba, b := intervals[1].at(t)
		r := sectionState{point: [2]parametric.Location{a, b}, afterPoint: [2]parametric.Location{aa, ba}}
		var err error
		r.where, err = mixedRoofBooleanLocation(op, r.point)
		if err != nil {
			return r, err
		}
		r.before, _ = mixedRoofBooleanLocation(op, [2]parametric.Location{ab, bb})
		r.after, _ = mixedRoofBooleanLocation(op, r.afterPoint)
		return r, nil
	}
	sideOwners := func(owners []MixedRoofFaceOwner, locations [2]parametric.Location) []MixedRoofFaceOwner {
		for i, loc := range locations {
			if loc != parametric.Boundary {
				continue
			}
			for j, f := range q.box[i].sideFaces {
				if intervals[i].sides&(1<<j) != 0 {
					owners = append(owners, MixedRoofFaceOwner{i, f})
				}
			}
		}
		return owners
	}
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].t.Cmp(pending[j].t) < 0 })
	current, err := state(minT)
	if err != nil {
		return zero, err
	}
	out.Start = current.where
	last, err := state(maxT)
	if err != nil {
		return zero, err
	}
	out.End = last.where
	spanStart := minT
	addSpan := func(end *big.Rat) {
		if spanStart.Cmp(end) >= 0 || current.after == parametric.Outside {
			return
		}
		span := geom.ExactParameterInterval{Lower: new(big.Rat).Set(spanStart), Upper: new(big.Rat).Set(end)}
		if current.after == parametric.Inside {
			out.Spans = append(out.Spans, span)
		} else {
			out.BoundarySpans = append(out.BoundarySpans, MixedRoofVerticalBoundarySpan{span, sideOwners(nil, current.afterPoint)})
		}
	}
	for i := 0; i < len(pending); {
		t := pending[i].t
		j := i + 1
		for j < len(pending) && pending[j].t.Cmp(t) == 0 {
			j++
		}
		if t.Cmp(minT) >= 0 && t.Cmp(maxT) <= 0 {
			s, err := state(t)
			if err != nil {
				return zero, err
			}
			if s.where == parametric.Boundary {
				e := MixedRoofVerticalEvent{Parameter: new(big.Rat).Set(t), Before: s.before == parametric.Inside, After: s.after == parametric.Inside}
				for _, p := range pending[i:j] {
					e.Owners = append(e.Owners, p.owner)
				}
				e.Owners = sideOwners(e.Owners, s.point)
				sort.Slice(e.Owners, func(i, j int) bool {
					a, b := e.Owners[i], e.Owners[j]
					return a.Operand < b.Operand || a.Operand == b.Operand && a.Face < b.Face
				})
				out.Events = append(out.Events, e)
				addSpan(t)
				spanStart, current = t, s
			}
		}
		i = j
	}
	addSpan(maxT)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
