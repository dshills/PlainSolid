//go:build research

package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicShellLineHit identifies a transverse material transition on an
// original operand face. The parameter and model bounds enclose the true
// contact. Direction remains the original operand's oriented crossing sign;
// Entering is with respect to Boolean material in line order.
type AlgebraicShellLineHit struct {
	Operand  int
	Entering bool
	SplineRayHit
}

// AlgebraicShellLineEvent retains one crossing or an exact simultaneous
// contact. Before and After describe material on the two open half-lines
// immediately next to the event. A simultaneous contact can have the same
// material state on both sides despite lying on the result boundary.
type AlgebraicShellLineEvent struct {
	ParameterMin, ParameterMax float64
	ExactParameter             *big.Rat // set when proved by a projected original chart
	Before, After              bool
	Hits                       []AlgebraicShellLineHit
}

// IntersectMaterialLine reports all proved material transitions on a finite
// interval of the normalized model line. It verifies a sweep starting and
// ending outside both operand enclosures. Buried operand crossings are omitted.
// Unresolved face contacts or overlapped root enclosures, including simultaneous
// contacts on the generated section, return an error and no partial list.
func (s AlgebraicShellBoolean) IntersectMaterialLine(ctx context.Context, origin, direction geom.Vec3, o SplineRayOptions) ([]AlgebraicShellLineHit, error) {
	events, e := s.intersectMaterialLine(ctx, origin, direction, o, false)
	if e != nil {
		return nil, e
	}
	var hits []AlgebraicShellLineHit
	for _, event := range events {
		hits = append(hits, event.Hits[0])
	}
	return hits, nil
}

// IntersectMaterialLineEvents additionally reports simultaneous crossings
// whose exact rational parameters coincide on the two original operands.
// The qualified section is transverse between operand surfaces. Therefore
// a simultaneous crossing lies on the Boolean boundary even when neither
// line side contains material. Nearby overlapping but unequal enclosures
// still fail without publishing a partial list.
func (s AlgebraicShellBoolean) IntersectMaterialLineEvents(ctx context.Context, origin, direction geom.Vec3, o SplineRayOptions) ([]AlgebraicShellLineEvent, error) {
	return s.intersectMaterialLine(ctx, origin, direction, o, true)
}

func (s AlgebraicShellBoolean) intersectMaterialLine(ctx context.Context, origin, direction geom.Vec3, o SplineRayOptions, simultaneous bool) ([]AlgebraicShellLineEvent, error) {
	if ctx == nil || s.source == nil || !finite(o.Line.MinParameter) || !finite(o.Line.MaxParameter) || o.Line.MinParameter >= o.Line.MaxParameter {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	line, e := geom.NewLine(origin, direction)
	if e != nil {
		return nil, e
	}
	if len(s.faces) == 0 {
		return nil, nil
	}
	minT, maxT := o.Line.MinParameter, o.Line.MaxParameter
	sources := [2]Solid{s.source.a, s.source.b}
	shift := new(big.Rat)
	// For a cardinal direction, replace the axial origin with an exactly
	// represented component of the source box. Candidate parameters are
	// translated back in rationals. This avoids large parameter magnitudes
	// in the original chart isolation without moving the actual line.
	d := line.Direction()
	if axis := algebraicCardinalAxis(d); axis >= 0 {
		directionUnit := [3]float64{d.X, d.Y, d.Z}[axis]
		x := [3]float64{origin.X, origin.Y, origin.Z}
		for i, operand := range sources {
			if len(operand.faces) == 0 {
				continue
			}
			b := s.source.bounds[i]
			lo, hi := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}[axis], [3]float64{b.Max.X, b.Max.Y, b.Max.Z}[axis]
			if math.Abs(x[axis]-lo) > 16*max(1, hi-lo) {
				shift.Quo(shift.Sub(ratInset(lo), ratInset(x[axis])), ratInset(directionUnit))
				x[axis] = lo
				origin = geom.Vec3{X: x[0], Y: x[1], Z: x[2]}
				line, e = geom.NewLine(origin, direction)
				if e != nil {
					return nil, e
				}
				o.Line.MinParameter, o.Line.MaxParameter = 0, 1
			}
			break
		}
	}
	for i, operand := range sources {
		if len(operand.faces) == 0 {
			continue
		}
		lo, hi, e := outsideBooleanLineBounds(line, s.source.bounds[i])
		if e != nil {
			return nil, e
		}
		o.Line.MinParameter, o.Line.MaxParameter = min(o.Line.MinParameter, lo), max(o.Line.MaxParameter, hi)
	}
	type candidate struct {
		AlgebraicShellLineHit
		parameter    *big.Rat
		lower, upper *big.Rat
	}
	var candidates []candidate
	for i, operand := range sources {
		if len(operand.faces) == 0 {
			continue
		}
		q, e := operand.SplineRay(ctx, origin, direction, o)
		if e != nil {
			return nil, fmt.Errorf("trimmed: material line operand %d: %w", i, e)
		}
		if !q.Complete {
			return nil, geom.ErrConvergence
		}
		for _, h := range q.Hits {
			r, at, e := tightenCardinalBooleanLineHit(ctx, operand.faces[h.Face], line, h, o.Patch)
			if e != nil {
				return nil, e
			}
			lo, hi := algebraicLineOrderBound(at, r.ParameterMin), algebraicLineOrderBound(at, r.ParameterMax)
			if shift.Sign() != 0 {
				lo, hi = new(big.Rat).Add(lo, shift), new(big.Rat).Add(hi, shift)
				if at != nil {
					at = lo
				}
				l, e := geom.RationalOutwardRange(lo)
				if e != nil {
					return nil, e
				}
				u, e := geom.RationalOutwardRange(hi)
				if e != nil {
					return nil, e
				}
				r.ParameterMin, r.ParameterMax = l.Min, u.Max
			}
			candidates = append(candidates, candidate{
				AlgebraicShellLineHit: AlgebraicShellLineHit{Operand: i, SplineRayHit: r}, parameter: at,
				lower: lo, upper: hi,
			})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].lower.Cmp(candidates[j].lower) < 0 })
	var state [2]bool
	material := func() bool {
		switch s.op {
		case parametric.Union:
			return state[0] || state[1]
		case parametric.Intersection:
			return state[0] && state[1]
		default:
			return state[0] && !state[1]
		}
	}
	var out []AlgebraicShellLineEvent
	last, far := new(big.Rat).Add(ratInset(o.Line.MinParameter), shift), new(big.Rat).Add(ratInset(o.Line.MaxParameter), shift)
	requestedLow, requestedHigh := ratInset(minT), ratInset(maxT)
	for i := 0; i < len(candidates); {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		h := candidates[i]
		group := []candidate{h}
		i++
		if simultaneous && h.parameter != nil && i < len(candidates) {
			next := candidates[i]
			if next.parameter != nil && next.parameter.Cmp(h.parameter) == 0 && next.Operand != h.Operand {
				group = append(group, next)
				i++
			}
		}
		lo, hi := h.lower, h.upper
		if lo.Cmp(last) <= 0 || hi.Cmp(far) >= 0 {
			return nil, fmt.Errorf("%w: overlapping or unbounded material crossing of operand %d face %d on [%g,%g], previous %s", geom.ErrConvergence, h.Operand, h.Face, h.ParameterMin, h.ParameterMax, last.RatString())
		}
		last = hi
		before := material()
		for _, x := range group {
			if x.Direction == 0 || (x.Direction > 0) != state[x.Operand] {
				return nil, fmt.Errorf("%w: uncertified material crossing of operand %d face %d on [%g,%g], orientation %d", geom.ErrConvergence, x.Operand, x.Face, x.ParameterMin, x.ParameterMax, x.Direction)
			}
			state[x.Operand] = !state[x.Operand]
		}
		after := material()
		if before == after && len(group) == 1 || hi.Cmp(requestedLow) < 0 || lo.Cmp(requestedHigh) > 0 {
			continue
		}
		if lo.Cmp(requestedLow) < 0 || hi.Cmp(requestedHigh) > 0 {
			return nil, fmt.Errorf("%w: crossing enclosure straddles requested range", geom.ErrConvergence)
		}
		event := AlgebraicShellLineEvent{ParameterMin: h.ParameterMin, ParameterMax: h.ParameterMax, ExactParameter: h.parameter,
			Before: before, After: after}
		for _, x := range group {
			x.Entering = !before && after
			event.Hits = append(event.Hits, x.AlgebraicShellLineHit)
		}
		out = append(out, event)
	}
	if state != ([2]bool{}) {
		return nil, geom.ErrConvergence
	}
	return out, nil
}

// A proved rational parameter is stronger than its outward presentation
// bounds. Unproved roots still use their conservative enclosing endpoints.
func algebraicLineOrderBound(q *big.Rat, b float64) *big.Rat {
	if q != nil {
		return q
	}
	return ratInset(b)
}

func algebraicCardinalAxis(v geom.Vec3) int {
	switch {
	case v.X != 0 && v.Y == 0 && v.Z == 0:
		return 0
	case v.X == 0 && v.Y != 0 && v.Z == 0:
		return 1
	case v.X == 0 && v.Y == 0 && v.Z != 0:
		return 2
	default:
		return -1
	}
}

// A Krawczyk enclosure may be broader than nearby hits on another shell.
// Where the whole original support graph has two affine projected coordinates,
// a cardinal line determines its exact UV directly. Use that proof only to
// tighten already-certified roots, never to skip unresolved support contacts.
func tightenCardinalBooleanLineHit(ctx context.Context, f Face, line geom.Line, hit SplineRayHit, o geom.SurfacePatchOptions) (SplineRayHit, *big.Rat, error) {
	direction, origin := line.Direction(), line.Origin()
	d := [3]float64{direction.X, direction.Y, direction.Z}
	x := [3]float64{origin.X, origin.Y, origin.Z}
	axis := algebraicCardinalAxis(direction)
	if axis < 0 {
		return hit, nil, nil
	}
	axes := [2]int{(axis + 1) % 3, (axis + 2) % 3}
	p, e := exactFaceSupportPatch(f, o)
	if e != nil {
		return hit, nil, e
	}
	p, e = p.Trim(hit.Cell)
	if e != nil {
		return hit, nil, e
	}
	g, e := geom.NewAffineProjectedGraph(ctx, p, axes, o)
	if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) {
		return hit, nil, nil
	}
	if e != nil {
		return hit, nil, e
	}
	opt := geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	uv, e := g.InvertRational(ctx, [2]*big.Rat{ratInset(x[axes[0]]), ratInset(x[axes[1]])}, opt)
	if e != nil || !uv.Inside {
		return hit, nil, e
	}
	j, e := g.EvaluateRational(ctx, uv.UV, opt)
	if e != nil {
		return hit, nil, e
	}
	t := new(big.Rat).Quo(new(big.Rat).Sub(j.Point[axis], ratInset(x[axis])), ratInset(d[axis]))
	if t.Cmp(ratInset(hit.ParameterMin)) < 0 || t.Cmp(ratInset(hit.ParameterMax)) > 0 {
		return hit, nil, geom.ErrConvergence
	}
	bound, e := geom.RationalOutwardRange(t)
	if e != nil {
		return hit, nil, e
	}
	hit.ParameterMin, hit.ParameterMax = bound.Min, bound.Max
	hit.UV, hit.Bounds = rationalUVBounds(uv.UV[0], uv.UV[1]), rationalPointBounds(j.Point)
	return hit, t, nil
}

func outsideBooleanLineBounds(line geom.Line, b geom.Box) (float64, float64, error) {
	x, v := line.Origin(), line.Direction()
	d := [3]float64{v.X, v.Y, v.Z}
	axis := 0
	for i := 1; i < 3; i++ {
		if math.Abs(d[i]) > math.Abs(d[axis]) {
			axis = i
		}
	}
	p := [3]float64{x.X, x.Y, x.Z}[axis]
	a := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}[axis]
	z := [3]float64{b.Max.X, b.Max.Y, b.Max.Z}[axis]
	delta := func(q float64) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Sub(ratInset(q), ratInset(p)), ratInset(d[axis]))
	}
	lo, hi := delta(a), delta(z)
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	l, e := geom.RationalOutwardRange(lo.Sub(lo, big.NewRat(1, 1)))
	if e != nil {
		return 0, 0, e
	}
	h, e := geom.RationalOutwardRange(hi.Add(hi, big.NewRat(1, 1)))
	if e != nil {
		return 0, 0, e
	}
	return l.Min, h.Max, nil
}
