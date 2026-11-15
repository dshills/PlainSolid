package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// SplineRayOptions bounds exact line isolation and outward trim checks.
// Distance limits in Line must be finite and increasing. Unknown supports or
// trims return errors; near-trim boxes can remain unresolved.
type SplineRayOptions struct {
	Patch                         geom.SurfacePatchOptions
	Line                          geom.LineSplineSurfaceOptions
	Crossing                      geom.SurfaceTransversalityOptions
	Trim                          trimvalidate.Options
	MaxFaces, MaxTrimSubdivisions int
}

// SplineRayHit is a root proved to be in the interior of a trimmed spline
// face. Direction is the certified oriented crossing sign when nonzero.
type SplineRayHit struct {
	Face int
	geom.LineSplineSurfaceHit
	Direction int
}

// SplineRayIssue records a possible root that has not been classified.
type SplineRayIssue struct {
	Face   int
	Domain geom.ParameterRectangle
	UV     geom.UVBox
	Reason string
}

// SplineRayResult is exhaustive only when Complete. Roots certified outside
// the trims are excluded. Remaining root and trim ambiguities are retained.
type SplineRayResult struct {
	Hits     []SplineRayHit
	Issues   []SplineRayIssue
	Complete bool
}

// SplineRay intersects a bounded line with trimmed clamped rational spline
// or exact Bézier supports, including affine placements. It classifies the entire root UV box:
// straight edges use exact orientation; supported curved edges use outward
// bounds. Faces with native exact Bézier uses have certified rational
// winding and bounded homogeneous trim exclusion. The supplied solid
// must have embedded UV loops. Coincidence and roots near a curved trim
// can remain unresolved; no sampled winding replaces a missing proof.
func (s Solid) SplineRay(ctx context.Context, origin, direction geom.Vec3, o SplineRayOptions) (SplineRayResult, error) {
	line, e := geom.NewLine(origin, direction)
	if e != nil {
		return SplineRayResult{}, e
	}
	d := line.Direction()
	return s.splineRay(ctx, [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)},
		[3]*big.Rat{ratInset(d.X), ratInset(d.Y), ratInset(d.Z)}, &line, o)
}

// SplineRayRational intersects a bounded exact model line with the original
// trimmed supports. Parameter bounds refer to origin+t*direction using the
// caller's exact direction without normalization. Incomplete proofs retain
// their issues just as SplineRay does.
func (s Solid) SplineRayRational(ctx context.Context, origin, direction [3]*big.Rat, o SplineRayOptions) (SplineRayResult, error) {
	for i := range origin {
		if origin[i] == nil || direction[i] == nil {
			return SplineRayResult{}, geom.ErrInvalid
		}
	}
	if direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 {
		return SplineRayResult{}, geom.ErrInvalid
	}
	return s.splineRay(ctx, origin, direction, nil, o)
}

func (s Solid) splineRay(ctx context.Context, origin, direction [3]*big.Rat, represented *geom.Line, o SplineRayOptions) (SplineRayResult, error) {
	var r SplineRayResult
	if o.MaxFaces == 0 {
		o.MaxFaces = 128
	}
	if o.MaxTrimSubdivisions == 0 {
		o.MaxTrimSubdivisions = 1024
	}
	if ctx == nil || o.MaxFaces < 1 || len(s.faces) > o.MaxFaces || o.MaxTrimSubdivisions < 1 ||
		o.MaxTrimSubdivisions > 1<<20 ||
		!finite(o.Line.MinParameter) || !finite(o.Line.MaxParameter) || o.Line.MinParameter >= o.Line.MaxParameter {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		support, placement := parametric.SurfaceMapping(f.Surface)
		if !supportsExactSplinePatch(support) {
			return r, fmt.Errorf("trimmed: spline ray face %d has unsupported support %T", i, f.Surface)
		}
		exactTrim := hasNativeExactSplineTrim(f)
		if base, ok := support.(*geom.BSplineSurface); ok && exactTrim && represented != nil {
			handled, e := exactVerticalGraphFaceRay(ctx, &r, i, f, base, placement, *represented, o)
			if e != nil {
				return r, e
			}
			if handled {
				continue
			}
		}
		d, e := faceUVBounds(f)
		if e != nil {
			return r, e
		}
		p, e := exactFaceSupportPatch(f, o.Patch)
		if e != nil {
			return r, e
		}
		full := p.Domain()
		d.UMin, d.UMax = max(d.UMin, full.UMin), min(d.UMax, full.UMax)
		d.VMin, d.VMax = max(d.VMin, full.VMin), min(d.VMax, full.VMax)
		p, e = p.Trim(d)
		if e != nil {
			return r, e
		}
		report, e := geom.IntersectRationalLineSplineSurface(ctx, origin, direction, p, o.Line)
		if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, context.Canceled) && !errors.Is(e, context.DeadlineExceeded) {
			return r, e
		}
		if !report.Complete {
			for _, pending := range report.Pending {
				r.Issues = append(r.Issues, SplineRayIssue{Face: i, Domain: pending.Cell, Reason: pending.Reason})
			}
			if len(report.Pending) == 0 {
				r.Issues = append(r.Issues, SplineRayIssue{Face: i, Domain: d, Reason: "incomplete support isolation"})
			}
		}
		if err := ctx.Err(); err != nil {
			return r, err
		}
		for _, hit := range report.Hits {
			var outside, known bool
			var e error
			if exactTrim {
				loc, certified, err := classifySplineFaceRootBox(ctx, f, hit.UV, o.Trim, o.MaxTrimSubdivisions)
				// A boundary root has not been classified as a shell crossing.
				// It can have more than one owning face and different signs.
				outside, known, e = loc == trimvalidate.Outside, certified && loc != trimvalidate.Boundary, err
			} else {
				loc, certified, err := classifyTrimBox(ctx, f, hit.UV, o.MaxTrimSubdivisions)
				outside, known, e = loc == parametric.Outside, certified, err
			}
			if e != nil {
				return r, e
			}
			if !known {
				r.Issues = append(r.Issues, SplineRayIssue{Face: i, Domain: d, UV: hit.UV, Reason: "root enclosure meets a trim bound"})
				continue
			}
			if outside {
				continue
			}
			c, e := hit.RationalCrossing(direction, p, o.Crossing)
			if e != nil && !errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrConvergence) {
				return r, e
			}
			sign := c.Sign
			if f.Reversed {
				sign = -sign
			}
			r.Hits = append(r.Hits, SplineRayHit{Face: i, LineSplineSurfaceHit: hit, Direction: sign})
		}
	}
	sort.SliceStable(r.Hits, func(i, j int) bool { return r.Hits[i].ParameterMin < r.Hits[j].ParameterMin })
	r.Complete = len(r.Issues) == 0
	if !r.Complete {
		return r, geom.ErrConvergence
	}
	return r, nil
}

func classifyTrimBox(ctx context.Context, f Face, q geom.UVBox, limit int) (parametric.Location, bool, error) {
	overlap := func(a, b geom.Vec2) bool {
		return a.X <= q.Max.X && b.X >= q.Min.X && a.Y <= q.Max.Y && b.Y >= q.Min.Y
	}
	p := geom.Vec2{X: q.Min.X/2 + q.Max.X/2, Y: q.Min.Y/2 + q.Max.Y/2}
	corners := [4]geom.Vec2{q.Min, {X: q.Min.X, Y: q.Max.Y}, q.Max, {X: q.Max.X, Y: q.Min.Y}}
	sum := 0
	for _, l := range append([][]Use{f.Outer}, f.Holes...) {
		loop := make([]geom.Vec2, len(l))
		for i, u := range l {
			loop[i] = u.UVStart
			if e := ctx.Err(); e != nil {
				return parametric.Outside, false, e
			}
			lo, hi, e := trimBounds(u)
			if e != nil {
				return parametric.Outside, false, e
			}
			if !overlap(lo, hi) {
				continue
			}
			if u.Curve != nil {
				if c, reverse, ok := trimCertifiedSpline(u.Curve); ok {
					path, known, e := boundedTrimChordPath(ctx, c, func(ctx context.Context, a, b float64) (geom.UVBox, bool, error) {
						return c.EncloseSplineUV(ctx, a, b, geom.SurfaceArcIntervalOptions{MaxIterations: 2})
					}, reverse, q, &limit)
					if e != nil || !known || path[0] != u.UVStart || path[len(path)-1] != u.UVEnd {
						return parametric.Outside, false, e
					}
					sum += winding(p, path) - winding(p, []geom.Vec2{path[0], path[len(path)-1]})
					continue
				}
				if c, reverse, ok := trimTube(u.Curve); ok {
					path, known, e := tubeTrimChordPath(ctx, c, reverse, q, &limit)
					if e != nil || !known || path[0] != u.UVStart || path[len(path)-1] != u.UVEnd {
						return parametric.Outside, false, e
					}
					sum += winding(p, path) - winding(p, []geom.Vec2{path[0], path[len(path)-1]})
					continue
				}
				if c, ok := u.Curve.(SplineUV); ok {
					path, known, e := splineTrimChordPath(ctx, c, q, &limit)
					if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrUnrepresentable) {
						return parametric.Outside, false, nil
					}
					if e != nil || !known {
						return parametric.Outside, false, e
					}
					if path[0] != u.UVStart || path[len(path)-1] != u.UVEnd {
						return parametric.Outside, false, nil
					}
					if len(path) > 2 {
						// Replace this chord's winding contribution by
						// the certified refined chord chain.
						sum += winding(p, path) - winding(p, []geom.Vec2{path[0], path[len(path)-1]})
					}
					continue
				}
				return parametric.Outside, false, nil
			}
			positive, negative := true, true
			for _, c := range corners {
				o, e := geom.Orient2D(u.UVStart, u.UVEnd, c)
				if e != nil {
					return parametric.Outside, false, e
				}
				positive, negative = positive && o > 0, negative && o < 0
			}
			if !positive && !negative {
				return parametric.Outside, false, nil
			}
		}
		sum += winding(p, loop)
	}
	switch sum {
	case 0:
		return parametric.Outside, true, nil
	case 1:
		return parametric.Inside, true, nil
	default:
		return parametric.Outside, false, fmt.Errorf("trimmed: invalid certified winding %d", sum)
	}
}

func splineTrimChordPath(ctx context.Context, c SplineUV, q geom.UVBox, limit *int) ([]geom.Vec2, bool, error) {
	o := geom.SurfacePatchOptions{}
	model, e := geom.NewExactSplineCurve(c.Curve, o)
	if e != nil {
		return nil, false, e
	}
	model, e = model.Restrict(c.Range.Min, c.Range.Max, o)
	if e != nil {
		return nil, false, e
	}
	var points []geom.Vec2
	point := func(curve *geom.ExactSplineCurve, x float64) (geom.Vec2, bool) {
		p, e := curve.ExactPoint(x)
		return geom.Vec2{X: p.X, Y: p.Y}, e == nil && p.Z == 0
	}
	stack := []*geom.ExactSplineCurve{model}
	for len(stack) != 0 {
		if e := ctx.Err(); e != nil {
			return nil, false, e
		}
		if *limit <= 0 {
			return nil, false, nil
		}
		*limit = *limit - 1
		curve := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		box, e := curve.ProjectionBounds()
		if e != nil {
			return nil, false, e
		}
		a, b := curve.Domain()
		if box.Max.X < q.Min.X || box.Min.X > q.Max.X || box.Max.Y < q.Min.Y || box.Min.Y > q.Max.Y {
			p, ok := point(curve, a)
			if !ok {
				return nil, false, nil
			}
			points = append(points, p)
			continue
		}
		x := a/2 + b/2
		if x <= a || x >= b {
			return nil, false, nil
		}
		l, r, e := curve.Split(x, o)
		if e != nil {
			return nil, false, e
		}
		stack = append(stack, r, l)
	}
	_, b := model.Domain()
	p, ok := point(model, b)
	if !ok {
		return nil, false, nil
	}
	points = append(points, p)
	if c.Reversed {
		slices.Reverse(points)
	}
	return points, true, nil
}
