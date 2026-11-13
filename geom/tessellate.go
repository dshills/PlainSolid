package geom

import (
	"math"
	"sort"
)

// CurveTessellationOptions controls sampled adaptive subdivision. Zero fields
// select defaults: chord tolerance 1e-3, initial segments 16, max depth 24,
// and max evaluations 100000. ChordTolerance is a sampling criterion, not a
// certified Hausdorff bound for arbitrary curves.
type CurveTessellationOptions struct {
	ChordTolerance                            float64
	InitialSegments, MaxDepth, MaxEvaluations int
}

// CurveSegment is an independent chord. At spline discontinuities the prior
// segment ends at the left limit and the next segment starts at the right limit.
type CurveSegment struct {
	StartParameter, EndParameter float64
	Start, End                   Vec3
}

// TessellateCurve checks quarter/mid/three-quarter points against each chord.
// Initial subdivision reduces aliasing but cannot certify arbitrary curves.
// Ordinary and periodic spline knots are mandatory breakpoints. Work limits
// return ErrConvergence
// with no partial output. Segments are sorted by increasing parameter.
func TessellateCurve(curve Curve, start, end float64, options CurveTessellationOptions) ([]CurveSegment, error) {
	if curve == nil || !finite(start) || !finite(end) || start >= end || !finite(end-start) {
		return nil, ErrDomain
	}
	o := options
	if o.ChordTolerance == 0 {
		o.ChordTolerance = 1e-3
	}
	if o.InitialSegments == 0 {
		o.InitialSegments = 16
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxEvaluations == 0 {
		o.MaxEvaluations = 100000
	}
	if !finite(o.ChordTolerance) || o.ChordTolerance <= 0 || o.InitialSegments < 1 || o.InitialSegments > 1000000 || o.MaxDepth < 1 || o.MaxDepth > 64 || o.MaxEvaluations < 5 || o.InitialSegments > o.MaxEvaluations/5 {
		return nil, ErrInvalid
	}
	breaks := make([]float64, 0, o.InitialSegments+1)
	for i := 0; i <= o.InitialSegments; i++ {
		t := start + (end-start)*(float64(i)/float64(o.InitialSegments))
		if i == o.InitialSegments {
			t = end
		}
		breaks = append(breaks, t)
	}
	spline, isSpline := curve.(*BSpline)
	var hom []homogeneous
	if isSpline {
		if !spline.validParameter(start) || !spline.validParameter(end) {
			return nil, ErrDomain
		}
		hom = toHom(spline.poles, spline.weights)
		for _, k := range spline.knots {
			if k > start && k < end {
				breaks = append(breaks, k)
			}
		}
		sort.Float64s(breaks)
	}
	if periodic, ok := curve.(*PeriodicBSpline); ok {
		knots, e := periodic.periodicBreakpoints(start, end, o.MaxEvaluations/5)
		if e != nil {
			return nil, e
		}
		breaks = append(breaks, knots...)
		sort.Float64s(breaks)
	}
	evaluations := 0
	evaluate := func(t float64, left bool) (Vec3, error) {
		if evaluations >= o.MaxEvaluations {
			return Vec3{}, ErrConvergence
		}
		evaluations++
		if isSpline && left {
			span := sort.SearchFloat64s(spline.knots, t) - 1
			return projectHom(evalHomSpan(spline.degree, hom, spline.knots, t, span))
		}
		p, e := curve.Eval(t)
		if e != nil {
			return Vec3{}, e
		}
		if !p.IsFinite() {
			return Vec3{}, ErrInvalid
		}
		return p, nil
	}
	segments := make([]CurveSegment, 0, o.InitialSegments)
	var subdivide func(float64, float64, Vec3, Vec3, int) error
	subdivide = func(a, b float64, pa, pb Vec3, depth int) error {
		mid := a + (b-a)/2
		if mid == a || mid == b {
			return ErrConvergence
		}
		var samples [3]Vec3
		maxDeviation := 0.0
		for i := range samples {
			t := a + (b-a)*(float64(i+1)/4)
			p, e := evaluate(t, false)
			if e != nil {
				return e
			}
			samples[i] = p
			q, _, e := ClosestOnSegment(p, pa, pb)
			if e != nil {
				return e
			}
			maxDeviation = math.Max(maxDeviation, p.Distance(q))
		}
		if maxDeviation <= o.ChordTolerance {
			segments = append(segments, CurveSegment{a, b, pa, pb})
			return nil
		}
		if depth >= o.MaxDepth {
			return ErrConvergence
		}
		if e := subdivide(a, mid, pa, samples[1], depth+1); e != nil {
			return e
		}
		return subdivide(mid, b, samples[1], pb, depth+1)
	}
	for i := 1; i < len(breaks); i++ {
		a, b := breaks[i-1], breaks[i]
		if a == b {
			continue
		}
		pa, e := evaluate(a, false)
		if e != nil {
			return nil, e
		}
		pb, e := evaluate(b, true)
		if e != nil {
			return nil, e
		}
		if e = subdivide(a, b, pa, pb, 0); e != nil {
			return nil, e
		}
	}
	return segments, nil
}
