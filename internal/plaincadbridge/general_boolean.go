package plaincadbridge

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// This file is the adapter of the general Boolean trial
// (specs/general-boolean). It is reached only on a Bridge whose general
// field is set, which only a test constructor does; the default bridge is
// unchanged.

// generalOptions are the tolerances the bridge passes to the general
// Boolean (L4). The integration takes the sample budget that a result
// with an oblique or annular cap needs to converge at 1e-9; the default
// of four million samples runs out on those.
var generalOptions = boolean.Options{
	Linear:      1e-7,
	Angular:     1e-9,
	Volume:      1e-7,
	Integration: parametric.IntegrationOptions{RelativeTolerance: 1e-9, MaxSamples: 64000000},
}

var generalOperations = map[string]boolean.Operation{"fuse": boolean.Union, "common": boolean.Common, "cut": boolean.Cut}

// generalBoolean serves cut, fuse and common with boolean.Combine. There
// is no fall back to a legacy path.
func (b *Bridge) generalBoolean(ctx context.Context, a, c stored, op string) (Shape, error) {
	r, err := generalCombine(ctx, a, c, generalOperations[op])
	if err == nil && op == "cut" {
		err = generalCut(r.VolumeA, r)
	}
	if err != nil {
		return Shape{}, err
	}
	return b.putGeneral(ctx, r)
}

// generalCut holds a cut to the contract that the bridge's cuts share:
// material remains, and some was removed.
func generalCut(before float64, r boolean.Result) error {
	if r.Solids == 0 || !removesMaterial(before, r.Volume) {
		return &failureError{"UNSUPPORTED", "Boolean cut removed no volume, or left no material"}
	}
	return nil
}

// generalCombine converts two stored shapes and combines them.
func generalCombine(ctx context.Context, a, c stored, op boolean.Operation) (boolean.Result, error) {
	x, err := generalOperand(a)
	if err != nil {
		return boolean.Result{}, err
	}
	return generalStep(ctx, x, c, op)
}

// generalStep is one step of a fold: a solid combined with a stored shape.
func generalStep(ctx context.Context, x trimmed.Solid, c stored, op boolean.Operation) (boolean.Result, error) {
	y, err := generalOperand(c)
	if err != nil {
		return boolean.Result{}, err
	}
	r, err := boolean.Combine(ctx, x, y, op, generalOptions)
	if err != nil {
		return boolean.Result{}, generalFailure(err)
	}
	return r, nil
}

// generalOperand reads a stored shape only to obtain its trimmed solid
// (L2): the solid a bored or analytic body holds, or the lift of a
// cylinder, a patch solid or a planar solid.
func generalOperand(s stored) (trimmed.Solid, error) {
	var t trimmed.Solid
	var err error
	switch {
	case s.bore != nil:
		t = s.bore.solid
	case s.analytic != nil && s.analytic.patches != nil && s.analytic.solid.Tolerance() == 0:
		t, err = trimmed.FromParametric(*s.analytic.patches)
	case s.analytic != nil:
		t = s.analytic.solid
	case s.cylinder != nil:
		t, err = trimmed.FromParametric(s.cylinder.solid)
	case s.planar():
		if len(s.solid.Faces()) == 0 {
			return trimmed.Solid{}, nil
		}
		t, err = trimmed.FromPlanar(s.solid, generalOptions.Linear)
	default:
		return trimmed.Solid{}, &failureError{"UNSUPPORTED", "the general Boolean takes no stored shape of this kind"}
	}
	if err != nil || t.Tolerance() == 0 {
		return trimmed.Solid{}, &failureError{"UNSUPPORTED", fmt.Sprintf("the stored shape has no trimmed solid (%v)", err)}
	}
	return t, nil
}

// putGeneral publishes a result of the general Boolean: the empty solid
// as the legacy paths publish it, and any other as an analytic body that
// holds the result solid, with its integrated volume and area.
func (b *Bridge) putGeneral(ctx context.Context, r boolean.Result) (Shape, error) {
	if r.Solids == 0 {
		return b.put(brep.Solid{})
	}
	p, err := r.Solid.MassProperties(ctx, generalOptions.Integration)
	if err != nil {
		return Shape{}, err
	}
	return b.publishGeneral(&analyticBody{solid: r.Solid, volume: p.Volume, area: p.Area}, r.Solids)
}

// publishGeneral stores an analytic body of any number of shells. It
// calls publish and not putAnalyticBody, which takes one shell only and
// pads the bounds by the tolerance of the solid.
func (b *Bridge) publishGeneral(body *analyticBody, solids int) (Shape, error) {
	box, err := generalBounds(body.solid)
	if err != nil {
		return Shape{}, err
	}
	return b.publish(stored{analytic: body, result: Shape{
		GeometryAssertions: Assertions{true, body.volume, body.area, solids},
		Bounds:             Bounds{coordinates(box.Min), coordinates(box.Max)},
	}})
}

// generalBody reports whether an analytic body is one that publishGeneral
// stored: a solid with its measures and no recipe of a legacy path.
func generalBody(body *analyticBody) bool {
	plain := analyticBody{solid: body.solid, volume: body.volume, area: body.area}
	v, w := reflect.ValueOf(*body), reflect.ValueOf(plain)
	for i := range v.NumField() {
		if v.Field(i).IsZero() != w.Field(i).IsZero() {
			return false
		}
	}
	return true
}

// generalBounds is the box of a solid bounded by planes and cylinders:
// the box of its edges. A coordinate is linear along every ruling of
// such a face, and a ruling ends on the boundary, so no face reaches
// further than its edges. trimmed.Solid.Bounds is wider on both counts:
// it boxes the whole parameter rectangle of each face, and it pads the
// box by the tolerance of the solid.
func generalBounds(s trimmed.Solid) (geom.Box, error) {
	points := s.Vertices()
	for _, e := range s.Edges() {
		// Where a coordinate of the curve turns: its derivative changes
		// sign between two of the samples, and bisection finds the place.
		const samples = 16
		slope := func(t float64) [3]float64 {
			d, _ := e.Curve.Derivative(t)
			return [3]float64{d.X, d.Y, d.Z}
		}
		step := (e.Range.Max - e.Range.Min) / samples
		for k := range samples {
			lo, hi := e.Range.Min+float64(k)*step, e.Range.Min+float64(k+1)*step
			from, to := slope(lo), slope(hi)
			for i := range 3 {
				if from[i]*to[i] >= 0 {
					continue
				}
				u, v := lo, hi
				for range 60 {
					if m := (u + v) / 2; slope(m)[i]*from[i] > 0 {
						u = m
					} else {
						v = m
					}
				}
				p, err := e.Curve.Eval((u + v) / 2)
				if err != nil {
					return geom.Box{}, err
				}
				points = append(points, p)
			}
		}
	}
	return geom.Bounds(points)
}

// generalCutMany is cutMany as a left fold of cuts (CO1). Nothing is
// published unless every step succeeds.
func (b *Bridge) generalCutMany(ctx context.Context, a stored, tools []string) (Shape, error) {
	x, err := generalOperand(a)
	if err != nil {
		return Shape{}, err
	}
	var r boolean.Result
	before := math.NaN()
	for i, id := range tools {
		c, err := b.get(id)
		if err != nil {
			return Shape{}, err
		}
		if r, err = generalStep(ctx, x, c, boolean.Cut); err != nil {
			return Shape{}, err
		}
		if i == 0 {
			before = r.VolumeA
		}
		x = r.Solid
	}
	if err := generalCut(before, r); err != nil {
		return Shape{}, err
	}
	return b.putGeneral(ctx, r)
}

// generalJoinAll is joinAll as a left fold of unions (CO1): the targets,
// then the tool.
func (b *Bridge) generalJoinAll(ctx context.Context, targets []string, tool stored) (Shape, error) {
	var x trimmed.Solid
	var r boolean.Result
	for i, id := range targets {
		c, err := b.get(id)
		if err != nil {
			return Shape{}, err
		}
		if i == 0 {
			if x, err = generalOperand(c); err != nil {
				return Shape{}, err
			}
			continue
		}
		if r, err = generalStep(ctx, x, c, boolean.Union); err != nil {
			return Shape{}, err
		}
		x = r.Solid
	}
	// As on the legacy path, the targets may be apart, the tool has to
	// connect them, and it has to add material.
	before := r.Volume
	r, err := generalStep(ctx, x, tool, boolean.Union)
	if err != nil {
		return Shape{}, err
	}
	if r.Solids != 1 {
		return Shape{}, errors.New("the join requires connected solids: connect every selected target with the tool")
	}
	if !removesMaterial(r.Volume, before) {
		return Shape{}, errors.New("the join added no volume beyond the selected target bodies")
	}
	return b.putGeneral(ctx, r)
}

// generalOverlapVolume is the volume of the common (CO2).
func generalOverlapVolume(ctx context.Context, a, c stored) (any, error) {
	r, err := generalCombine(ctx, a, c, boolean.Common)
	if err != nil {
		return nil, err
	}
	return map[string]float64{"volume": r.Volume}, nil
}

// generalToFace is prismToFace and circleToFace as CO3 states them: the
// common of the sweep, extended past the target plane, with a block one
// face of which lies on the target plane. The target must still have a
// face on that plane, and the whole profile must lie before the plane
// along the sweep.
func (b *Bridge) generalToFace(ctx context.Context, target stored, q request) (Shape, error) {
	origin, normal := q.Origin.vec(), q.Normal.vec()
	err := errors.New("the target has no planar face to end on")
	if target.analytic != nil {
		_, err = target.analytic.planarFace(origin, normal)
	} else if target.planar() {
		_, err = matchingPlanarFace(target.solid, origin, normal)
	}
	if err != nil {
		return Shape{}, fmt.Errorf("to-face reference was lost or is ambiguous: %w", err)
	}
	profile, radius := loop(q.Outer), 0.
	if q.Op == "circleToFace" {
		if q.Center == nil || len(q.Holes) != 0 {
			return Shape{}, errors.New("circular to-face extrusion requires a center and no inner loops")
		}
		profile, radius = []geom.Vec3{q.Center.vec()}, float64(q.Radius)
	}
	n, e1 := normal.Unit()
	d, e2 := q.Direction.vec().Unit()
	rate := d.Dot(n)
	if len(profile) == 0 || e1 != nil || e2 != nil || math.Abs(rate) < generalOptions.Angular {
		return Shape{}, errors.New("extrude to face failed: the sweep does not reach the target plane")
	}
	// The distance along the sweep from the profile to the plane, and
	// the width of the profile.
	near, far, width := math.Inf(1), 0., radius
	for _, p := range profile {
		t, slack := origin.Sub(p).Dot(n)/rate, radius*n.Cross(d).Length()/math.Abs(rate)
		near, far, width = math.Min(near, t-slack), math.Max(far, t+slack), math.Max(width, radius+p.Distance(profile[0]))
	}
	if !(near > generalOptions.Linear) {
		return Shape{}, errors.New("extrude to face failed: the profile does not lie before the target plane")
	}
	var sweep stored
	if q.Op == "circleToFace" {
		frame, err := geom.FrameFromNormal(profile[0], d)
		if err != nil {
			return Shape{}, err
		}
		sweep.cylinder, err = newCylinder(frame, radius, 2*far)
		if err != nil {
			return Shape{}, err
		}
	} else {
		holes := make([][]geom.Vec3, len(q.Holes))
		for i, h := range q.Holes {
			holes[i] = loop(h)
		}
		if sweep.solid, err = brep.PrismWithHoles(profile, holes, d.Scale(2*far)); err != nil {
			return Shape{}, err
		}
	}
	// The block: a square on the target plane about the foot of the
	// sweep, wide enough to hold the sweep, raised to the profile's side.
	frame, err := geom.FrameFromNormal(profile[0].Add(d.Scale(origin.Sub(profile[0]).Dot(n)/rate)), n)
	if err != nil {
		return Shape{}, err
	}
	side := 4 * (far + width) / math.Abs(rate)
	foot, x, y := frame.Origin(), frame.XAxis().Scale(side), frame.YAxis().Scale(side)
	block, err := brep.Prism([]geom.Vec3{foot.Sub(x).Sub(y), foot.Add(x).Sub(y), foot.Add(x).Add(y), foot.Sub(x).Add(y)}, n.Scale(-math.Copysign(2*far, rate)))
	if err != nil {
		return Shape{}, err
	}
	r, err := generalCombine(ctx, sweep, stored{solid: block}, boolean.Common)
	if err != nil {
		return Shape{}, err
	}
	return b.putGeneral(ctx, r)
}

// generalFailure maps the errors of the general Boolean to bridge
// failures.
func generalFailure(err error) error {
	code := "INVALID"
	switch {
	case errors.Is(err, boolean.ErrInvalid):
	case errors.Is(err, boolean.ErrUnsupported), errors.Is(err, boolean.ErrNonManifold), errors.Is(err, boolean.ErrTolerance):
		code = "UNSUPPORTED"
	case errors.Is(err, boolean.ErrBudget):
		code = "LIMIT"
	case errors.Is(err, boolean.ErrInconsistent), errors.Is(err, boolean.ErrValidation):
		code = "INTERNAL"
	default:
		return err
	}
	return &failureError{code, err.Error()}
}
