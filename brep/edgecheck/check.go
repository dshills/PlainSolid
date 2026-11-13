// Package edgecheck certifies continuous agreement of rational Bezier curves
// at the same normalized parameter. It checks represented geometry in real
// arithmetic, not floating evaluation error or parameter-independent distance.
package edgecheck

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
)

// Curve has positive rational weights (nil means one). Reversed compares the
// control sequence backwards, corresponding to t -> 1-t. Constant curves are
// supported. Poles and weights are copied into exact rational arithmetic.
type Curve struct {
	Poles    []geom.Vec3
	Weights  []float64
	Reversed bool
}

// PlanarCurve is a UV-space rational Bezier curve.
type PlanarCurve struct {
	Poles    []geom.Vec2
	Weights  []float64
	Reversed bool
}

// Chart maps (u,v) to Origin+u*U+v*V. Its independent axes need not be unit or
// perpendicular. Mapping is performed exactly in homogeneous coordinates.
type Chart struct{ Origin, U, V geom.Vec3 }
type Status uint8

const (
	Unresolved Status = iota
	Certified
	Exceeded
)

var ErrInvalid = errors.New("edgecheck: invalid curve, chart, tolerance or options")
var ErrLimit = errors.New("edgecheck: proof resource limit exceeded")

// Options defaults: degree 32 (maximum configurable 128), two million work
// units, 100000 visited intervals, subdivision depth 48 (maximum 256), and
// 16384 bits per checked rational numerator/denominator. Temporary expressions
// may exceed the stored-value bit bound. Zero selects defaults.
type Options struct{ MaxDegree, MaxWork, MaxIntervals, MaxDepth, MaxBits int }

// Witness gives an exact normalized parameter where agreement exceeds tolerance.
// Exact points and squared distance are caller-owned; PointA/B are rounded
// conveniences and can coincide or overflow despite finite exact witnesses.
type Witness struct {
	Parameter                *big.Rat
	ExactPointA, ExactPointB [3]*big.Rat
	PointA, PointB           geom.Vec3
	SquaredDistance          *big.Rat
}

// Report must be checked even if error is nil. Certified proves discrepancy <=
// Tolerance for every t in [0,1]. Exceeded includes an exact witness. Unresolved
// means the proof could not decide within its limits, not a tolerance violation.
// SquaredUpperBound, when nonnil, is a valid conservative global bound, tightened
// to tolerance squared after certification. It may be loose when unresolved.
type Report struct {
	Status            Status
	Tolerance         float64
	SquaredUpperBound *big.Rat
	Witness           *Witness
	Reason            string
	Work, Intervals   int
}
type q = *big.Rat
type hom [4]q

func num(x float64) q       { return new(big.Rat).SetFloat64(x) }
func add(a, b q) q          { return new(big.Rat).Add(a, b) }
func sub(a, b q) q          { return new(big.Rat).Sub(a, b) }
func mul(a, b q) q          { return new(big.Rat).Mul(a, b) }
func div(a, b q) q          { return new(big.Rat).Quo(a, b) }
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

type budget struct {
	ctx             context.Context
	o               Options
	work, intervals int
	err             error
}

func (b *budget) tick() bool {
	if b.err != nil {
		return false
	}
	if e := b.ctx.Err(); e != nil {
		b.err = e
		return false
	}
	if b.work >= b.o.MaxWork {
		b.err = ErrLimit
		return false
	}
	b.work++
	return true
}
func (b *budget) check(x q) {
	if x.Num().BitLen() > b.o.MaxBits || x.Denom().BitLen() > b.o.MaxBits {
		b.err = ErrLimit
	}
}
func setup(ctx context.Context, tolerance float64, o Options) (*budget, error) {
	if ctx == nil || !finite(tolerance) || tolerance < 0 || o.MaxDegree < 0 || o.MaxDegree > 128 || o.MaxWork < 0 || o.MaxIntervals < 0 || o.MaxDepth < 0 || o.MaxDepth > 256 || o.MaxBits < 0 {
		return nil, ErrInvalid
	}
	p := []*int{&o.MaxDegree, &o.MaxWork, &o.MaxIntervals, &o.MaxDepth, &o.MaxBits}
	d := []int{32, 2000000, 100000, 48, 16384}
	for i, x := range p {
		if *x == 0 {
			*x = d[i]
		}
	}
	b := &budget{ctx: ctx, o: o}
	b.tick()
	return b, nil
}
func controls(c Curve, b *budget) ([]hom, error) {
	if len(c.Poles) == 0 || c.Weights != nil && len(c.Weights) != len(c.Poles) {
		return nil, ErrInvalid
	}
	if len(c.Poles)-1 > b.o.MaxDegree {
		b.err = ErrLimit
		return nil, nil
	}
	out := make([]hom, len(c.Poles))
	for j, p := range c.Poles {
		if !b.tick() {
			return nil, nil
		}
		if !p.IsFinite() {
			return nil, ErrInvalid
		}
		w := 1.
		if c.Weights != nil {
			w = c.Weights[j]
		}
		if !finite(w) || w <= 0 {
			return nil, ErrInvalid
		}
		i := j
		if c.Reversed {
			i = len(out) - 1 - j
		}
		weight := num(w)
		out[i] = hom{mul(num(p.X), weight), mul(num(p.Y), weight), mul(num(p.Z), weight), weight}
		for _, v := range out[i] {
			b.check(v)
		}
	}
	return out, nil
}
func component(c []hom, k int) []q {
	r := make([]q, len(c))
	for i, p := range c {
		r[i] = p[k]
	}
	return r
}
func binomial(n int) []q {
	r := make([]q, n+1)
	for i := range r {
		r[i] = new(big.Rat).SetInt(new(big.Int).Binomial(int64(n), int64(i)))
	}
	return r
}
func product(a, c []q, b *budget) []q {
	m, n := len(a)-1, len(c)-1
	aa, cc, dd := binomial(m), binomial(n), binomial(m+n)
	r := make([]q, m+n+1)
	for k := range r {
		r[k] = new(big.Rat)
	}
	for i, x := range a {
		for j, y := range c {
			if !b.tick() {
				return r
			}
			factor := div(mul(aa[i], cc[j]), dd[i+j])
			r[i+j] = add(r[i+j], mul(factor, mul(x, y)))
			b.check(r[i+j])
		}
	}
	return r
}
func split(coeff []q, b *budget) ([]q, []q) {
	n := len(coeff)
	work := append([]q(nil), coeff...)
	left, right := make([]q, n), make([]q, n)
	left[0], right[n-1] = work[0], work[n-1]
	for level := 1; level < n; level++ {
		for i := 0; i < n-level; i++ {
			if !b.tick() {
				return left, right
			}
			work[i] = div(add(work[i], work[i+1]), big.NewRat(2, 1))
			b.check(work[i])
		}
		left[level], right[n-level-1] = work[0], work[n-level-1]
	}
	return left, right
}
func evaluate(c []hom, t q, b *budget) ([3]q, bool) {
	p := append([]hom(nil), c...)
	one := sub(big.NewRat(1, 1), t)
	for n := len(p) - 1; n > 0; n-- {
		for i := 0; i < n; i++ {
			if !b.tick() {
				return [3]q{}, false
			}
			for k := 0; k < 4; k++ {
				p[i][k] = add(mul(one, p[i][k]), mul(t, p[i+1][k]))
				b.check(p[i][k])
			}
		}
	}
	if b.err != nil {
		return [3]q{}, false
	}
	var result [3]q
	for k := range result {
		result[k] = div(p[0][k], p[0][3])
		b.check(result[k])
	}
	return result, b.err == nil
}
func witness(a, c []hom, t q, b *budget) (*Witness, bool) {
	x, ok := evaluate(a, t, b)
	if !ok {
		return nil, false
	}
	y, ok := evaluate(c, t, b)
	if !ok {
		return nil, false
	}
	w := &Witness{Parameter: new(big.Rat).Set(t), SquaredDistance: new(big.Rat)}
	var xx, yy [3]float64
	for k := 0; k < 3; k++ {
		w.ExactPointA[k] = new(big.Rat).Set(x[k])
		w.ExactPointB[k] = new(big.Rat).Set(y[k])
		d := sub(x[k], y[k])
		w.SquaredDistance.Add(w.SquaredDistance, mul(d, d))
		xx[k], _ = x[k].Float64()
		yy[k], _ = y[k].Float64()
	}
	b.check(w.SquaredDistance)
	w.PointA = geom.Vec3{X: xx[0], Y: xx[1], Z: xx[2]}
	w.PointB = geom.Vec3{X: yy[0], Y: yy[1], Z: yy[2]}
	return w, b.err == nil
}
func finish(r Report, b *budget) (Report, error) {
	r.Work, r.Intervals = b.work, b.intervals
	if b.err != nil {
		r.Status = Unresolved
		r.Witness = nil
		r.Reason = b.err.Error()
		if !errors.Is(b.err, ErrLimit) {
			return r, b.err
		}
	}
	return r, nil
}

// Check compares two spatial rational Bezier curves continuously at identical
// normalized parameters. Limits return Unresolved with nil error; invalid data
// and cancellation return errors. Zero tolerance can prove exact identity.
func Check(ctx context.Context, a, c Curve, tolerance float64, o Options) (Report, error) {
	r := Report{Tolerance: tolerance}
	b, e := setup(ctx, tolerance, o)
	if e != nil {
		return r, e
	}
	aa, e := controls(a, b)
	if e != nil {
		return r, e
	}
	cc, e := controls(c, b)
	if e != nil {
		return r, e
	}
	if b.err != nil {
		return finish(r, b)
	}
	return prove(aa, cc, tolerance, b)
}

// CheckPlanar maps a UV curve through an affine chart exactly and compares it to
// a spatial model curve. Chart axes must be exactly independent. No intermediate
// model-space pole is rounded to float64 before the proof.
func CheckPlanar(ctx context.Context, uv PlanarCurve, chart Chart, model Curve, tolerance float64, o Options) (Report, error) {
	r := Report{Tolerance: tolerance}
	b, e := setup(ctx, tolerance, o)
	if e != nil {
		return r, e
	}
	aa, e := chartControls(uv, chart, b)
	if e != nil {
		return r, e
	}
	cc, e := controls(model, b)
	if e != nil {
		return r, e
	}
	if b.err != nil {
		return finish(r, b)
	}
	return prove(aa, cc, tolerance, b)
}
func chartControls(uv PlanarCurve, chart Chart, b *budget) ([]hom, error) {
	if !chart.Origin.IsFinite() || !chart.U.IsFinite() || !chart.V.IsFinite() {
		return nil, ErrInvalid
	}
	origin := [3]q{num(chart.Origin.X), num(chart.Origin.Y), num(chart.Origin.Z)}
	u := [3]q{num(chart.U.X), num(chart.U.Y), num(chart.U.Z)}
	v := [3]q{num(chart.V.X), num(chart.V.Y), num(chart.V.Z)}
	for _, arr := range [][3]q{origin, u, v} {
		for _, x := range arr {
			b.check(x)
		}
	}
	independent := false
	for k := 0; k < 3; k++ {
		i, j := (k+1)%3, (k+2)%3
		independent = independent || sub(mul(u[i], v[j]), mul(u[j], v[i])).Sign() != 0
	}
	if !independent {
		return nil, ErrInvalid
	}
	if len(uv.Poles) > b.o.MaxDegree+1 {
		b.err = ErrLimit
		return nil, nil
	}
	poles := make([]geom.Vec3, len(uv.Poles))
	for i, p := range uv.Poles {
		poles[i] = geom.Vec3{X: p.X, Y: p.Y}
	}
	aa, e := controls(Curve{Poles: poles, Weights: uv.Weights, Reversed: uv.Reversed}, b)
	if e != nil {
		return nil, e
	}
	for i, p := range aa {
		for k := 0; k < 3; k++ {
			aa[i][k] = add(add(mul(origin[k], p[3]), mul(u[k], p[0])), mul(v[k], p[1]))
			b.check(aa[i][k])
		}
	}
	return aa, nil
}

// CheckPlanarPair compares two rational UV curves mapped through independent
// affine planar charts, with no intermediate coordinate rounding.
func CheckPlanarPair(ctx context.Context, a PlanarCurve, chartA Chart, c PlanarCurve, chartB Chart, tolerance float64, o Options) (Report, error) {
	r := Report{Tolerance: tolerance}
	b, e := setup(ctx, tolerance, o)
	if e != nil {
		return r, e
	}
	aa, e := chartControls(a, chartA, b)
	if e != nil {
		return r, e
	}
	cc, e := chartControls(c, chartB, b)
	if e != nil {
		return r, e
	}
	if b.err != nil {
		return finish(r, b)
	}
	return prove(aa, cc, tolerance, b)
}

// CheckPlanarLine compares an affine-chart UV curve with an analytic line over
// [from,to]. Reversed intervals are allowed. The normalized map is exactly
// Origin+((1-t)*from+t*to)*Direction; endpoint construction is rational.
func CheckPlanarLine(ctx context.Context, uv PlanarCurve, chart Chart, line geom.Line, from, to, tolerance float64, o Options) (Report, error) {
	r := Report{Tolerance: tolerance}
	b, e := setup(ctx, tolerance, o)
	if e != nil {
		return r, e
	}
	origin, direction := line.Origin(), line.Direction()
	if !origin.IsFinite() || !direction.IsFinite() || direction == (geom.Vec3{}) || !finite(from) || !finite(to) {
		return r, ErrInvalid
	}
	aa, e := chartControls(uv, chart, b)
	if e != nil {
		return r, e
	}
	base := [3]float64{origin.X, origin.Y, origin.Z}
	axis := [3]float64{direction.X, direction.Y, direction.Z}
	cc := make([]hom, 2)
	for i, t := range []float64{from, to} {
		cc[i][3] = big.NewRat(1, 1)
		for k := 0; k < 3; k++ {
			cc[i][k] = add(num(base[k]), mul(num(t), num(axis[k])))
			b.check(cc[i][k])
		}
	}
	if b.err != nil {
		return finish(r, b)
	}
	return prove(aa, cc, tolerance, b)
}

func prove(a, c []hom, tolerance float64, b *budget) (Report, error) {
	r := Report{Tolerance: tolerance}
	wa, wc := component(a, 3), component(c, 3)
	den := product(wa, wc, b)
	if b.err != nil {
		return finish(r, b)
	}
	var numerator [3][]q
	upper := new(big.Rat)
	minDen := new(big.Rat).Set(den[0])
	for _, x := range den {
		if x.Cmp(minDen) < 0 {
			minDen = x
		}
	}
	for k := 0; k < 3; k++ {
		x, y := product(component(a, k), wc, b), product(component(c, k), wa, b)
		if b.err != nil {
			return finish(r, b)
		}
		numerator[k] = make([]q, len(x))
		max := new(big.Rat)
		for i := range x {
			d := sub(x[i], y[i])
			b.check(d)
			numerator[k][i] = d
			v := new(big.Rat).Abs(d)
			if v.Cmp(max) > 0 {
				max = v
			}
		}
		upper.Add(upper, mul(max, max))
	}
	r.SquaredUpperBound = div(upper, mul(minDen, minDen))
	b.check(r.SquaredUpperBound)
	tol2 := mul(num(tolerance), num(tolerance))
	b.check(tol2)
	if b.err != nil {
		return finish(r, b)
	}
	if r.SquaredUpperBound.Cmp(tol2) <= 0 {
		r.Status, r.Reason = Certified, "global homogeneous hull proves agreement"
		return finish(r, b)
	}
	polynomial := product(den, den, b)
	if b.err != nil {
		return finish(r, b)
	}
	for i := range polynomial {
		polynomial[i] = mul(polynomial[i], tol2)
		b.check(polynomial[i])
	}
	for k := 0; k < 3; k++ {
		squared := product(numerator[k], numerator[k], b)
		if b.err != nil {
			return finish(r, b)
		}
		for i := range polynomial {
			polynomial[i] = sub(polynomial[i], squared[i])
			b.check(polynomial[i])
		}
	}
	if b.err != nil {
		return finish(r, b)
	}
	type interval struct {
		p      []q
		lo, hi q
		depth  int
	}
	stack := []interval{{polynomial, new(big.Rat), big.NewRat(1, 1), 0}}
	for len(stack) > 0 {
		if !b.tick() {
			return finish(r, b)
		}
		if b.intervals >= b.o.MaxIntervals {
			b.err = ErrLimit
			return finish(r, b)
		}
		b.intervals++
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		var negative q
		if x.p[0].Sign() < 0 {
			negative = x.lo
		} else if x.p[len(x.p)-1].Sign() < 0 {
			negative = x.hi
		}
		if negative != nil {
			w, ok := witness(a, c, negative, b)
			if !ok {
				return finish(r, b)
			}
			r.Status, r.Witness, r.Reason = Exceeded, w, "exact witness exceeds tolerance"
			return finish(r, b)
		}
		nonnegative := true
		for _, coefficient := range x.p {
			if coefficient.Sign() < 0 {
				nonnegative = false
				break
			}
		}
		if nonnegative {
			continue
		}
		if x.depth >= b.o.MaxDepth {
			b.err = ErrLimit
			return finish(r, b)
		}
		left, right := split(x.p, b)
		if b.err != nil {
			return finish(r, b)
		}
		middle := div(add(x.lo, x.hi), big.NewRat(2, 1))
		stack = append(stack, interval{right, middle, x.hi, x.depth + 1}, interval{left, x.lo, middle, x.depth + 1})
	}
	r.Status, r.Reason = Certified, "Bernstein subdivision proves continuous agreement"
	if r.SquaredUpperBound.Cmp(tol2) > 0 {
		r.SquaredUpperBound = new(big.Rat).Set(tol2)
	}
	return finish(r, b)
}
