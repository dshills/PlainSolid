// Package intersect implements certified spline–plane branch discovery and
// local numerical intersection of parametric surfaces. General surface
// tracing follows supplied transverse branches; it does not certify
// complete enumeration or resolve general tangencies and coincidences.
package intersect

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/geom"
)

var (
	ErrUnresolved = errors.New("intersect: corrector did not converge")
	ErrSingular   = errors.New("intersect: singular or tangent surface section")
	ErrDomain     = errors.New("intersect: corrector reached patch boundary")
)

// Patch defines a surface and finite parameter limits.
type Patch struct {
	Surface  geom.Surface
	Min, Max geom.Vec2
}
type Parameters struct{ A, B geom.Vec2 }
type Sample struct {
	Parameters Parameters
	Point      geom.Vec3
	Tangent    geom.Vec3
	Residual   float64
}

// Options sets distance tolerance, approximate step size in model space, and
// work limits. MaxNewton defaults to 16 and Steps must be 1..100000.
type Options struct {
	Tolerance, Step  float64
	Steps, MaxNewton int
	Reverse          bool
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (p Patch) valid() bool {
	return p.Surface != nil && p.Min.X < p.Max.X && p.Min.Y < p.Max.Y && finite(p.Max.X-p.Min.X) && finite(p.Max.Y-p.Min.Y)
}
func (p Patch) accepts(v geom.Vec2) bool {
	return v.X >= p.Min.X && v.X <= p.Max.X && v.Y >= p.Min.Y && v.Y <= p.Max.Y
}

type frame struct {
	p, q       geom.Vec3
	a, b, c, d geom.Vec3
	t          geom.Vec3
}

func evaluate(a, b Patch, p Parameters) (frame, error) {
	if !a.accepts(p.A) || !b.accepts(p.B) {
		return frame{}, ErrDomain
	}
	x, e := a.Surface.Eval(p.A.X, p.A.Y)
	if e != nil {
		return frame{}, e
	}
	y, e := b.Surface.Eval(p.B.X, p.B.Y)
	if e != nil {
		return frame{}, e
	}
	du, dv, e := a.Surface.Partials(p.A.X, p.A.Y)
	if e != nil {
		return frame{}, e
	}
	dx, dy, e := b.Surface.Partials(p.B.X, p.B.Y)
	if e != nil {
		return frame{}, e
	}
	n0, e := normal(du, dv)
	if e != nil {
		return frame{}, e
	}
	n1, e := normal(dx, dy)
	if e != nil {
		return frame{}, e
	}
	cross := n0.Cross(n1)
	if !x.IsFinite() || !y.IsFinite() || cross.Length() < 1e-10 {
		return frame{}, ErrSingular
	}
	t, e := cross.Unit()
	return frame{x, y, du, dv, dx, dy, t}, e
}
func normal(a, b geom.Vec3) (geom.Vec3, error) {
	x, e := a.Unit()
	if e != nil {
		return geom.Vec3{}, ErrSingular
	}
	y, e := b.Unit()
	if e != nil {
		return geom.Vec3{}, ErrSingular
	}
	n := x.Cross(y)
	if n.Length() < 1e-12 {
		return geom.Vec3{}, ErrSingular
	}
	return n.Unit()
}

// Trace starts at a supplied near-intersection seed and follows one direction
// using an arclength predictor and Newton correction on a transverse plane.
// On error, it returns the already verified prefix. Patch boundaries do not
// trigger automatic seam crossings. Fixed Steps does not imply a closed loop
// or a complete geometric intersection. Residual is measured in model units.
func Trace(ctx context.Context, a, b Patch, seed Parameters, o Options) ([]Sample, error) {
	if ctx == nil || !a.valid() || !b.valid() || o.Steps < 1 || o.Steps > 100000 || o.MaxNewton < 0 || o.MaxNewton > 100 ||
		o.Step <= 0 || o.Tolerance <= 0 || o.Step/o.Tolerance < 8 || !finite(o.Step) || !finite(o.Tolerance) {
		return nil, fmt.Errorf("intersect: invalid trace options")
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxNewton == 0 {
		o.MaxNewton = 16
	}
	f, e := evaluate(a, b, seed)
	if e != nil {
		return nil, e
	}
	base := f.p.Scale(.5).Add(f.q.Scale(.5))
	p, f, e := correct(ctx, a, b, seed, base, f.t, o)
	if e != nil {
		return nil, e
	}
	out := make([]Sample, 0, o.Steps+1)
	for i := 0; i <= o.Steps; i++ {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		center := f.p.Scale(.5).Add(f.q.Scale(.5))
		t := f.t
		if i == 0 && o.Reverse || i > 0 && t.Dot(out[i-1].Tangent) < 0 {
			t = t.Scale(-1)
		}
		out = append(out, Sample{p, center, t, f.p.Sub(f.q).Length()})
		if i == o.Steps {
			break
		}
		// Predict parameter displacement in both tangent spaces. Project the
		// corrected point onto the plane one step ahead of the last sample.
		pred, err := predict(f, t, o.Step)
		if err != nil {
			return out, err
		}
		target := center.Add(t.Scale(o.Step))
		p, f, e = correct(ctx, a, b, shift(p, pred, 1), target, t, o)
		if e != nil {
			return out, e
		}
		if f.p.Sub(center).Dot(t) <= o.Tolerance || f.p.Sub(center).Length() > 4*o.Step {
			return out, ErrUnresolved
		}
	}
	return out, nil
}

func predict(f frame, t geom.Vec3, length float64) ([4]float64, error) {
	local := func(a, b geom.Vec3) (float64, float64, error) {
		u, e := a.Unit()
		if e != nil {
			return 0, 0, e
		}
		v, e := b.Unit()
		if e != nil {
			return 0, 0, e
		}
		c := u.Dot(v)
		d := u.Cross(v).Length()
		if d < 1e-12 {
			return 0, 0, ErrSingular
		}
		x := (t.Dot(u) - c*t.Dot(v)) / (d * d) / a.Length() * length
		y := (t.Dot(v) - c*t.Dot(u)) / (d * d) / b.Length() * length
		if !finite(x) || !finite(y) {
			return 0, 0, ErrUnresolved
		}
		return x, y, nil
	}
	x, y, e := local(f.a, f.b)
	if e != nil {
		return [4]float64{}, e
	}
	z, w, e := local(f.c, f.d)
	return [4]float64{x, y, z, w}, e
}
func shift(p Parameters, d [4]float64, s float64) Parameters {
	return Parameters{geom.Vec2{X: p.A.X + s*d[0], Y: p.A.Y + s*d[1]}, geom.Vec2{X: p.B.X + s*d[2], Y: p.B.Y + s*d[3]}}
}
func correct(ctx context.Context, a, b Patch, p Parameters, target, t geom.Vec3, o Options) (Parameters, frame, error) {
	for i := 0; i < o.MaxNewton; i++ {
		if e := ctx.Err(); e != nil {
			return p, frame{}, e
		}
		f, e := evaluate(a, b, p)
		if e != nil {
			return p, frame{}, e
		}
		err := f.q.Sub(f.p)
		plane := target.Sub(f.p).Dot(t)
		r := math.Hypot(err.Length(), plane)
		if r <= o.Tolerance {
			return p, f, nil
		}
		m := [4][5]float64{
			{f.a.X, f.b.X, -f.c.X, -f.d.X, err.X},
			{f.a.Y, f.b.Y, -f.c.Y, -f.d.Y, err.Y},
			{f.a.Z, f.b.Z, -f.c.Z, -f.d.Z, err.Z},
			{f.a.Dot(t), f.b.Dot(t), 0, 0, plane},
		}
		d, e := solve(m)
		if e != nil {
			return p, frame{}, e
		}
		accepted := false
		for j := 0; j < 12; j++ {
			x := shift(p, d, math.Ldexp(1, -j))
			if !a.accepts(x.A) || !b.accepts(x.B) {
				continue
			}
			u, v := a.Surface, b.Surface
			q, e := u.Eval(x.A.X, x.A.Y)
			if e != nil {
				return p, frame{}, e
			}
			w, e := v.Eval(x.B.X, x.B.Y)
			if e != nil {
				return p, frame{}, e
			}
			if rr := math.Hypot(q.Sub(w).Length(), target.Sub(q).Dot(t)); rr < r {
				p = x
				accepted = true
				break
			}
		}
		if !accepted {
			return p, frame{}, ErrUnresolved
		}
	}
	return p, frame{}, ErrUnresolved
}

func solve(a [4][5]float64) ([4]float64, error) {
	var x, scale [4]float64
	for j := range 4 {
		for i := range 4 {
			scale[j] = math.Max(scale[j], math.Abs(a[i][j]))
		}
		if scale[j] <= 0 || !finite(scale[j]) {
			return x, ErrSingular
		}
		for i := range 4 {
			a[i][j] /= scale[j]
		}
	}
	for i := range 4 {
		p := i
		for j := i + 1; j < 4; j++ {
			if math.Abs(a[j][i]) > math.Abs(a[p][i]) {
				p = j
			}
		}
		if math.Abs(a[p][i]) < 1e-12 {
			return x, ErrSingular
		}
		a[i], a[p] = a[p], a[i]
		d := a[i][i]
		for j := i; j < 5; j++ {
			a[i][j] /= d
		}
		for j := range 4 {
			if j == i {
				continue
			}
			d := a[j][i]
			for k := i; k < 5; k++ {
				a[j][k] -= d * a[i][k]
			}
		}
	}
	for i := range 4 {
		x[i] = a[i][4] / scale[i]
		if !finite(x[i]) {
			return x, ErrUnresolved
		}
	}
	return x, nil
}
