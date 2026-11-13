package geom

// Parabola has vertex at frame origin and focus at local (focalLength,0,0).
// Its dimensionless parameter t gives local (f*t*t,2*f*t,0).
type Parabola struct {
	frame Frame
	focal float64
}

func NewParabola(frame Frame, focalLength float64) (Parabola, error) {
	if !frame.valid() || !finite(focalLength) || focalLength <= 0 {
		return Parabola{}, ErrInvalid
	}
	return Parabola{frame, focalLength}, nil
}
func (p Parabola) Parameters() (Frame, float64) { return p.frame, p.focal }
func (p Parabola) Eval(t float64) (Vec3, error) {
	if !p.frame.valid() || p.focal <= 0 || !finite(t) {
		return Vec3{}, ErrInvalid
	}
	v := p.frame.ToWorld(Vec3{p.focal * t * t, 2 * p.focal * t, 0})
	if !v.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return v, nil
}
func (p Parabola) Derivative(t float64) (Vec3, error) {
	if !p.frame.valid() || p.focal <= 0 || !finite(t) {
		return Vec3{}, ErrInvalid
	}
	v := p.frame.x.Scale(2 * p.focal * t).Add(p.frame.y.Scale(2 * p.focal))
	if !v.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return v, nil
}
func (p Parabola) SecondDerivative(t float64) (Vec3, error) {
	if _, e := p.Derivative(t); e != nil {
		return Vec3{}, e
	}
	return p.frame.x.Scale(2 * p.focal), nil
}

// RationalBSpline returns a quadratic polynomial arc on [start,end], preserving
// the analytic parameter. The conic representation is exact in real arithmetic;
// constructing float64 poles incurs ordinary rounding, not a fitting tolerance.
func (p Parabola) RationalBSpline(start, end float64) (*BSpline, error) {
	if !finite(start) || !finite(end) || start >= end {
		return nil, ErrDomain
	}
	if _, e := p.Eval(start); e != nil {
		return nil, e
	}
	if _, e := p.Eval(end); e != nil {
		return nil, e
	}
	local := []Vec3{{p.focal * start * start, 2 * p.focal * start, 0}, {p.focal * start * end, p.focal * (start + end), 0}, {p.focal * end * end, 2 * p.focal * end, 0}}
	for i := range local {
		local[i] = p.frame.ToWorld(local[i])
	}
	return NewBSpline(2, local, []float64{start, start, start, end, end, end}, nil)
}

// Hyperbola uses the rational parameter -1<t<1 on one branch of
// x²/a²-y²/b²=1. Local coordinates are (branch*a*(1+t²)/(1-t²),
// 2*b*t/(1-t²),0). At the excluded parameter endpoints the curve approaches its asymptotes at infinity.
// This parameter is tanh(h/2), where h is the usual hyperbolic angle.
type Hyperbola struct {
	frame  Frame
	a, b   float64
	branch int
}

// NewHyperbola constructs the positive-x branch.
func NewHyperbola(frame Frame, a, b float64) (Hyperbola, error) {
	return NewHyperbolaBranch(frame, a, b, 1)
}

// NewHyperbolaBranch accepts branch +1 or -1, reflecting the local x coordinate.
func NewHyperbolaBranch(frame Frame, a, b float64, branch int) (Hyperbola, error) {
	if !frame.valid() || !finite(a) || !finite(b) || a <= 0 || b <= 0 || (branch != 1 && branch != -1) {
		return Hyperbola{}, ErrInvalid
	}
	return Hyperbola{frame, a, b, branch}, nil
}
func (h Hyperbola) Parameters() (Frame, float64, float64, int) { return h.frame, h.a, h.b, h.branch }
func (h Hyperbola) check(t float64) error {
	if !h.frame.valid() || h.a <= 0 || h.b <= 0 || (h.branch != 1 && h.branch != -1) || !finite(t) {
		return ErrInvalid
	}
	if t <= -1 || t >= 1 {
		return ErrDomain
	}
	return nil
}
func (h Hyperbola) Eval(t float64) (Vec3, error) {
	if e := h.check(t); e != nil {
		return Vec3{}, e
	}
	w := (1 - t) * (1 + t)
	v := h.frame.ToWorld(Vec3{float64(h.branch) * h.a * (1 + t*t) / w, 2 * h.b * t / w, 0})
	if !v.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return v, nil
}
func (h Hyperbola) Derivative(t float64) (Vec3, error) {
	if e := h.check(t); e != nil {
		return Vec3{}, e
	}
	w := (1 - t) * (1 + t)
	v := h.frame.x.Scale(4 * float64(h.branch) * h.a * t / w / w).Add(h.frame.y.Scale(2 * h.b * (1 + t*t) / w / w))
	if !v.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return v, nil
}
func (h Hyperbola) SecondDerivative(t float64) (Vec3, error) {
	if e := h.check(t); e != nil {
		return Vec3{}, e
	}
	w := (1 - t) * (1 + t)
	v := h.frame.x.Scale(4 * float64(h.branch) * h.a * (1 + 3*t*t) / w / w / w).Add(h.frame.y.Scale(4 * h.b * t * (3 + t*t) / w / w / w))
	if !v.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return v, nil
}

// RationalBSpline returns an exact-form positive-weight quadratic arc with the
// same rational parameter and domain [start,end], strictly inside (-1,1).
// Floating-point construction rounds poles/weights; no approximation fitting is
// performed. Endpoint proximity to asymptotes increases numerical conditioning.
func (h Hyperbola) RationalBSpline(start, end float64) (*BSpline, error) {
	if e := h.check(start); e != nil {
		return nil, e
	}
	if e := h.check(end); e != nil {
		return nil, e
	}
	if start >= end {
		return nil, ErrDomain
	}
	products := []float64{start * start, start * end, end * end}
	ys := []float64{2 * start, start + end, 2 * end}
	weights := []float64{(1 - start) * (1 + start), 1 - start*end, (1 - end) * (1 + end)}
	poles := make([]Vec3, 3)
	for i, w := range weights {
		poles[i] = h.frame.ToWorld(Vec3{float64(h.branch) * h.a * (1 + products[i]) / w, h.b * ys[i] / w, 0})
	}
	return NewBSpline(2, poles, []float64{start, start, start, end, end, end}, weights)
}
