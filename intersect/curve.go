package intersect

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// BranchCurve is a piecewise linear parameter curve fitted to the samples of
// one Trace branch. It keeps an immutable copy of the samples. Model-space
// evaluation uses the actual surface; it does not reproduce an exact
// intersection between samples.
type BranchCurve struct {
	patch Patch
	uv    []geom.Vec2
	t     []float64
}

func (c BranchCurve) Range() (float64, float64) {
	if len(c.t) == 0 {
		return math.NaN(), math.NaN()
	}
	return 0, c.t[len(c.t)-1]
}
func (c BranchCurve) UV(t float64) (geom.Vec2, error) {
	u, _, e := c.evaluate(t)
	return u, e
}
func (c BranchCurve) Eval(t float64) (geom.Vec3, error) {
	p, e := c.UV(t)
	if e != nil {
		return geom.Vec3{}, e
	}
	return c.patch.Surface.Eval(p.X, p.Y)
}
func (c BranchCurve) Derivative(t float64) (geom.Vec3, error) {
	p, d, e := c.evaluate(t)
	if e != nil {
		return geom.Vec3{}, e
	}
	a, b, e := c.patch.Surface.Partials(p.X, p.Y)
	if e != nil {
		return geom.Vec3{}, e
	}
	v := a.Scale(d.X).Add(b.Scale(d.Y))
	if !v.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return v, nil
}

func (c BranchCurve) evaluate(t float64) (p, d geom.Vec2, e error) {
	if len(c.t) < 2 || !finite(t) || t < 0 || t > c.t[len(c.t)-1] {
		return p, d, geom.ErrDomain
	}
	lo, hi := 0, len(c.t)-1
	for hi-lo > 1 {
		m := (lo + hi) / 2
		if t < c.t[m] {
			hi = m
		} else {
			lo = m
		}
	}
	v, w := c.uv[lo], c.uv[hi]
	h := c.t[hi] - c.t[lo]
	x := (t - c.t[lo]) / h
	return geom.Vec2{X: (1-x)*v.X + x*w.X, Y: (1-x)*v.Y + x*w.Y}, geom.Vec2{X: (w.X - v.X) / h, Y: (w.Y - v.Y) / h}, nil
}

// FitBranch creates paired p-curves over the same cumulative chord-length
// parameter. Every sample and three interior evaluations per chord must have
// mutual surface residual <= tolerance. This does not certify the fit between
// those checks. Refine the original trace if it fails.
func FitBranch(a, b Patch, samples []Sample, tolerance float64) (BranchCurve, BranchCurve, error) {
	if !a.valid() || !b.valid() || len(samples) < 2 || len(samples) > 100001 || tolerance <= 0 || !finite(tolerance) {
		return BranchCurve{}, BranchCurve{}, geom.ErrInvalid
	}
	t := make([]float64, len(samples))
	u, v := make([]geom.Vec2, len(samples)), make([]geom.Vec2, len(samples))
	for i, p := range samples {
		u[i], v[i] = p.Parameters.A, p.Parameters.B
		if !p.Point.IsFinite() || !a.accepts(u[i]) || !b.accepts(v[i]) {
			return BranchCurve{}, BranchCurve{}, geom.ErrDomain
		}
		if i == 0 {
			continue
		}
		t[i] = t[i-1] + p.Point.Sub(samples[i-1].Point).Length()
		if !finite(t[i]) || t[i] <= t[i-1] {
			return BranchCurve{}, BranchCurve{}, fmt.Errorf("intersect: unresolved chord length")
		}
	}
	c, d := BranchCurve{a, u, t}, BranchCurve{b, v, t}
	for i, h := range t {
		count := 4
		if i == len(t)-1 {
			count = 1
		}
		for j := 0; j < count; j++ {
			x := h
			if j != 0 {
				x += (t[i+1] - h) * float64(j) / 4
			}
			p, e := c.Eval(x)
			if e != nil {
				return BranchCurve{}, BranchCurve{}, e
			}
			q, e := d.Eval(x)
			if e != nil {
				return BranchCurve{}, BranchCurve{}, e
			}
			if !p.IsFinite() || !q.IsFinite() || p.Sub(q).Length() > tolerance {
				return BranchCurve{}, BranchCurve{}, ErrUnresolved
			}
		}
	}
	return c, d, nil
}

// TrimCurve adapts an interval of a fitted branch to normalized [0,1]
// parameter-space evaluation. It can serve as a trimmed.Face boundary when
// the caller's topology and model-space tolerance match.
type TrimCurve struct {
	Branch     BranchCurve
	Start, End float64
}

func (c TrimCurve) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 || !finite(c.Start) || !finite(c.End) || c.Start == c.End {
		return geom.Vec2{}, geom.ErrDomain
	}
	return c.Branch.UV((1-t)*c.Start + t*c.End)
}

// DerivativeUV returns the derivative with respect to normalized t. At a
// branch knot it selects the following chord, or the preceding chord at
// the final endpoint.
func (c TrimCurve) DerivativeUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 || !finite(c.Start) || !finite(c.End) || c.Start == c.End {
		return geom.Vec2{}, geom.ErrDomain
	}
	_, d, e := c.Branch.evaluate((1-t)*c.Start + t*c.End)
	h := c.End - c.Start
	return geom.Vec2{X: d.X * h, Y: d.Y * h}, e
}
