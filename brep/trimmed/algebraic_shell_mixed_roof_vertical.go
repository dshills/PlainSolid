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

// MixedRoofVerticalSection is the exact clipped line section. Spans give the
// closures of the open interior intervals in line parameter order. Events
// include isolated boundary contacts but exclude artificial clip endpoints.
// Start and End classify the clip endpoints, even on a zero-length clip.
// Returned rationals are owned by the caller and do not alias one another.
type MixedRoofVerticalSection struct {
	Events          []MixedRoofVerticalEvent
	Spans           []geom.ExactParameterInterval
	Start, End      parametric.Location
	ExactOperations int
}

type mixedRoofLineInterval struct {
	active       bool
	lower, upper *big.Rat
}

func (v mixedRoofLineInterval) at(t *big.Rat) (before, after bool, point parametric.Location) {
	if !v.active {
		return false, false, parametric.Outside
	}
	l, u := t.Cmp(v.lower), t.Cmp(v.upper)
	switch {
	case l < 0 || u > 0:
		point = parametric.Outside
	case l == 0 || u == 0:
		point = parametric.Boundary
	default:
		point = parametric.Inside
	}
	return l > 0 && u <= 0, l >= 0 && u < 0, point
}

// IntersectVerticalRational evaluates the line (x,y,originZ+directionZ*t) for
// minT <= t <= maxT. A line on any proved footprint edge is unrepresentable:
// it is coincident with an original side face and requires a side-span proof.
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
	const sweepAllowance = 96
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
		switch b.footprint(xy) {
		case parametric.Outside:
			continue
		case parametric.Boundary:
			return zero, geom.ErrUnrepresentable
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
		intervals[i] = mixedRoofLineInterval{true, a.t, c.t}
		pending = append(pending, a, c)
	}
	op := q.source.boundary.op
	state := func(t *big.Rat) (bool, bool, parametric.Location, error) {
		ab, aa, a := intervals[0].at(t)
		bb, ba, b := intervals[1].at(t)
		where, err := mixedRoofBooleanLocation(op, [2]parametric.Location{a, b})
		return mixedRoofInterior(op, ab, bb), mixedRoofInterior(op, aa, ba), where, err
	}
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].t.Cmp(pending[j].t) < 0 })
	var inside bool
	_, inside, out.Start, err = state(minT)
	if err != nil {
		return zero, err
	}
	_, _, out.End, err = state(maxT)
	if err != nil {
		return zero, err
	}
	spanStart := minT
	addSpan := func(a, b *big.Rat) {
		if a.Cmp(b) < 0 {
			out.Spans = append(out.Spans, geom.ExactParameterInterval{Lower: new(big.Rat).Set(a), Upper: new(big.Rat).Set(b)})
		}
	}
	for i := 0; i < len(pending); {
		t := pending[i].t
		j := i + 1
		for j < len(pending) && pending[j].t.Cmp(t) == 0 {
			j++
		}
		if t.Cmp(minT) >= 0 && t.Cmp(maxT) <= 0 {
			before, after, where, err := state(t)
			if err != nil {
				return zero, err
			}
			if where == parametric.Boundary {
				e := MixedRoofVerticalEvent{Parameter: new(big.Rat).Set(t), Before: before, After: after}
				for _, p := range pending[i:j] {
					e.Owners = append(e.Owners, p.owner)
				}
				out.Events = append(out.Events, e)
				if inside {
					addSpan(spanStart, t)
				}
				spanStart, inside = t, after
			}
		}
		i = j
	}
	if inside {
		addSpan(spanStart, maxT)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}

func mixedRoofInterior(op parametric.BooleanOp, a, b bool) bool {
	switch op {
	case parametric.Union:
		return a || b
	case parametric.Intersection:
		return a && b
	case parametric.Difference:
		return a && !b
	default:
		return false
	}
}
