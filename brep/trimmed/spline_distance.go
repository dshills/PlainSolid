package trimmed

import (
	"container/heap"
	"context"
	"errors"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineBoundaryDistanceOptions controls a global point-to-boundary search.
// Defaults: squared gap 1e-6, 8192 cells, depth 30, 65536 queued poles and
// 128 input faces.
// Patch arithmetic limits apply independently to each exact subpatch.
type SplineBoundaryDistanceOptions struct {
	SquaredTolerance                            float64
	MaxCells, MaxDepth, MaxQueuePoles, MaxFaces int
	Patches                                     geom.SurfacePatchOptions
	// SupportIterations strengthens nonaffine cell bounds with exact
	// directional control-hull support checks. Zero leaves this off.
	SupportIterations int
}

// SplineBoundaryDistance bounds the exact squared distance to the bounded
// spline faces. With HasWitness, Point encloses a genuine point on the
// boundary, not merely a point in a surface's control hull. UV encloses its
// parameters. Complete means the outward displayed gap meets tolerance.
// Incomplete searches retain valid bounds, even on cancellation.
type SplineBoundaryDistance struct {
	SquaredMin, SquaredMax float64
	Face                   int
	UV                     geom.UVBox
	Point                  geom.Box
	HasWitness, Complete   bool
	Cells, PeakQueuePoles  int
}

type splineDistanceFace struct {
	face      Face
	rectangle bool
	affine    splineAffineProjection
}

type splineDistanceCell struct {
	patch       *geom.SurfacePatch
	lower       *big.Rat
	face, depth int
	poles       int
}

type splineDistanceQueue []splineDistanceCell

func (h splineDistanceQueue) Len() int      { return len(h) }
func (h splineDistanceQueue) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h splineDistanceQueue) Less(i, j int) bool {
	if c := h[i].lower.Cmp(h[j].lower); c != 0 {
		return c < 0
	}
	return h[i].face < h[j].face
}
func (h *splineDistanceQueue) Push(x any) { *h = append(*h, x.(splineDistanceCell)) }
func (h *splineDistanceQueue) Pop() any {
	n := len(*h) - 1
	x := (*h)[n]
	(*h)[n] = splineDistanceCell{}
	*h = (*h)[:n]
	return x
}

// BoundSplineBoundaryDistance computes conservative unsigned distance bounds
// on clamped rational spline faces, including affine placements. It excludes
// a cell only after a certified trim classification and derives a lower
// bound from its exact outward model hull. It supports straight trims and
// the certified spline, tube, and native exact Bézier trims used by section Booleans.
// The caller must supply embedded UV loops on each support. Unresolved
// trim cells are kept; no tessellation distance is substituted.
func (s Solid) BoundSplineBoundaryDistance(ctx context.Context, point geom.Vec3,
	o SplineBoundaryDistanceOptions) (r SplineBoundaryDistance, err error) {
	r.SquaredMax, r.Face = math.Inf(1), -1
	if ctx == nil || !point.IsFinite() || len(s.faces) == 0 {
		return r, geom.ErrInvalid
	}
	if o.SquaredTolerance == 0 {
		o.SquaredTolerance = 1e-6
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 30
	}
	if o.MaxQueuePoles == 0 {
		o.MaxQueuePoles = 65536
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 128
	}
	if !finite(o.SquaredTolerance) || o.SquaredTolerance <= 0 || o.MaxCells < 1 || o.MaxCells > 1<<20 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxQueuePoles < 4 || o.MaxQueuePoles > 1<<22 || o.MaxFaces < 1 || o.MaxFaces > 4096 || o.SupportIterations < 0 || o.SupportIterations > 64 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if len(s.faces) > o.MaxFaces {
		return r, parametric.ErrBudget
	}
	faces := make([]splineDistanceFace, len(s.faces))
	var queue splineDistanceQueue
	queued := 0
	var upper *big.Rat
	record := func(i int, uv geom.UVBox, b geom.Box) {
		_, d := splinePointBoxSquared(point, b)
		if upper == nil || d.Cmp(upper) < 0 {
			upper = d
			r.Face, r.UV, r.Point, r.HasWitness = i, uv, b, true
			r.SquaredMax = outwardSplineDistance(d, false)
		}
	}
	enqueue := func(p *geom.SurfacePatch, i, depth int) error {
		b, _ := p.Bounds()
		lo, _ := splinePointBoxSquared(point, b)
		if x := faces[i].affine.lower; x != nil && x.Cmp(lo) > 0 {
			lo = x
		}
		if upper != nil && lo.Cmp(upper) >= 0 {
			return nil
		}
		if o.SupportIterations > 0 && faces[i].affine.chart == nil {
			support, e := geom.BoundPointSurfaceSupport(ctx, point, p, geom.PointSurfaceSupportOptions{
				MaxIterations: o.SupportIterations, MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits,
			})
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return e
			}
			if support.HasBound && support.SquaredLower.Cmp(lo) > 0 {
				lo = support.SquaredLower
			}
			if upper != nil && lo.Cmp(upper) >= 0 {
				return nil
			}
		}
		u, v := p.ControlShape()
		if u > (o.MaxQueuePoles-queued)/v {
			return parametric.ErrBudget
		}
		n := u * v
		heap.Push(&queue, splineDistanceCell{p, lo, i, depth, n})
		queued += n
		r.PeakQueuePoles = max(r.PeakQueuePoles, queued)
		return nil
	}
	patches := make([]*geom.SurfacePatch, len(s.faces))
	lineProofs := massAffineProofs{left: 4000000, values: make(map[massProofKey]geom.TubeAffineLineCertificate)}
	winding := newSplineWindingCache(len(s.faces))
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		base, placement := parametric.SurfaceMapping(f.Surface)
		b, ok := base.(*geom.BSplineSurface)
		if !ok {
			return r, geom.ErrInvalid
		}
		f = cloneFace(f)
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for j, use := range loop {
				if use.Curve == nil {
					continue
				}
				_, _, tube := trimTube(use.Curve)
				_, _, section := trimCertifiedSpline(use.Curve)
				_, spline := use.Curve.(SplineUV)
				_, exact := exactRationalDistanceTrim(use.Curve)
				if !tube && !section && !spline && !exact {
					return r, geom.ErrInvalid
				}
				if tube {
					straight, e := lineProofs.zeroStrip(ctx, use)
					if e != nil {
						return r, e
					}
					if straight {
						loop[j].Curve = nil
					}
				}
			}
		}
		p, e := geom.NewSurfacePatch(b, o.Patches)
		if e != nil {
			return r, e
		}
		d, e := faceUVBounds(f)
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
		p, e = p.Transformed(placement)
		if e != nil {
			return r, e
		}
		patches[i] = p
		faces[i] = splineDistanceFace{face: f, rectangle: splineDistanceRectangle(f, d)}
		projection, e := affineSplineDistanceProjection(ctx, p, faces[i], point, o.Patches)
		if e != nil {
			return r, e
		}
		faces[i].affine = projection
		if projection.hit {
			record(i, projection.uv, projection.point)
		}
		if e := enqueue(p, i, 0); e != nil {
			return r, e
		}
	}
	for i, f := range faces {
		for _, l := range append([][]Use{f.face.Outer}, f.face.Holes...) {
			for _, use := range l {
				if e := ctx.Err(); e != nil {
					return r, e
				}
				if use.Curve == nil && f.affine.chart != nil {
					uv, p, e := affineSplineEdgeWitness(f.affine.chart, use, point)
					if e != nil {
						return r, e
					}
					record(i, uv, p)
					continue
				}
				uv, p, ok, e := splineDistanceTrimWitness(ctx, patches[i], use, .5)
				if e != nil {
					return r, e
				}
				if ok {
					record(i, uv, p)
				}
				if use.Curve == nil {
					uv, p, e := splineDistanceStraightWitness(ctx, patches[i], f, use, point)
					if e != nil {
						return r, e
					}
					record(i, uv, p)
				}
			}
		}
	}
	for len(queue) != 0 {
		front := queue[0]
		lower := front.lower
		if upper != nil && upper.Cmp(lower) < 0 {
			lower = upper
		}
		r.SquaredMin = outwardSplineDistance(lower, true)
		if upper != nil && finite(r.SquaredMax) && r.SquaredMax-r.SquaredMin <= o.SquaredTolerance {
			r.Complete = true
			return r, nil
		}
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if r.Cells == o.MaxCells {
			return r, parametric.ErrBudget
		}
		cell := heap.Pop(&queue).(splineDistanceCell)
		queued -= cell.poles
		r.Cells++
		f, d := faces[cell.face], cell.patch.Domain()
		box := geom.UVBox{Min: geom.Vec2{X: d.UMin, Y: d.VMin}, Max: geom.Vec2{X: d.UMax, Y: d.VMax}}
		if !f.rectangle {
			loc, known, e := winding.classify(ctx, cell.face, f.face, box)
			if e != nil {
				return r, e
			}
			if known && loc == parametric.Outside {
				continue
			}
		}
		mid := geom.Vec2{X: d.UMin/2 + d.UMax/2, Y: d.VMin/2 + d.VMax/2}
		uv := geom.UVBox{Min: mid, Max: mid}
		loc, known := parametric.Inside, f.rectangle
		if !known {
			var e error
			loc, known, e = winding.classify(ctx, cell.face, f.face, uv)
			if e != nil {
				return r, e
			}
		}
		if known && loc == parametric.Inside {
			x, e := cell.patch.EvaluateJet(mid.X, mid.Y)
			if e != nil {
				return r, e
			}
			record(cell.face, uv, x.PointBounds)
			if cell.lower.Cmp(upper) >= 0 {
				continue
			}
		}
		if cell.depth == o.MaxDepth {
			return r, parametric.ErrBudget
		}
		u := (d.UMax-d.UMin)/(patches[cell.face].Domain().UMax-patches[cell.face].Domain().UMin) >=
			(d.VMax-d.VMin)/(patches[cell.face].Domain().VMax-patches[cell.face].Domain().VMin)
		canU, canV := d.UMin < mid.X && mid.X < d.UMax, d.VMin < mid.Y && mid.Y < d.VMax
		if !canU && !canV {
			return r, geom.ErrUnrepresentable
		}
		u = canU && (u || !canV)
		var a, b *geom.SurfacePatch
		var e error
		if u {
			a, b, e = cell.patch.SplitU(mid.X)
		} else {
			a, b, e = cell.patch.SplitV(mid.Y)
		}
		if e != nil {
			return r, e
		}
		for _, p := range [2]*geom.SurfacePatch{a, b} {
			if e := enqueue(p, cell.face, cell.depth+1); e != nil {
				return r, e
			}
		}
	}
	if upper == nil {
		return r, geom.ErrConvergence
	}
	r.SquaredMin = outwardSplineDistance(upper, true)
	r.Complete = finite(r.SquaredMax) && r.SquaredMax-r.SquaredMin <= o.SquaredTolerance
	if !r.Complete {
		return r, geom.ErrUnrepresentable
	}
	return r, nil
}

func splineDistanceRectangle(f Face, d geom.ParameterRectangle) bool {
	if len(f.Outer) != 4 || len(f.Holes) != 0 {
		return false
	}
	corners := insetCorners(d)
	for _, u := range f.Outer {
		valid := false
		for i, p := range corners {
			valid = valid || u.Curve == nil && u.UVStart == p && u.UVEnd == corners[(i+1)%4]
		}
		if !valid {
			return false
		}
	}
	return true
}

func splineDistanceStraightWitness(ctx context.Context, p *geom.SurfacePatch, f splineDistanceFace, u Use,
	point geom.Vec3) (geom.UVBox, geom.Box, error) {
	q := u.UVStart
	j, e := p.EvaluateJet(q.X, q.Y)
	if e != nil {
		return geom.UVBox{}, geom.Box{}, e
	}
	if !f.rectangle {
		return geom.UVBox{Min: q, Max: q}, j.PointBounds, nil
	}
	// On a rectangle, a boundary support line is isoparametric. A bounded
	// independent curve-distance search supplies a good valid sample near
	// its nearest point; it is never used as a lower bound on the face.
	axis, fixed := 0, q.X
	if q.X != u.UVEnd.X {
		axis, fixed = 1, q.Y
	}
	curve, e := p.IsoCurve(axis, fixed, geom.SurfacePatchOptions{})
	if e != nil {
		return geom.UVBox{Min: q, Max: q}, j.PointBounds, nil
	}
	r, e := geom.BoundPointExactCurveDistance(ctx, point, curve, geom.CurveDistanceOptions{SquaredTolerance: 1e-6, MaxCells: 32, MaxOperations: 200000})
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return geom.UVBox{}, geom.Box{}, e
	}
	if r.HasWitness {
		if axis == 0 {
			q.Y = r.Parameter
		} else {
			q.X = r.Parameter
		}
		return geom.UVBox{Min: q, Max: q}, r.PointBounds, nil
	}
	return geom.UVBox{Min: q, Max: q}, j.PointBounds, nil
}

type rationalDistanceTrim interface {
	EvaluateRational(*big.Rat) ([2]*big.Rat, [2]*big.Rat, error)
}

func exactRationalDistanceTrim(c PCurve) (rationalDistanceTrim, bool) {
	switch q := c.(type) {
	case ExactQuadraticUV:
		return q, true
	case ExactBezierUV:
		return q, true
	default:
		return nil, false
	}
}

func splineDistanceTrimWitness(ctx context.Context, p *geom.SurfacePatch, u Use, t float64) (geom.UVBox, geom.Box, bool, error) {
	var uv geom.UVBox
	var ok bool
	var e error
	if u.Curve == nil {
		x, a := exactFloatMix(u.UVStart.X, u.UVEnd.X, t)
		y, b := exactFloatMix(u.UVStart.Y, u.UVEnd.Y, t)
		if !a || !b {
			return uv, geom.Box{}, false, nil
		}
		q := geom.Vec2{X: x, Y: y}
		j, e := p.EvaluateJet(q.X, q.Y)
		return geom.UVBox{Min: q, Max: q}, j.PointBounds, e == nil, e
	}
	if q, yes := exactRationalDistanceTrim(u.Curve); yes {
		xy, _, e := q.EvaluateRational(ratInset(t))
		if e != nil {
			return uv, geom.Box{}, false, e
		}
		lx, hx, e := outwardTrimRational(xy[0])
		if e != nil {
			return uv, geom.Box{}, false, e
		}
		ly, hy, e := outwardTrimRational(xy[1])
		if e != nil {
			return uv, geom.Box{}, false, e
		}
		j, e := p.EvaluateRational(ctx, xy[0], xy[1], geom.SurfacePatchEvaluationOptions{})
		uv = geom.UVBox{Min: geom.Vec2{X: lx, Y: ly}, Max: geom.Vec2{X: hx, Y: hy}}
		return uv, j.PointBounds, e == nil, e
	}
	if tube, reverse, yes := trimTube(u.Curve); yes {
		if reverse {
			t = 1 - t
		}
		if t == 0 || t == 1 {
			uv, e = tube.EvaluateBounds(t)
			ok = e == nil
		} else {
			uv, ok, e = tube.EncloseUV(ctx, math.Nextafter(t, 0), math.Nextafter(t, 1), geom.SurfaceTubeIntervalOptions{MaxIterations: 2})
		}
	} else if curve, reverse, yes := trimCertifiedSpline(u.Curve); yes {
		if reverse {
			t = 1 - t
		}
		uv, ok, e = curve.EncloseSplineUV(ctx, max(0, math.Nextafter(t, -1)), min(1, math.Nextafter(t, 2)), geom.SurfaceArcIntervalOptions{MaxIterations: 2})
	}
	if e != nil {
		if errors.Is(e, geom.ErrConvergence) || errors.Is(e, intersect.ErrUnresolved) {
			return uv, geom.Box{}, false, nil
		}
		return uv, geom.Box{}, false, e
	}
	if !ok {
		return uv, geom.Box{}, false, nil
	}
	d := p.Domain()
	if uv.Min.X < d.UMin || uv.Max.X > d.UMax || uv.Min.Y < d.VMin || uv.Max.Y > d.VMax {
		return uv, geom.Box{}, false, nil
	}
	d.UMin, d.UMax = expandSplineWitnessAxis(uv.Min.X, uv.Max.X, d.UMin, d.UMax)
	d.VMin, d.VMax = expandSplineWitnessAxis(uv.Min.Y, uv.Max.Y, d.VMin, d.VMax)
	x, e := p.Trim(d)
	if e != nil {
		return uv, geom.Box{}, false, e
	}
	b, e := x.Bounds()
	return uv, b, e == nil, e
}

func expandSplineWitnessAxis(lo, hi, mn, mx float64) (float64, float64) {
	lo, hi = max(lo, mn), min(hi, mx)
	if lo == hi {
		lo, hi = max(mn, math.Nextafter(lo, math.Inf(-1))), min(mx, math.Nextafter(hi, math.Inf(1)))
	}
	return lo, hi
}

func splinePointBoxSquared(p geom.Vec3, b geom.Box) (*big.Rat, *big.Rat) {
	lo, hi := new(big.Rat), new(big.Rat)
	for i, x := range [3]float64{p.X, p.Y, p.Z} {
		a := new(big.Rat).Sub(ratInset([3]float64{b.Min.X, b.Min.Y, b.Min.Z}[i]), ratInset(x))
		z := new(big.Rat).Sub(ratInset([3]float64{b.Max.X, b.Max.Y, b.Max.Z}[i]), ratInset(x))
		inside := a.Sign() <= 0 && z.Sign() >= 0
		a.Mul(a, a)
		z.Mul(z, z)
		if a.Cmp(z) > 0 {
			a, z = z, a
		}
		if !inside {
			lo.Add(lo, a)
		}
		hi.Add(hi, z)
	}
	return lo, hi
}

func outwardSplineDistance(x *big.Rat, lower bool) float64 {
	lo, hi := rationalFloatRange(x)
	if lower {
		return lo
	}
	return hi
}
