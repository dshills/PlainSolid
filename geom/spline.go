package geom

import (
	"fmt"
	"math"
	"sort"
)

// BSpline is an immutable rational B-spline curve with a full knot vector.
// Constructors copy input. Concurrent evaluation is safe. Degree is 1..64;
// weights must be strictly positive. NewBSpline normalizes by their maximum;
// normalization that underflows to zero is rejected. Repeated interior knots
// are supported. A zero value can be evaluated but returns ErrDomain.
type BSpline struct {
	degree         int
	poles          []Vec3
	knots, weights []float64
}
type homogeneous [4]float64

func validateKnots(degree, count int, knots []float64) error {
	if degree < 1 || degree > 64 || count <= degree || len(knots) != count+degree+1 {
		return fmt.Errorf("%w: degree, pole count or knot count", ErrInvalid)
	}
	run := 0
	for i, k := range knots {
		if !finite(k) || (i > 0 && k < knots[i-1]) {
			return fmt.Errorf("%w: knot order", ErrInvalid)
		}
		if i == 0 || k != knots[i-1] {
			run = 1
		} else {
			run++
		}
		if run > degree+1 {
			return fmt.Errorf("%w: knot multiplicity", ErrInvalid)
		}
	}
	if knots[degree] >= knots[count] || !finite(knots[len(knots)-1]-knots[0]) {
		return fmt.Errorf("%w: empty or unrepresentable knot domain", ErrInvalid)
	}
	return nil
}
func copyWeights(count int, weights []float64) ([]float64, error) {
	w := make([]float64, count)
	if weights == nil {
		for i := range w {
			w[i] = 1
		}
		return w, nil
	}
	if len(weights) != count {
		return nil, ErrInvalid
	}
	max := 0.0
	for _, x := range weights {
		if !finite(x) || x <= 0 {
			return nil, ErrInvalid
		}
		max = math.Max(max, x)
	}
	for i, x := range weights {
		w[i] = x / max
		if w[i] == 0 {
			return nil, fmt.Errorf("%w: weight dynamic range", ErrInvalid)
		}
	}
	return w, nil
}
func NewBSpline(degree int, poles []Vec3, knots, weights []float64) (*BSpline, error) {
	return newBSpline(degree, poles, knots, weights, false)
}

// NewBSplineWithExactWeights preserves the supplied binary64 weights. Use it
// when normalizing by the maximum would change their exact ratios. Each weight
// must be positive and its homogeneous pole products must be finite.
func NewBSplineWithExactWeights(degree int, poles []Vec3, knots, weights []float64) (*BSpline, error) {
	return newBSpline(degree, poles, knots, weights, true)
}

func newBSpline(degree int, poles []Vec3, knots, weights []float64, exactWeights bool) (*BSpline, error) {
	if e := validateKnots(degree, len(poles), knots); e != nil {
		return nil, e
	}
	for _, p := range poles {
		if !p.IsFinite() {
			return nil, ErrInvalid
		}
	}
	w, e := splineWeights(poles, weights, exactWeights)
	if e != nil {
		return nil, e
	}
	return &BSpline{degree, append([]Vec3(nil), poles...), append([]float64(nil), knots...), w}, nil
}

func splineWeights(poles []Vec3, weights []float64, exact bool) ([]float64, error) {
	if !exact {
		return copyWeights(len(poles), weights)
	}
	if len(weights) != len(poles) {
		return nil, ErrInvalid
	}
	for i, w := range weights {
		p := poles[i]
		if !finite(w) || w <= 0 || !finite(p.X*w) || !finite(p.Y*w) || !finite(p.Z*w) {
			return nil, ErrInvalid
		}
	}
	return append([]float64(nil), weights...), nil
}
func (c *BSpline) Degree() int {
	if c == nil {
		return 0
	}
	return c.degree
}

// PoleCount returns the number of control points without copying them.
func (c *BSpline) PoleCount() int {
	if c == nil {
		return 0
	}
	return len(c.poles)
}

// Domain returns NaNs for a nil or zero-value curve.
func (c *BSpline) Domain() (float64, float64) {
	if c == nil || len(c.poles) == 0 {
		return math.NaN(), math.NaN()
	}
	return c.knots[c.degree], c.knots[len(c.poles)]
}
func (c *BSpline) Poles() []Vec3      { return append([]Vec3(nil), c.poles...) }
func (c *BSpline) Knots() []float64   { return append([]float64(nil), c.knots...) }
func (c *BSpline) Weights() []float64 { return append([]float64(nil), c.weights...) }
func toHom(poles []Vec3, weights []float64) []homogeneous {
	h := make([]homogeneous, len(poles))
	for i, p := range poles {
		w := weights[i]
		h[i] = homogeneous{p.X * w, p.Y * w, p.Z * w, w}
	}
	return h
}
func splineSpan(count int, knots []float64, t float64) int {
	if t < knots[count] {
		return sort.Search(len(knots), func(i int) bool { return knots[i] > t }) - 1
	}
	return sort.Search(len(knots), func(i int) bool { return knots[i] >= t }) - 1
}
func evalHom(degree int, poles []homogeneous, knots []float64, t float64) homogeneous {
	return evalHomSpan(degree, poles, knots, t, splineSpan(len(poles), knots, t))
}
func evalHomSpan(degree int, poles []homogeneous, knots []float64, t float64, span int) homogeneous {
	var d [65]homogeneous // Validated constructors cap degree at 64.
	copy(d[:], poles[span-degree:span+1])
	return reduceHom(degree, knots, t, span, &d)
}
func reduceHom(degree int, knots []float64, t float64, span int, d *[65]homogeneous) homogeneous {
	for r := 1; r <= degree; r++ {
		for j := degree; j >= r; j-- {
			i := span - degree + j
			den := knots[i+degree-r+1] - knots[i]
			alpha := 0.0
			if den != 0 {
				alpha = (t - knots[i]) / den
			}
			for k := 0; k < 4; k++ {
				d[j][k] = (1-alpha)*d[j-1][k] + alpha*d[j][k]
			}
		}
	}
	return d[degree]
}
func derivHom(degree int, poles []homogeneous, knots []float64, t float64) homogeneous {
	d := make([]homogeneous, len(poles)-1)
	for i := range d {
		den := knots[i+degree+1] - knots[i+1]
		if den != 0 {
			for k := 0; k < 4; k++ {
				d[i][k] = float64(degree) * (poles[i+1][k] - poles[i][k]) / den
			}
		}
	}
	return evalHom(degree-1, d, knots[1:len(knots)-1], t)
}
func projectHom(h homogeneous) (Vec3, error) {
	if !finite(h[3]) || h[3] <= 0 {
		return Vec3{}, ErrDegenerate
	}
	p := Vec3{h[0] / h[3], h[1] / h[3], h[2] / h[3]}
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func rationalDerivative(h, d homogeneous) (Vec3, error) {
	p, e := projectHom(h)
	if e != nil {
		return Vec3{}, e
	}
	v := Vec3{(d[0] - p.X*d[3]) / h[3], (d[1] - p.Y*d[3]) / h[3], (d[2] - p.Z*d[3]) / h[3]}
	if !v.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return v, nil
}
func (c *BSpline) validParameter(t float64) bool {
	if c == nil || len(c.poles) == 0 {
		return false
	}
	a, b := c.Domain()
	return finite(t) && t >= a && t <= b
}
func (c *BSpline) Eval(t float64) (Vec3, error) {
	if !c.validParameter(t) {
		return Vec3{}, ErrDomain
	}
	return projectHom(evalHom(c.degree, toHom(c.poles, c.weights), c.knots, t))
}

// Derivative returns the right derivative at interior knots and left derivative
// at the final endpoint. At discontinuities it refers to the selected side.
func (c *BSpline) Derivative(t float64) (Vec3, error) {
	if !c.validParameter(t) {
		return Vec3{}, ErrDomain
	}
	p := toHom(c.poles, c.weights)
	return rationalDerivative(evalHom(c.degree, p, c.knots, t), derivHom(c.degree, p, c.knots, t))
}

// Sample returns count uniformly spaced parameter samples including endpoints.
// It does not promise any geometric chord error bound.
func (c *BSpline) Sample(count int) ([]Vec3, error) {
	if c == nil || len(c.poles) == 0 || count < 2 || count > 10000000 {
		return nil, ErrInvalid
	}
	a, b := c.Domain()
	p := make([]Vec3, count)
	for i := range p {
		t := a + (b-a)*(float64(i)/float64(count-1))
		if i == count-1 {
			t = b
		}
		v, e := c.Eval(t)
		if e != nil {
			return nil, e
		}
		p[i] = v
	}
	return p, nil
}

// BSplineSurface is an immutable rational tensor-product surface. Pole indexing
// is u*countV+v. Both directions use full knot vectors.
type BSplineSurface struct {
	degreeU, degreeV, countU, countV int
	poles                            []Vec3
	knotsU, knotsV, weights          []float64
}

func NewBSplineSurface(degreeU, degreeV, countU, countV int, poles []Vec3, knotsU, knotsV, weights []float64) (*BSplineSurface, error) {
	return newBSplineSurface(degreeU, degreeV, countU, countV, poles, knotsU, knotsV, weights, false)
}

// NewBSplineSurfaceWithExactWeights preserves the represented binary64
// weights rather than rescaling them. Use it when rescaling would perturb
// their exact ratios. Every weight and every homogeneous pole product must
// have a finite floating evaluator; zero weights remain invalid.
func NewBSplineSurfaceWithExactWeights(degreeU, degreeV, countU, countV int, poles []Vec3, knotsU, knotsV, weights []float64) (*BSplineSurface, error) {
	return newBSplineSurface(degreeU, degreeV, countU, countV, poles, knotsU, knotsV, weights, true)
}

func newBSplineSurface(degreeU, degreeV, countU, countV int, poles []Vec3, knotsU, knotsV, weights []float64, exactWeights bool) (*BSplineSurface, error) {
	if e := validateKnots(degreeU, countU, knotsU); e != nil {
		return nil, e
	}
	if e := validateKnots(degreeV, countV, knotsV); e != nil {
		return nil, e
	}
	if countU > len(poles)/countV || countU*countV != len(poles) {
		return nil, ErrInvalid
	}
	for _, p := range poles {
		if !p.IsFinite() {
			return nil, ErrInvalid
		}
	}
	w, e := splineWeights(poles, weights, exactWeights)
	if e != nil {
		return nil, e
	}
	return &BSplineSurface{degreeU, degreeV, countU, countV, append([]Vec3(nil), poles...), append([]float64(nil), knotsU...), append([]float64(nil), knotsV...), w}, nil
}

// Domain returns NaNs for a nil or zero-value surface.
func (s *BSplineSurface) Domain() (u0, u1, v0, v1 float64) {
	if s == nil || len(s.poles) == 0 {
		return math.NaN(), math.NaN(), math.NaN(), math.NaN()
	}
	return s.knotsU[s.degreeU], s.knotsU[s.countU], s.knotsV[s.degreeV], s.knotsV[s.countV]
}
func (s *BSplineSurface) validParameter(u, v float64) bool {
	if s == nil || len(s.poles) == 0 {
		return false
	}
	a, b, c, d := s.Domain()
	return finite(u) && finite(v) && u >= a && u <= b && v >= c && v <= d
}
func (s *BSplineSurface) activeRow(row *[65]homogeneous, i, spanV int) {
	start := i*s.countV + spanV - s.degreeV
	for j := 0; j <= s.degreeV; j++ {
		p, w := s.poles[start+j], s.weights[start+j]
		row[j] = homogeneous{p.X * w, p.Y * w, p.Z * w, w}
	}
}

func localHomDerivative(p, q homogeneous, degree int, den float64) (d homogeneous) {
	if den != 0 {
		for k := range d {
			d[k] = float64(degree) * (q[k] - p[k]) / den
		}
	}
	return d
}

// De Boor uses only the control net of the selected tensor span. The
// derivative knot vectors omit both end knots, so their spans are su-1/sv-1
// and the first contributing derivative control has the same original index.
func (s *BSplineSurface) activeHom(u, v float64, derivative bool) (h, du, dv homogeneous) {
	su, sv := splineSpan(s.countU, s.knotsU, u), splineSpan(s.countV, s.knotsV, v)
	var rows, row, vRows, work [65]homogeneous
	for i := 0; i <= s.degreeU; i++ {
		s.activeRow(&row, su-s.degreeU+i, sv)
		if derivative {
			for j := 0; j < s.degreeV; j++ {
				den := s.knotsV[sv+j+1] - s.knotsV[sv-s.degreeV+j+1]
				work[j] = localHomDerivative(row[j], row[j+1], s.degreeV, den)
			}
			vRows[i] = reduceHom(s.degreeV-1, s.knotsV[1:], v, sv-1, &work)
		}
		rows[i] = reduceHom(s.degreeV, s.knotsV, v, sv, &row)
	}
	if derivative {
		for i := 0; i < s.degreeU; i++ {
			den := s.knotsU[su+i+1] - s.knotsU[su-s.degreeU+i+1]
			work[i] = localHomDerivative(rows[i], rows[i+1], s.degreeU, den)
		}
		du = reduceHom(s.degreeU-1, s.knotsU[1:], u, su-1, &work)
		dv = reduceHom(s.degreeU, s.knotsU, u, su, &vRows)
	}
	return reduceHom(s.degreeU, s.knotsU, u, su, &rows), du, dv
}

func (s *BSplineSurface) Eval(u, v float64) (Vec3, error) {
	if !s.validParameter(u, v) {
		return Vec3{}, ErrDomain
	}
	h, _, _ := s.activeHom(u, v, false)
	return projectHom(h)
}

// Partials returns analytic first partial derivatives in u and v.
func (s *BSplineSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if !s.validParameter(u, v) {
		return Vec3{}, Vec3{}, ErrDomain
	}
	h, du, dv := s.activeHom(u, v, true)
	a, e := rationalDerivative(h, du)
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	b, e := rationalDerivative(h, dv)
	return a, b, e
}
func (s *BSplineSurface) Normal(u, v float64) (Vec3, error) {
	a, b, e := s.Partials(u, v)
	if e != nil {
		return Vec3{}, e
	}
	return a.Cross(b).Unit()
}
