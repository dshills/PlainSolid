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

// IntersectMaterialLine reports all proved material transitions on a finite
// interval of the normalized model line. It verifies a sweep starting and
// ending outside both operand enclosures. Buried operand crossings are omitted.
// Unresolved face contacts or overlapped root enclosures, including simultaneous
// contacts on the generated section, return an error and no partial list.
func (s AlgebraicShellBoolean) IntersectMaterialLine(ctx context.Context, origin, direction geom.Vec3, o SplineRayOptions) ([]AlgebraicShellLineHit, error) {
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
	var candidates []AlgebraicShellLineHit
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
			if r, proved, e := tightenCardinalBooleanLineHit(ctx, operand.faces[h.Face], line, h, o.Patch); e != nil {
				return nil, e
			} else if proved {
				h = r
			}
			candidates = append(candidates, AlgebraicShellLineHit{Operand: i, SplineRayHit: h})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].ParameterMin < candidates[j].ParameterMin })
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
	var out []AlgebraicShellLineHit
	last := o.Line.MinParameter
	for _, h := range candidates {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if h.ParameterMin <= last || h.ParameterMax >= o.Line.MaxParameter {
			return nil, fmt.Errorf("%w: overlapping or unbounded material crossing of operand %d face %d on [%g,%g], previous %g", geom.ErrConvergence, h.Operand, h.Face, h.ParameterMin, h.ParameterMax, last)
		}
		if h.Direction == 0 || (h.Direction > 0) != state[h.Operand] {
			return nil, fmt.Errorf("%w: uncertified material crossing of operand %d face %d on [%g,%g], previous %g, orientation %d", geom.ErrConvergence, h.Operand, h.Face, h.ParameterMin, h.ParameterMax, last, h.Direction)
		}
		last = h.ParameterMax
		before := material()
		state[h.Operand] = !state[h.Operand]
		after := material()
		if before == after || h.ParameterMax < minT || h.ParameterMin > maxT {
			continue
		}
		if h.ParameterMin < minT || h.ParameterMax > maxT {
			return nil, fmt.Errorf("%w: crossing enclosure straddles requested range", geom.ErrConvergence)
		}
		h.Entering = after
		out = append(out, h)
	}
	if state != ([2]bool{}) {
		return nil, geom.ErrConvergence
	}
	return out, nil
}

// A Krawczyk enclosure may be broader than nearby hits on another shell.
// Where the whole original support graph has two affine projected coordinates,
// a cardinal line determines its exact UV directly. Use that proof only to
// tighten already-certified roots, never to skip unresolved support contacts.
func tightenCardinalBooleanLineHit(ctx context.Context, f Face, line geom.Line, hit SplineRayHit, o geom.SurfacePatchOptions) (SplineRayHit, bool, error) {
	direction, origin := line.Direction(), line.Origin()
	d := [3]float64{direction.X, direction.Y, direction.Z}
	x := [3]float64{origin.X, origin.Y, origin.Z}
	axis := -1
	for i, v := range d {
		if v != 0 {
			if axis >= 0 {
				return hit, false, nil
			}
			axis = i
		}
	}
	if axis < 0 {
		return hit, false, geom.ErrInvalid
	}
	axes := [2]int{(axis + 1) % 3, (axis + 2) % 3}
	p, e := exactFaceSupportPatch(f, o)
	if e != nil {
		return hit, false, e
	}
	p, e = p.Trim(hit.Cell)
	if e != nil {
		return hit, false, e
	}
	g, e := geom.NewAffineProjectedGraph(ctx, p, axes, o)
	if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDegenerate) {
		return hit, false, nil
	}
	if e != nil {
		return hit, false, e
	}
	opt := geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	uv, e := g.InvertRational(ctx, [2]*big.Rat{ratInset(x[axes[0]]), ratInset(x[axes[1]])}, opt)
	if e != nil || !uv.Inside {
		return hit, false, e
	}
	j, e := g.EvaluateRational(ctx, uv.UV, opt)
	if e != nil {
		return hit, false, e
	}
	t := new(big.Rat).Quo(new(big.Rat).Sub(j.Point[axis], ratInset(x[axis])), ratInset(d[axis]))
	if t.Cmp(ratInset(hit.ParameterMin)) < 0 || t.Cmp(ratInset(hit.ParameterMax)) > 0 {
		return hit, false, geom.ErrConvergence
	}
	bound, e := geom.RationalOutwardRange(t)
	if e != nil {
		return hit, false, e
	}
	hit.ParameterMin, hit.ParameterMax = bound.Min, bound.Max
	hit.UV, hit.Bounds = rationalUVBounds(uv.UV[0], uv.UV[1]), rationalPointBounds(j.Point)
	return hit, true, nil
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
