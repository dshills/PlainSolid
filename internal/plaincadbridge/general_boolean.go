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

// This file is the adapter of the general Boolean (specs/general-boolean).
// On a Bridge whose general field is set, which only a test constructor
// does, it serves every Boolean request. On the default bridge it is the
// fallback: a legacy path answers first, and orGeneral says when the
// general Boolean is asked.

// generalOptions are the tolerances the bridge passes to the general
// Boolean (L4). The general Boolean measures volume, area and centroid in
// closed form, so the integration options serve only a solid with a face
// that the closed form does not cover. They then ask for 1e-9 with the
// sample budget that a result with an oblique or annular cap needs to
// converge; the default of four million samples runs out on those.
var generalOptions = boolean.Options{
	Linear:      1e-7,
	Angular:     1e-9,
	Volume:      1e-7,
	Integration: parametric.IntegrationOptions{RelativeTolerance: 1e-9, MaxSamples: 64000000},
}

var generalOperations = map[string]boolean.Operation{"fuse": boolean.Union, "common": boolean.Common, "cut": boolean.Cut}

// generalBoolean serves cut, fuse and common with boolean.Combine. It
// never calls a legacy path.
func (b *Bridge) generalBoolean(ctx context.Context, a, c stored, op string) (Shape, error) {
	r, err := generalCombine(ctx, a, c, generalOperations[op])
	if err == nil && op == "cut" {
		err = generalCut(ctx, r.VolumeA, r, a, c)
	}
	if err != nil {
		return Shape{}, err
	}
	return b.putGeneral(r)
}

// generalCut holds a cut to the contract that the bridge's cuts share:
// material remains, and some was removed.
//
// The two volumes settle it where they differ by more than a boundary
// that is known to the working tolerance can account for, which is that
// tolerance times the area of the result. Below that they cannot tell a
// small cut from none. A face of a tool within tolerance of a face of the
// stock is taken as that face, and the result is measured as it is built,
// with the wall of the tool ending where the tool ended; the same tool
// cut again then finds a volume that differs by the gap times the area of
// the wall's end. There the cut removed material if the stock and one of
// the tools have a solid in common, and the general Boolean is asked.
func generalCut(ctx context.Context, before float64, r boolean.Result, stock stored, tools ...stored) error {
	none := &failureError{"UNSUPPORTED", "Boolean cut removed no volume, or left no material"}
	if r.Solids == 0 || !removesMaterial(before, r.Volume) {
		return none
	}
	if before-r.Volume > generalOptions.Linear*r.Properties.Area {
		return nil
	}
	for _, tool := range tools {
		common, err := generalCombineRaw(ctx, stock, tool, boolean.Common)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A common that the general Boolean does not give leaves the
		// volumes as the only evidence, and they say material was removed.
		if err != nil || common.Solids > 0 {
			return nil
		}
	}
	return none
}

// generalCombine converts two stored shapes and combines them.
func generalCombine(ctx context.Context, a, c stored, op boolean.Operation) (boolean.Result, error) {
	r, err := generalCombineRaw(ctx, a, c, op)
	if err != nil {
		return boolean.Result{}, generalFailure(err)
	}
	return r, nil
}

// generalCombineRaw is generalCombine with the error as boolean.Combine
// returned it, for a caller that decides on its kind.
func generalCombineRaw(ctx context.Context, a, c stored, op boolean.Operation) (boolean.Result, error) {
	x, err := generalOperand(a)
	if err != nil {
		return boolean.Result{}, err
	}
	return generalStepRaw(ctx, x, generalKnown(a), c, op)
}

// generalStep is one step of a fold: a solid, with its measures where
// they are known, combined with a stored shape.
func generalStep(ctx context.Context, x trimmed.Solid, known *parametric.Properties, c stored, op boolean.Operation) (boolean.Result, error) {
	r, err := generalStepRaw(ctx, x, known, c, op)
	if err != nil {
		return boolean.Result{}, generalFailure(err)
	}
	return r, nil
}

// generalStepRaw is generalStep with the error as boolean.Combine
// returned it.
func generalStepRaw(ctx context.Context, x trimmed.Solid, known *parametric.Properties, c stored, op boolean.Operation) (boolean.Result, error) {
	y, err := generalOperand(c)
	if err != nil {
		return boolean.Result{}, err
	}
	o := generalOptions
	o.KnownA, o.KnownB = known, generalKnown(c)
	return boolean.Combine(ctx, x, y, op, o)
}

// generalKnown is the volume and area the bridge stores for a published
// shape, which the general Boolean then does not integrate again. A
// shape that was never published has none.
func generalKnown(s stored) *parametric.Properties {
	if g := s.result.GeometryAssertions; g.Valid {
		return &parametric.Properties{Volume: g.Volume, Area: g.SurfaceArea}
	}
	return nil
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
// holds the result solid, with the volume and area that the general
// Boolean reports for it. The adapter does not integrate the result.
func (b *Bridge) putGeneral(r boolean.Result) (Shape, error) {
	if r.Solids == 0 {
		return b.put(brep.Solid{})
	}
	return b.publishGeneral(&analyticBody{solid: r.Solid, volume: r.Properties.Volume, area: r.Properties.Area, general: true}, r.Solids)
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

// generalBody reports whether an analytic body is a solid with its measures
// and no recipe of a legacy path, which is what publishGeneral stores.
func generalBody(body *analyticBody) bool {
	plain := analyticBody{solid: body.solid, volume: body.volume, area: body.area, general: body.general}
	v, w := reflect.ValueOf(*body), reflect.ValueOf(plain)
	for i := range v.NumField() {
		if v.Field(i).IsZero() != w.Field(i).IsZero() {
			return false
		}
	}
	return true
}

// generalResult reports whether a stored analytic body is handled as a
// result of the general Boolean when it is transformed or written to STEP.
// On a general bridge that is every body without a legacy recipe, as in the
// trial. On the default bridge it is only a body that putGeneral published,
// so a legacy body with no recipe keeps the handling it had.
func (b *Bridge) generalResult(body *analyticBody) bool {
	return (b.general || body.general) && generalBody(body)
}

// generalBounds is the box of a solid bounded by planes and cylinders:
// the box of its edges, each arc taken to its true extent. It holds
// every face. A coordinate is a linear function of position; on a planar
// face a linear function is greatest and least on the boundary, and on a
// cylinder it is linear along each ruling, which ends on the boundary of
// the face at both ends. trimmed.Solid.Bounds is wider on two counts: it
// boxes the whole parameter rectangle of each face, and it pads the box
// by the tolerance of the solid.
func generalBounds(s trimmed.Solid) (geom.Box, error) {
	points := s.Vertices()
	for _, e := range s.Edges() {
		// Where a coordinate of the curve turns, its derivative passes
		// through zero. The samples only bracket the place; bisection
		// of the derivative finds it, so the box has the extent of the
		// arc and not of a polygon in it. An arc of the result is at
		// most a quarter turn, which turns once at most in a coordinate.
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
				if from[i]*to[i] > 0 || from[i] == 0 {
					continue // no turn here, or one at a sample already taken
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
	box, err := geom.Bounds(points)
	if err != nil {
		return geom.Box{}, err
	}
	// A planar face across a coordinate axis, with the solid on one side of
	// it, holds every point of the solid that lies in its plane. Where the
	// box ends within tolerance of such a plane on the side the face looks
	// to, the box ends at the plane: the vertices in it were computed, from
	// arcs among other things, and are off it by rounding.
	lo, hi := [3]*float64{&box.Min.X, &box.Min.Y, &box.Min.Z}, [3]*float64{&box.Max.X, &box.Max.Y, &box.Max.Z}
	for _, f := range s.Faces() {
		base, m := parametric.SurfaceMapping(f.Surface)
		plane, ok := base.(geom.PlaneSurface)
		if !ok {
			continue
		}
		frame := plane.Frame()
		o, n := m.Apply(frame.Origin()), m.ApplyVector(frame.ZAxis())
		at, along := [3]float64{o.X, o.Y, o.Z}, [3]float64{n.X, n.Y, n.Z}
		for i := range 3 {
			if math.Abs(along[i]) != 1 || along[(i+1)%3] != 0 || along[(i+2)%3] != 0 {
				continue
			}
			// The stored normal is the outward one where the two signs
			// agree, as in the drawing of bores.
			outward := along[i]
			if f.Reversed != (m.Determinant() < 0) {
				outward = -outward
			}
			end := lo[i]
			if outward > 0 {
				end = hi[i]
			}
			if math.Abs(*end-at[i]) <= s.Tolerance() {
				*end = at[i]
			}
		}
	}
	return box, nil
}

// generalCutMany is cutMany as a fold of cuts (CO1). The tools are cut
// one after another; one that the general Boolean cannot cut from the
// body as it stands is set aside and tried again after the others
// (inAnyOrder). Nothing is published unless every tool is cut.
func (b *Bridge) generalCutMany(ctx context.Context, a stored, tools []string) (Shape, error) {
	x, err := generalOperand(a)
	if err != nil {
		return Shape{}, err
	}
	cutters := make([]stored, len(tools))
	for i, id := range tools {
		if cutters[i], err = b.get(id); err != nil {
			return Shape{}, err
		}
	}
	var r boolean.Result
	before, known, cut := math.NaN(), generalKnown(a), false
	err = inAnyOrder(len(cutters), func(i int) error {
		step, err := generalStepRaw(ctx, x, known, cutters[i], boolean.Cut)
		if err != nil {
			return err
		}
		if !cut {
			before, cut = step.VolumeA, true
		}
		r = step
		measures := r.Properties
		x, known = r.Solid, &measures
		return nil
	})
	if err != nil {
		return Shape{}, generalFailure(err)
	}
	if err := generalCut(ctx, before, r, a, cutters...); err != nil {
		return Shape{}, err
	}
	return b.putGeneral(r)
}

// inAnyOrder applies n steps, each once, and returns nil when all are
// applied. The steps are tried in order. One that the general Boolean
// returns as unsupported is set aside, and when the rest have been tried
// the steps set aside are tried again in their order, round after round,
// while each round applies at least one. A round that applies none ends
// with the error its first step returned. Any other error ends at once.
// A body less several tools is the same set of points in whatever order
// they are taken, and a tool that has no closed form against the body as
// it stands may have one once another tool has removed what it met.
func inAnyOrder(n int, apply func(i int) error) error {
	aside := make([]int, 0, n)
	for i := range n {
		aside = append(aside, i)
	}
	for len(aside) > 0 {
		var left []int
		var first error
		for _, i := range aside {
			err := apply(i)
			switch {
			case err == nil:
			case !errors.Is(err, boolean.ErrUnsupported):
				return err
			default:
				if left = append(left, i); first == nil {
					first = err
				}
			}
		}
		if len(left) == len(aside) {
			return first
		}
		aside = left
	}
	return nil
}

// generalJoinAll is joinAll as a left fold of unions (CO1): the targets,
// then the tool.
func (b *Bridge) generalJoinAll(ctx context.Context, targets []string, tool stored) (Shape, error) {
	var x trimmed.Solid
	var known *parametric.Properties
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
			known = generalKnown(c)
			continue
		}
		if r, err = generalStep(ctx, x, known, c, boolean.Union); err != nil {
			return Shape{}, err
		}
		measures := r.Properties
		x, known = r.Solid, &measures
	}
	// As on the legacy path, the targets may be apart, the tool has to
	// connect them, and it has to add material.
	before := r.Volume
	r, err := generalStep(ctx, x, known, tool, boolean.Union)
	if err != nil {
		return Shape{}, err
	}
	if r.Solids != 1 {
		return Shape{}, errors.New("the join requires connected solids: connect every selected target with the tool")
	}
	none := errors.New("the join added no volume beyond the selected target bodies")
	if !removesMaterial(r.Volume, before) {
		return Shape{}, none
	}
	// As for a cut (generalCut), volumes that differ by no more than the
	// working tolerance times the area of the result do not settle
	// whether the tool added material. It did if the tool less the
	// targets is a solid.
	if r.Volume-before <= generalOptions.Linear*r.Properties.Area {
		if y, err := generalOperand(tool); err == nil {
			o := generalOptions
			o.KnownA, o.KnownB = generalKnown(tool), known
			beyond, err := boolean.Combine(ctx, y, x, boolean.Cut, o)
			if ctx.Err() != nil {
				return Shape{}, ctx.Err()
			}
			if err == nil && beyond.Solids == 0 {
				return Shape{}, none
			}
		}
	}
	return b.putGeneral(r)
}

// generalOverlapVolume is the volume of the common (CO2).
func generalOverlapVolume(ctx context.Context, a, c stored) (any, error) {
	volume, err := overlapOf(
		func() (boolean.Result, error) { return generalCombineRaw(ctx, a, c, boolean.Common) },
		func() (boolean.Result, error) { return generalCombineRaw(ctx, a, c, boolean.Cut) },
	)
	if err != nil {
		return nil, generalFailure(err)
	}
	return map[string]float64{"volume": volume}, nil
}

// overlapOf is V(a ∩ b) from the common of a and b. Where the common is
// refused as not a manifold, as one pinched into parts that meet only at
// points is, it is V(a) − V(a − b) from their cut, which is the same
// quantity for any two solids (CO2). Where the cut fails as well, the
// error is that of the common, unless the cut was stopped by its context.
func overlapOf(common, cut func() (boolean.Result, error)) (float64, error) {
	r, err := common()
	if err == nil {
		return r.Volume, nil
	}
	if !errors.Is(err, boolean.ErrNonManifold) {
		return 0, err
	}
	d, e := cut()
	switch {
	case e == nil:
		return d.VolumeA - d.Volume, nil
	case errors.Is(e, context.Canceled), errors.Is(e, context.DeadlineExceeded):
		return 0, e
	}
	return 0, err
}

// generalToFace is prismToFace and circleToFace as CO3 states them: the
// common of the sweep, extended past the target plane, with a block one
// face of which lies on the target plane. The target must still have a
// face on that plane, the whole profile must lie before the plane along
// the sweep, and the target must cover the end of the tool.
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
	// Coverage (CO3): the end cap of the tool is every face of it whose
	// corners all lie on the target plane. It is covered when no face of
	// the union of the tool with the target, and none of their common,
	// has the cap among its sources.
	end := map[int]bool{}
	for i, f := range r.Solid.Faces() {
		end[i] = true
		for _, u := range f.Outer {
			p, err := f.Surface.Eval(u.UVStart.X, u.UVStart.Y)
			end[i] = end[i] && err == nil && math.Abs(p.Sub(origin).Dot(n)) <= r.Solid.Tolerance()
		}
	}
	for _, op := range [2]boolean.Operation{boolean.Union, boolean.Common} {
		c, err := generalStep(ctx, r.Solid, &r.Properties, target, op)
		if err != nil {
			return Shape{}, err
		}
		for _, sources := range c.Sources {
			for _, from := range sources {
				if from[0] == 0 && end[from[1]] {
					return Shape{}, errors.New("extrude to face failed: the termination face does not cover the projected profile")
				}
			}
		}
	}
	return b.putGeneral(r)
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

// orGeneral is the fallback of the default bridge. A legacy path answers
// first. Where it refuses the request as unsupported, the general Boolean
// is tried, and what it gives is the answer. Where the general Boolean
// fails as well, the answer is the legacy refusal, unchanged.
func orGeneral[T any](legacy T, err error, general func() (T, error)) (T, error) {
	var refusal *failureError
	if !errors.As(err, &refusal) || refusal.code != "UNSUPPORTED" {
		return legacy, err
	}
	if r, e := general(); e == nil {
		return r, nil
	}
	return legacy, err
}
