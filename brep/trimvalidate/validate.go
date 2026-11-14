// Package trimvalidate certifies planar UV-loop topology for straight and
// bounded positive-weight rational Bézier trims. It does not certify model-space surface embedding or
// continuous agreement between model-space edges and p-curves.
package trimvalidate

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
)

// SegmentKind identifies a supported exact UV curve representation.
type SegmentKind uint8

const (
	Line SegmentKind = iota
	Quadratic
	UnsupportedCurve
	RationalQuadratic
	exactRationalQuadratic
	exactRationalBezier
)

// Segment is directed from Start to End; Control is used for either quadratic kind.
// For a line, the zero-value Kind is sufficient.
type Segment struct {
	Start, Control, End geom.Vec2
	Kind                SegmentKind
	Weights             [3]float64 // Required strictly positive for RationalQuadratic.
	exact               *curve
}
type Loop []Segment

// ExactQuadratic constructs a proof segment from homogeneous (w*u,w*v,w)
// Bernstein coefficients, without requiring Euclidean controls or weights to
// fit float64. The coefficients are copied. Degree-one rational segments may
// be exactly degree elevated before calling this constructor.
func ExactQuadratic(h [3][3]*big.Rat) (Segment, error) {
	c := &curve{}
	for i, v := range h {
		if v[0] == nil || v[1] == nil || v[2] == nil || v[2].Sign() <= 0 {
			return Segment{}, ErrOptions
		}
		c.p[i] = point{div(v[0], v[2]), div(v[1], v[2])}
		c.w[i] = new(big.Rat).Set(v[2])
	}
	return Segment{Kind: exactRationalQuadratic, exact: c}, nil
}

// ExactBezier constructs a proof segment from positive homogeneous
// Bernstein controls (w*u,w*v,w). Higher degree segments need a proved
// strictly monotone projection. The constructor copies all coefficients.
func ExactBezier(h [][3]*big.Rat) (Segment, error) {
	if len(h) == 3 {
		return ExactQuadratic([3][3]*big.Rat{h[0], h[1], h[2]})
	}
	if len(h) < 4 || len(h) > 9 {
		return Segment{}, ErrOptions
	}
	c := &curve{controls: make([]point, len(h)), weights: make([]q, len(h))}
	for i, v := range h {
		if v[0] == nil || v[1] == nil || v[2] == nil || v[2].Sign() <= 0 {
			return Segment{}, ErrOptions
		}
		c.controls[i] = point{div(v[0], v[2]), div(v[1], v[2])}
		c.weights[i] = new(big.Rat).Set(v[2])
	}
	c.p[0], c.p[2] = c.controls[0], c.controls[len(c.controls)-1]
	return Segment{Kind: exactRationalBezier, exact: c}, nil
}

// Status is deliberately unresolved at its zero value. Callers must check it
// even when ValidateLoops returns a nil error.
type Status uint8

const (
	Unresolved Status = iota
	Certified
	Invalid
)

var ErrLimit = errors.New("trimvalidate: proof budget exhausted")
var ErrOptions = errors.New("trimvalidate: invalid options")

// Options bounds edges (default 256), proof work (200000), subdivisions (100000),
// binary subdivision depth (48, maximum 256), and stored control-coordinate rational bits (16384).
// Zero selects defaults. Limits produce Unresolved, not an invalidity claim.
type Options struct{ MaxEdges, MaxWork, MaxSubdivisions, MaxDepth, MaxBits int }

// Report identifies offending loop/edge indices (outer loop zero, holes 1..n).
// Indices are -1 where no individual offending edge applies. Certified means
// every required claim was proved; unsupported curve types remain Unresolved.
type Report struct {
	Status                     Status
	Reason                     string
	LoopA, EdgeA, LoopB, EdgeB int
	Work, Subdivisions         int
}

// Location is the position of a represented UV point relative to the
// certified outer and inner loops.
type Location uint8

const (
	Unknown Location = iota
	Outside
	Inside
	Boundary
)

type q = *big.Rat
type point [2]q
type curve struct {
	p                 [3]point
	w                 [3]q // nil means equal unit weights.
	line              bool
	loop, edge, depth int
	controls          []point
	weights           []q
}

func (c curve) points() []point {
	if c.controls != nil {
		return c.controls
	}
	return c.p[:]
}

func num(v float64) q           { return new(big.Rat).SetFloat64(v) }
func exact(p geom.Vec2) point   { return point{num(p.X), num(p.Y)} }
func add(a, b q) q              { return new(big.Rat).Add(a, b) }
func sub(a, b q) q              { return new(big.Rat).Sub(a, b) }
func mul(a, b q) q              { return new(big.Rat).Mul(a, b) }
func div(a, b q) q              { return new(big.Rat).Quo(a, b) }
func plus(a, b point) point     { return point{add(a[0], b[0]), add(a[1], b[1])} }
func minus(a, b point) point    { return point{sub(a[0], b[0]), sub(a[1], b[1])} }
func scale(a point, t q) point  { return point{mul(a[0], t), mul(a[1], t)} }
func midpoint(a, b point) point { return scale(plus(a, b), big.NewRat(1, 2)) }
func cross(a, b point) q        { return sub(mul(a[0], b[1]), mul(a[1], b[0])) }
func dot(a, b point) q          { return add(mul(a[0], b[0]), mul(a[1], b[1])) }
func same(a, b point) bool      { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 }
func zero(a point) bool         { return a[0].Sign() == 0 && a[1].Sign() == 0 }
func finite(p geom.Vec2) bool   { return geom.Vec3{X: p.X, Y: p.Y}.IsFinite() }
func onSegment(p, a, b point) bool {
	d := minus(b, a)
	if cross(d, minus(p, a)).Sign() != 0 {
		return false
	}
	x := dot(minus(p, a), d)
	return x.Sign() >= 0 && x.Cmp(dot(d, d)) <= 0
}
func weight(c curve, i int) q {
	if c.w[0] == nil {
		return big.NewRat(1, 1)
	}
	return c.w[i]
}
func evaluate(c curve, t q) point {
	one := sub(big.NewRat(1, 1), t)
	bs := [3]q{mul(one, one), mul(big.NewRat(2, 1), mul(t, one)), mul(t, t)}
	sum := point{new(big.Rat), new(big.Rat)}
	den := new(big.Rat)
	for i, p := range c.p {
		w := mul(weight(c, i), bs[i])
		sum = plus(sum, scale(p, w))
		den = add(den, w)
	}
	return scale(sum, div(big.NewRat(1, 1), den))
}
func onCurve(p point, c curve) bool {
	if c.controls != nil {
		// Higher-degree exact point membership is handled conservatively
		// by control-hull subdivision. Endpoints need no root search.
		return same(p, c.p[0]) || same(p, c.p[2])
	}
	if c.line {
		return onSegment(p, c.p[0], c.p[2])
	}
	if same(p, c.p[2]) {
		return true
	}
	a, b, d := minus(c.p[1], c.p[0]), minus(c.p[2], c.p[0]), minus(p, c.p[0])
	det := cross(a, b)
	if det.Sign() == 0 {
		return false
	}
	l1, l2 := div(cross(d, b), det), div(cross(a, d), det)
	l0 := sub(sub(big.NewRat(1, 1), l1), l2)
	if l0.Sign() < 0 || l1.Sign() < 0 || l2.Sign() < 0 {
		return false
	}
	numer := mul(l1, weight(c, 0))
	den := add(mul(big.NewRat(2, 1), mul(weight(c, 1), l0)), numer)
	if den.Sign() == 0 {
		return false
	}
	t := div(numer, den)
	return same(evaluate(c, t), p)
}

type budget struct {
	ctx                context.Context
	o                  Options
	work, subdivisions int
	err                error
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
func (b *budget) check(p point) {
	for _, v := range p {
		if v.Num().BitLen() > b.o.MaxBits || v.Denom().BitLen() > b.o.MaxBits {
			b.err = ErrLimit
		}
	}
}
func (b *budget) split(c curve) (curve, curve) {
	if c.depth >= b.o.MaxDepth || b.subdivisions >= b.o.MaxSubdivisions {
		b.err = ErrLimit
		return curve{}, curve{}
	}
	b.subdivisions++
	if c.controls != nil {
		return b.splitBezier(c)
	}
	left, right := c, c
	left.depth++
	right.depth++
	var a, d, m point
	if c.w[0] == nil {
		a, d = midpoint(c.p[0], c.p[1]), midpoint(c.p[1], c.p[2])
		m = midpoint(a, d)
	} else {
		w01, w12 := div(add(c.w[0], c.w[1]), big.NewRat(2, 1)), div(add(c.w[1], c.w[2]), big.NewRat(2, 1))
		wm := div(add(w01, w12), big.NewRat(2, 1))
		h01, h12 := midpoint(scale(c.p[0], c.w[0]), scale(c.p[1], c.w[1])), midpoint(scale(c.p[1], c.w[1]), scale(c.p[2], c.w[2]))
		hm := midpoint(h01, h12)
		a, d, m = scale(h01, div(big.NewRat(1, 1), w01)), scale(h12, div(big.NewRat(1, 1), w12)), scale(hm, div(big.NewRat(1, 1), wm))
		left.w = [3]q{c.w[0], w01, wm}
		right.w = [3]q{wm, w12, c.w[2]}
		for _, w := range []q{w01, w12, wm} {
			b.check(point{w, w})
		}
	}
	b.check(a)
	b.check(d)
	b.check(m)
	left.p = [3]point{c.p[0], a, m}
	right.p = [3]point{m, d, c.p[2]}

	return left, right
}
func axes(c curve) []point {
	var out []point
	p := c.points()
	// Every convex hull side joins two control points, which can be
	// nonconsecutive in a higher-degree net. Extra axes are safe.
	for i, a := range p {
		for _, z := range p[i+1:] {
			d := minus(z, a)
			if !zero(d) {
				out = append(out, d, point{new(big.Rat).Neg(d[1]), d[0]})
			}
		}
	}
	return out
}
func interval(c curve, axis point) (q, q) {
	lo, hi := dot(c.p[0], axis), dot(c.p[0], axis)
	for _, p := range c.points()[1:] {
		v := dot(p, axis)
		if v.Cmp(lo) < 0 {
			lo = v
		}
		if v.Cmp(hi) > 0 {
			hi = v
		}
	}
	return lo, hi
}

// hullSeparated proves disjoint convex control hulls, or intersection confined
// to the one prescribed common endpoint. Closed half-plane intersections on a
// separator are themselves reduced to exact one-dimensional intervals.
func hullSeparated(a, b curve, allowed *point) bool {
	if coordinateHullSeparated(a, b) {
		return true
	}
	return hullSeparatedAxes(a, b, allowed)
}

func hullSeparatedAxes(a, b curve, allowed *point) bool {
	for _, axis := range append(axes(a), axes(b)...) {
		alo, ahi := interval(a, axis)
		blo, bhi := interval(b, axis)
		if ahi.Cmp(blo) < 0 || bhi.Cmp(alo) < 0 {
			return true
		}
		if allowed == nil {
			continue
		}
		origin := dot(*allowed, axis)
		if !(alo.Cmp(origin) >= 0 && bhi.Cmp(origin) <= 0 || blo.Cmp(origin) >= 0 && ahi.Cmp(origin) <= 0) {
			continue
		}
		tangent := point{new(big.Rat).Neg(axis[1]), axis[0]}
		ranges := func(c curve) (q, q) {
			var lo, hi q
			for _, p := range c.points() {
				if dot(p, axis).Cmp(origin) != 0 {
					continue
				}
				v := dot(minus(p, *allowed), tangent)
				if lo == nil || v.Cmp(lo) < 0 {
					lo = v
				}
				if hi == nil || v.Cmp(hi) > 0 {
					hi = v
				}
			}
			return lo, hi
		}
		al, ah := ranges(a)
		bl, bh := ranges(b)
		if al == nil || bl == nil {
			return true
		}
		if al.Cmp(bl) < 0 {
			al = bl
		}
		if ah.Cmp(bh) > 0 {
			ah = bh
		}
		if al.Cmp(ah) > 0 || al.Sign() == 0 && ah.Sign() == 0 {
			return true
		}
	}
	return false
}
func outsideHull(p point, c curve) bool {
	for _, a := range axes(c) {
		lo, hi := interval(c, a)
		v := dot(p, a)
		if v.Cmp(lo) < 0 || v.Cmp(hi) > 0 {
			return true
		}
	}
	return false
}

// graphCrossing proves an intersection by the intermediate value theorem: A
// is a graph over its chord, B remains within its open horizontal extent, and
// B's endpoints lie on opposite sides of A's entire vertical control hull.
func graphCrossing(a, b curve) bool {
	d := minus(a.p[2], a.p[0])
	if !monotoneProjection(a, d, true) {
		return false
	}
	length := dot(d, d)
	var lo, hi q
	for _, p := range a.points() {
		v := cross(d, minus(p, a.p[0]))
		if lo == nil || v.Cmp(lo) < 0 {
			lo = v
		}
		if hi == nil || v.Cmp(hi) > 0 {
			hi = v
		}
	}
	for _, p := range b.points() {
		x := dot(minus(p, a.p[0]), d)
		if x.Sign() <= 0 || x.Cmp(length) >= 0 {
			return false
		}
	}
	start, end := cross(d, minus(b.p[0], a.p[0])), cross(d, minus(b.p[2], a.p[0]))
	return start.Cmp(lo) < 0 && end.Cmp(hi) > 0 || end.Cmp(lo) < 0 && start.Cmp(hi) > 0
}
func lineCrossing(a, b curve, allowed [2]*point) bool {
	u, v, w := minus(a.p[2], a.p[0]), minus(b.p[2], b.p[0]), minus(b.p[0], a.p[0])
	den := cross(u, v)
	if den.Sign() == 0 {
		return false
	}
	s, t := div(cross(w, v), den), div(cross(w, u), den)
	one := big.NewRat(1, 1)
	if s.Sign() < 0 || s.Cmp(one) > 0 || t.Sign() < 0 || t.Cmp(one) > 0 {
		return false
	}
	p := plus(a.p[0], scale(u, s))
	return !permitted(p, allowed)
}

// Rational line/quadratic roots include every linear root and every double
// (tangential) root, even when its parameter is not dyadic. Irrational transverse
// roots are left to graph crossing/subdivision rather than rounded here.
func rationalLineContact(c, line curve, allowed [2]*point) bool {
	if c.controls != nil {
		return false
	}
	d := minus(line.p[2], line.p[0])
	var h [3]q
	for i, p := range c.p {
		h[i] = mul(weight(c, i), cross(d, minus(p, line.p[0])))
	}
	aa := add(sub(h[0], mul(big.NewRat(2, 1), h[1])), h[2])
	bb := mul(big.NewRat(2, 1), sub(h[1], h[0]))
	cc := h[0]
	var roots []q
	if aa.Sign() == 0 {
		if bb.Sign() != 0 {
			roots = append(roots, div(new(big.Rat).Neg(cc), bb))
		}
	} else {
		disc := sub(mul(bb, bb), mul(big.NewRat(4, 1), mul(aa, cc)))
		if disc.Sign() >= 0 {
			rn := new(big.Int).Sqrt(disc.Num())
			rd := new(big.Int).Sqrt(disc.Denom())
			if new(big.Int).Mul(new(big.Int).Set(rn), rn).Cmp(disc.Num()) == 0 && new(big.Int).Mul(new(big.Int).Set(rd), rd).Cmp(disc.Denom()) == 0 {
				root := new(big.Rat).SetFrac(rn, rd)
				den := mul(big.NewRat(2, 1), aa)
				roots = append(roots, div(sub(new(big.Rat).Neg(bb), root), den), div(add(new(big.Rat).Neg(bb), root), den))
			}
		}
	}
	for _, t := range roots {
		if t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
			continue
		}
		p := evaluate(c, t)
		if onSegment(p, line.p[0], line.p[2]) && !permitted(p, allowed) {
			return true
		}
	}
	return false
}

func extent(c curve) q {
	lo, hi := interval(c, point{big.NewRat(1, 1), new(big.Rat)})
	x := sub(hi, lo)
	lo, hi = interval(c, point{new(big.Rat), big.NewRat(1, 1)})
	y := sub(hi, lo)
	return add(mul(x, x), mul(y, y))
}
func permitted(p point, allowed [2]*point) bool {
	return allowed[0] != nil && same(p, *allowed[0]) || allowed[1] != nil && same(p, *allowed[1])
}

func separate(a, b curve, allowed [2]*point, budget *budget) Status {
	type pair struct{ a, b curve }
	stack := []pair{{a, b}}
	for len(stack) > 0 {
		if !budget.tick() {
			return Unresolved
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if hullSeparated(p.a, p.b, allowed[0]) || allowed[1] != nil && hullSeparated(p.a, p.b, allowed[1]) {
			continue
		}
		for _, x := range []point{p.a.p[0], p.a.p[2]} {
			if !permitted(x, allowed) && onCurve(x, p.b) {
				return Invalid
			}
		}
		for _, x := range []point{p.b.p[0], p.b.p[2]} {
			if !permitted(x, allowed) && onCurve(x, p.a) {
				return Invalid
			}
		}
		if p.a.line && rationalLineContact(p.b, p.a, allowed) || p.b.line && rationalLineContact(p.a, p.b, allowed) || p.a.line && p.b.line && lineCrossing(p.a, p.b, allowed) || graphCrossing(p.a, p.b) || graphCrossing(p.b, p.a) {
			return Invalid
		}
		if extent(p.a).Cmp(extent(p.b)) >= 0 {
			x, y := budget.split(p.a)
			if budget.err != nil {
				return Unresolved
			}
			stack = append(stack, pair{y, p.b}, pair{x, p.b})
		} else {
			x, y := budget.split(p.b)
			if budget.err != nil {
				return Unresolved
			}
			stack = append(stack, pair{p.a, y}, pair{p.a, x})
		}
	}
	return Certified
}
func area(c curve) q {
	a := c.p[0]
	b := scale(minus(c.p[1], a), big.NewRat(2, 1))
	d := plus(minus(a, scale(c.p[1], big.NewRat(2, 1))), c.p[2])
	return add(add(cross(a, b), cross(a, d)), div(cross(b, d), big.NewRat(3, 1)))
}

// Positive-weight rational arcs remain inside their control triangles and on
// one side of the endpoint chord. Thus their oriented area lies between the
// chord's area and the control polygon's area; homogeneous subdivision sharpens
// that exact enclosure without integrating rational functions approximately.
func areaBounds(c curve) (q, q) {
	if c.controls != nil {
		return bezierChordAreaBounds(c)
	}
	if c.w[0] == nil {
		x := area(c)
		return x, x
	}
	chord := cross(c.p[0], c.p[2])
	triangle := cross(minus(c.p[1], c.p[0]), minus(c.p[2], c.p[0]))
	if triangle.Sign() < 0 {
		return add(chord, triangle), chord
	}
	return chord, add(chord, triangle)
}
func orientation(loop []curve, b *budget) (int, bool) {
	pieces := append([]curve(nil), loop...)
	for {
		lo, hi := new(big.Rat), new(big.Rat)
		largest, index := new(big.Rat), -1
		for i, c := range pieces {
			if !b.tick() {
				return 0, false
			}
			a, z := areaBounds(c)
			lo = add(lo, a)
			hi = add(hi, z)
			width := sub(z, a)
			if width.Cmp(largest) > 0 {
				largest, index = width, i
			}
		}
		if lo.Sign() > 0 {
			return 1, true
		}
		if hi.Sign() < 0 {
			return -1, true
		}
		if lo.Sign() == 0 && hi.Sign() == 0 {
			return 0, true
		}
		if index < 0 {
			b.err = ErrLimit
			return 0, false
		}
		a, c := b.split(pieces[index])
		if b.err != nil {
			return 0, false
		}
		pieces[index] = a
		pieces = append(pieces, c)
	}
}

func winding(p point, loop []curve, b *budget) (int, bool) {
	sum := 0
	stack := append([]curve(nil), loop...)
	for len(stack) > 0 {
		if !b.tick() {
			return 0, false
		}
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !outsideHull(p, c) {
			x, y := b.split(c)
			if b.err != nil {
				return 0, false
			}
			stack = append(stack, y, x)
			continue
		}
		a, z := c.p[0], c.p[2]
		side := cross(minus(z, a), minus(p, a)).Sign()
		if a[1].Cmp(p[1]) <= 0 && z[1].Cmp(p[1]) > 0 && side > 0 {
			sum++
		}
		if a[1].Cmp(p[1]) > 0 && z[1].Cmp(p[1]) <= 0 && side < 0 {
			sum--
		}
	}
	return sum, true
}

// WithDefaults validates proof limits and fills their default values.
func (o Options) WithDefaults() (Options, error) {
	if o.MaxEdges < 0 || o.MaxWork < 0 || o.MaxSubdivisions < 0 || o.MaxDepth < 0 || o.MaxDepth > 256 || o.MaxBits < 0 {
		return Options{}, ErrOptions
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 256
	}
	if o.MaxWork == 0 {
		o.MaxWork = 200000
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 100000
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 48
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	return o, nil
}

// ValidateLoops proves simple, correctly oriented UV loops with strictly
// contained, disjoint, nonnested holes. Invalid geometry returns Invalid with nil
// error; unsupported kinds or exhausted proof budgets return Unresolved with
// nil error. Bad options and cancellation return an error and never Certified.
func ValidateLoops(ctx context.Context, outer Loop, holes []Loop, o Options) (Report, error) {
	r, _, e := validateLoops(ctx, outer, holes, nil, o, nil, nil)
	return r, e
}

// LocateInLoops first proves the trim topology, then computes exact winding
// by certified hull exclusion. Straight and rational quadratic boundary hits
// are recognized exactly. A higher-degree hit that cannot be proved by the
// budget remains Unknown. A failed topology or location proof never guesses.
func LocateInLoops(ctx context.Context, outer Loop, holes []Loop, uv geom.Vec2, o Options) (Location, Report, error) {
	if !finite(uv) {
		return Unknown, Report{LoopA: -1, EdgeA: -1, LoopB: -1, EdgeB: -1}, ErrOptions
	}
	return LocateRationalInLoops(ctx, outer, holes, [2]*big.Rat{num(uv.X), num(uv.Y)}, o)
}

// LocateRationalInLoops classifies an exact rational UV without rounding it
// to a represented point. The caller's rationals are read-only.
func LocateRationalInLoops(ctx context.Context, outer Loop, holes []Loop, uv [2]*big.Rat, o Options) (Location, Report, error) {
	if uv[0] == nil || uv[1] == nil {
		return Unknown, Report{LoopA: -1, EdgeA: -1, LoopB: -1, EdgeB: -1}, ErrOptions
	}
	p := point{uv[0], uv[1]}
	r, location, e := validateLoops(ctx, outer, holes, &p, o, nil, nil)
	return location, r, e
}

// LocateRationalBoxInLoops proves one material classification throughout a
// closed rational box. The box may be degenerate along either axis. If any
// boundary touches it, the result is Unknown; overlapping hulls alone never
// establish that a trim boundary intersects the box.
func LocateRationalBoxInLoops(ctx context.Context, outer Loop, holes []Loop, lo, hi [2]*big.Rat, o Options) (Location, Report, error) {
	for i := range lo {
		if lo[i] == nil || hi[i] == nil || lo[i].Cmp(hi[i]) > 0 {
			return Unknown, Report{LoopA: -1, EdgeA: -1, LoopB: -1, EdgeB: -1}, ErrOptions
		}
	}
	box := rationalQueryBox{lo: lo, hi: hi}
	r, location, e := validateLoops(ctx, outer, holes, nil, o, nil, nil, box)
	return location, r, e
}

type rationalQueryBox struct{ lo, hi point }

func (q rationalQueryBox) excludes(c curve, b *budget) bool {
	stack := []curve{c}
	for len(stack) > 0 {
		if !b.tick() {
			return false
		}
		c = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if c.line {
			if q.intersectsLine(c.p[0], c.p[2], b) {
				return false
			}
			continue
		}
		points := c.points()
		disjoint := false
		for axis := range q.lo {
			lower, upper := true, true
			for _, p := range points {
				lower = lower && p[axis].Cmp(q.lo[axis]) < 0
				upper = upper && p[axis].Cmp(q.hi[axis]) > 0
			}
			disjoint = disjoint || lower || upper
		}
		if disjoint {
			continue
		}
		if q.contains(c.p[0]) || q.contains(c.p[2]) {
			return false
		}
		a, z := b.split(c)
		if b.err != nil {
			return false
		}
		stack = append(stack, z, a)
	}
	return true
}

func (q rationalQueryBox) contains(p point) bool {
	return p[0].Cmp(q.lo[0]) >= 0 && p[0].Cmp(q.hi[0]) <= 0 &&
		p[1].Cmp(q.lo[1]) >= 0 && p[1].Cmp(q.hi[1]) <= 0
}

func (q rationalQueryBox) intersectsLine(a, z point, b *budget) bool {
	lo, hi := new(big.Rat), big.NewRat(1, 1)
	for i := range a {
		d := sub(z[i], a[i])
		if d.Sign() == 0 {
			if a[i].Cmp(q.lo[i]) < 0 || a[i].Cmp(q.hi[i]) > 0 {
				return false
			}
			continue
		}
		l, h := div(sub(q.lo[i], a[i]), d), div(sub(q.hi[i], a[i]), d)
		b.check(point{l, h})
		if b.err != nil {
			return true
		}
		if d.Sign() < 0 {
			l, h = h, l
		}
		if l.Cmp(lo) > 0 {
			lo = l
		}
		if h.Cmp(hi) < 0 {
			hi = h
		}
	}
	return lo.Cmp(hi) <= 0
}

func validateLoops(ctx context.Context, outer Loop, holes []Loop, query *point, o Options, retain **CertifiedLoops, forest *[]RingPlacement, boxes ...rationalQueryBox) (Report, Location, error) {
	r := Report{Status: Unresolved, LoopA: -1, EdgeA: -1, LoopB: -1, EdgeB: -1}
	o, e := o.WithDefaults()
	if ctx == nil || e != nil {
		return r, Unknown, ErrOptions
	}
	b := &budget{ctx: ctx, o: o}
	location := Unknown
	finish := func(status Status, reason string) (Report, Location, error) {
		r.Status, r.Reason, r.Work, r.Subdivisions = status, reason, b.work, b.subdivisions
		if b.err != nil && !errors.Is(b.err, ErrLimit) {
			return r, Unknown, b.err
		}
		return r, location, nil
	}
	if !b.tick() {
		return finish(Unresolved, "proof budget or cancellation")
	}
	var loops [][]curve
	var signs []int
	var all []curve
	for li, uses := range append([]Loop{outer}, holes...) {
		r.LoopA = li
		r.EdgeA = -1
		if len(uses) < 2 {
			return finish(Invalid, "loop has fewer than two segments")
		}
		var loop []curve
		for ei, u := range uses {
			r.EdgeA = ei
			if !b.tick() {
				return finish(Unresolved, "proof budget or cancellation")
			}
			if len(all) >= o.MaxEdges {
				return finish(Unresolved, "edge budget exceeded")
			}
			c := curve{loop: li, edge: ei}
			if (u.Kind == exactRationalQuadratic || u.Kind == exactRationalBezier) && u.exact != nil {
				c.p, c.w, c.controls, c.weights = u.exact.p, u.exact.w, u.exact.controls, u.exact.weights
			} else {
				if !finite(u.Start) || !finite(u.End) {
					return finish(Invalid, "nonfinite UV endpoints")
				}
				c.p[0], c.p[2] = exact(u.Start), exact(u.End)
			}
			if len(loop) != 0 && !same(loop[len(loop)-1].p[2], c.p[0]) {
				return finish(Invalid, "disconnected UV endpoints")
			}
			if same(c.p[0], c.p[2]) {
				if c.controls != nil {
					return finish(Unresolved, "closed higher degree proof span")
				}
				return finish(Invalid, "zero-length or retraced edge")
			}
			switch u.Kind {
			case Line:
				c.p[1] = midpoint(c.p[0], c.p[2])
			case Quadratic, RationalQuadratic:
				if !finite(u.Control) {
					return finish(Invalid, "nonfinite quadratic control")
				}
				c.p[1] = exact(u.Control)
				if u.Kind == RationalQuadratic {
					for i, w := range u.Weights {
						if !finite(geom.Vec2{X: w}) || w <= 0 {
							return finish(Invalid, "rational weights must be finite and positive")
						}
						c.w[i] = num(w)
					}
				}
			case exactRationalQuadratic, exactRationalBezier:
				if u.exact == nil {
					return finish(Unresolved, "missing exact rational control")
				}
			default:
				return finish(Unresolved, "unsupported curve representation")
			}

			for _, p := range c.points() {
				b.check(p)
			}
			if c.w[0] != nil {
				for _, w := range c.w {
					b.check(point{w, w})
				}
				if c.w[0].Cmp(c.w[1]) == 0 && c.w[1].Cmp(c.w[2]) == 0 {
					c.w = [3]q{}
				}
			}
			if b.err != nil {
				return finish(Unresolved, "rational bit budget exceeded")
			}
			if c.controls != nil {
				for _, w := range c.weights {
					b.check(point{w, w})
				}
				if b.err != nil {
					return finish(Unresolved, "rational bit budget exceeded")
				}
				if !certifyBezierMonotone(c, minus(c.p[2], c.p[0]), b) {
					return finish(Unresolved, "higher degree trim lacks a proved monotone chord projection")
				}
			} else if cross(minus(c.p[1], c.p[0]), minus(c.p[2], c.p[0])).Sign() == 0 {
				if !onSegment(c.p[1], c.p[0], c.p[2]) {
					return finish(Invalid, "quadratic retraces a line")
				}
				c.line = true
				c.w = [3]q{}
				c.p[1] = midpoint(c.p[0], c.p[2])
			}
			loop = append(loop, c)
			all = append(all, c)
		}
		if !same(loop[len(loop)-1].p[2], loop[0].p[0]) {
			return finish(Invalid, "disconnected UV endpoints")
		}
		sign, ok := orientation(loop, b)
		if !ok {
			return finish(Unresolved, "orientation proof budget exhausted")
		}
		if forest == nil && (li == 0 && sign <= 0 || li > 0 && sign >= 0) {
			return finish(Invalid, "outer must be counterclockwise and holes clockwise")
		}
		if forest != nil {
			if sign == 0 {
				return finish(Invalid, "ring has zero orientation")
			}
			signs = append(signs, sign)
		}
		loops = append(loops, loop)
	}
	for i, a := range all {
		for _, c := range all[i+1:] {
			r.LoopA, r.EdgeA, r.LoopB, r.EdgeB = a.loop, a.edge, c.loop, c.edge
			var allowed [2]*point
			if a.loop == c.loop {
				n := len(loops[a.loop])
				if (a.edge+1)%n == c.edge {
					x := a.p[2]
					allowed[0] = &x
				}
				if (c.edge+1)%n == a.edge {
					x := c.p[2]
					if allowed[0] == nil {
						allowed[0] = &x
					} else {
						allowed[1] = &x
					}
				}
			}
			status := separate(a, c, allowed, b)
			if status == Invalid {
				return finish(Invalid, "proved trim intersection or contact")
			}
			if status == Unresolved {
				return finish(Unresolved, "intersection proof budget exhausted")
			}
		}
	}
	if forest != nil {
		rings, status, reason := classifyRingForest(loops, signs, &r, b)
		if status == Certified {
			*forest = rings
			if retain != nil {
				*retain = &CertifiedLoops{rings: loops, edges: all, options: o, forest: true}
			}
			r.LoopA, r.EdgeA, r.LoopB, r.EdgeB = -1, -1, -1, -1
		}
		return finish(status, reason)
	}
	for i := 1; i < len(loops); i++ {
		r.LoopA, r.EdgeA = i, 0
		for j := 0; j < len(loops); j++ {
			if i == j {
				continue
			}
			r.LoopB, r.EdgeB = j, -1
			w, ok := winding(loops[i][0].p[0], loops[j], b)
			if !ok {
				return finish(Unresolved, "containment proof budget exhausted")
			}
			if j == 0 && w != 1 {
				return finish(Invalid, "hole is outside outer loop")
			}
			if j > 0 && w != 0 {
				return finish(Invalid, "nested holes")
			}
		}
	}
	r.LoopA, r.EdgeA, r.LoopB, r.EdgeB = -1, -1, -1, -1
	if retain != nil {
		*retain = &CertifiedLoops{rings: loops, edges: all, options: o}
	}
	var status Status
	var reason string
	location, status, reason = locateCertifiedLoops(loops, all, query, &r, b, false, boxes...)
	return finish(status, reason)
}

func locateCertifiedLoops(loops [][]curve, all []curve, query *point, r *Report, b *budget, forest bool, boxes ...rationalQueryBox) (Location, Status, string) {
	if len(boxes) == 1 {
		box := boxes[0]
		b.check(box.lo)
		b.check(box.hi)
		for _, c := range all {
			if !box.excludes(c, b) {
				r.LoopA, r.EdgeA = c.loop, c.edge
				return Unknown, Unresolved, "box separation from trim boundary unproved"
			}
		}
		p := midpoint(box.lo, box.hi)
		query = &p
	}
	if query != nil {
		b.check(*query)
		if b.err != nil {
			return Unknown, Unresolved, "rational query exceeds bit budget"
		}
		pointProofPending := false
		for _, c := range all {
			if !b.tick() {
				return Unknown, Unresolved, "point location budget exhausted"
			}
			if onCurve(*query, c) {
				return Boundary, Certified, "exact boundary point"
			}
			if c.controls != nil && !outsideHull(*query, c) {
				hit, complete := b.locateBezierPoint(*query, c)
				if hit {
					return Boundary, Certified, "exact algebraic boundary point"
				}
				pointProofPending = pointProofPending || !complete
			}
		}
		windingSum := 0
		for i, loop := range loops {
			w, ok := winding(*query, loop, b)
			if !ok {
				r.LoopA = i
				if pointProofPending {
					return Unknown, Unresolved, "higher degree point location proof exhausted"
				}
				return Unknown, Unresolved, "point location budget exhausted or boundary unresolved"
			}
			if !forest && (i == 0 && w == 0 || i != 0 && w != 0) {
				return Outside, Certified, "exact winding proves point outside material"
			}
			windingSum += w
		}
		if forest && windingSum == 0 {
			return Outside, Certified, "total winding is zero"
		}
		return Inside, Certified, "exact winding proves point inside material"
	}
	return Unknown, Certified, "simple oriented loops and disjoint contained holes proved"
}
