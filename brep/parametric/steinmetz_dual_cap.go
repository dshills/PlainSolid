package parametric

import (
	"context"
	"math"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// steinmetzDualCapChart traces one side of the rectangle at fixed y. Its
// free radial endpoint is ±r*cos(u); a fixed endpoint is an original cap.
// Splitting U at each exchange of a radial endpoint and a cap retains
// differentiable charts and shared boundary edges.
type steinmetzDualCapChart struct {
	center     geom.Vec3
	radius     float64 // smaller cross-section radius, setting Y=radius*sin(u)
	large      float64 // zero for equal radii
	wideAxis   int     // X/Z coordinate whose width uses the larger radius
	caps       [2][2]float64
	fixed      [2][2]bool
	axis, side int
}

func (s steinmetzDualCapChart) valid() bool {
	if !s.center.IsFinite() || !finite(s.radius) || s.radius <= 0 || s.axis < 0 || s.axis >= 2 || s.side < 0 || s.side >= 2 {
		return false
	}
	if s.large != 0 && (!finite(s.large) || s.large <= s.radius || s.wideAxis < 0 || s.wideAxis > 1) {
		return false
	}
	for axis, pair := range s.caps {
		limit := s.radius
		if s.large > 0 && axis == s.wideAxis {
			limit = s.large
		}
		if !finite(pair[0]) || !finite(pair[1]) || pair[0] < -limit || pair[1] > limit || pair[0] >= pair[1] {
			return false
		}
		for side, cap := range pair {
			if s.fixed[axis][side] && math.Abs(cap) >= limit {
				return false
			}
		}
	}
	return true
}

func (s steinmetzDualCapChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, err := s.evaluate(u, v)
	return p, err
}

func (s steinmetzDualCapChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, du, dv, err := s.evaluate(u, v)
	return du, dv, err
}

func (s steinmetzDualCapChart) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !s.valid() || !finite(u) || !finite(v) || math.Abs(u) > math.Pi/2 || v < 0 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	sin, cos := math.Sincos(u)
	if math.Abs(u) == math.Pi/2 {
		cos = 0
	}
	r, dr := s.radius*cos, -s.radius*sin
	var p, du, dv [3]float64
	p[1], du[1] = s.radius*sin, r
	width, derivative := [2]float64{r, r}, [2]float64{dr, dr}
	if s.large > 0 {
		y := p[1] / s.large
		w := math.Sqrt((1-y)*(1+y)) * s.large
		if !finite(w) || w <= 0 {
			return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
		}
		width[s.wideAxis], derivative[s.wideAxis] = w, -p[1]/w*r
	}
	constant, free := 2*s.axis, 2*(1-s.axis)
	endpoint := func(axis, side int) (float64, float64) {
		if s.fixed[axis][side] {
			return s.caps[axis][side], 0
		}
		if side == 0 {
			return -width[axis], -derivative[axis]
		}
		return width[axis], derivative[axis]
	}
	p[constant], du[constant] = endpoint(s.axis, s.side)
	start, dStart := endpoint(1-s.axis, 0)
	end, dEnd := endpoint(1-s.axis, 1)
	p[free], du[free], dv[free] = start+(end-start)*v, dStart+(dEnd-dStart)*v, end-start
	if v == 1 {
		p[free], du[free] = end, dEnd
	}
	point := s.center.Add(geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
	if !point.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return point, geom.Vec3{X: du[0], Y: du[1], Z: du[2]}, geom.Vec3{X: dv[0], Y: dv[1], Z: dv[2]}, nil
}

func (s steinmetzDualCapChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, 0, err
	}
	if !s.valid() || u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	if s.large == 0 {
		return upMul(3, s.radius), upMul(2, s.radius), 0, nil
	}
	base, mixed, _, err := (unequalCylinderCommonChart{small: s.radius, large: s.large}).secondDerivativeBounds(ctx, u, v)
	return upMul(3, base), upMul(2, mixed), 0, err
}

// finiteSteinmetzDualUpperCaps retains x<=caps[0] and z<=caps[1].
func finiteSteinmetzDualUpperCaps(ctx context.Context, center geom.Vec3, radius float64, caps [2]float64, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	return finiteSteinmetzCapWindows(ctx, center, radius, [2][2]float64{{-radius, caps[0]}, {-radius, caps[1]}}, tolerance, CylinderCommonPerpendicularDualCap)
}

// finiteSteinmetzCapWindows clips each rectangular Y slice by both raw axial
// windows. A sign-stable angular band has at most four native boundary charts.
func finiteSteinmetzCapWindows(ctx context.Context, center geom.Vec3, radius float64, windows [2][2]float64, tolerance float64, construction CylinderCommonConstruction) (FiniteCylinderPairCommonResult, error) {
	return finiteSteinmetzRadiusWindows(ctx, center, [2]float64{radius, radius}, windows, tolerance, construction)
}

func finiteSteinmetzRadiusWindows(ctx context.Context, center geom.Vec3, widths [2]float64, windows [2][2]float64, tolerance float64, construction CylinderCommonConstruction) (FiniteCylinderPairCommonResult, error) {
	var zero FiniteCylinderPairCommonResult
	radius, large := min(widths[0], widths[1]), max(widths[0], widths[1])
	wide := 0
	if widths[1] > widths[0] {
		wide = 1
	}
	if ctx == nil || !center.IsFinite() || radius <= 0 || !finite(radius) || !finite(2*large) || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	cuts := []float64{-math.Pi / 2, math.Pi / 2}
	seen := map[string]bool{}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	square := func(x float64) *big.Rat { r := q(x); return new(big.Rat).Mul(r, r) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if q(radius).Cmp(pad) <= 0 || large > radius && new(big.Rat).Sub(q(large), q(radius)).Cmp(pad) <= 0 {
		return zero, geom.ErrConvergence
	}
	for axis, window := range windows {
		limit := widths[axis]
		if !finite(window[0]) || !finite(window[1]) || window[0] >= window[1] {
			return zero, geom.ErrInvalid
		}
		if window[0] >= limit || window[1] <= -limit {
			return zero, nil
		}
		windows[axis] = [2]float64{max(-limit, window[0]), min(limit, window[1])}
		w := windows[axis]
		if new(big.Rat).Sub(q(w[1]), q(w[0])).Cmp(pad) <= 0 {
			return zero, geom.ErrConvergence
		}
		for _, cap := range w {
			abs := math.Abs(cap)
			if abs == 0 || abs == limit {
				continue
			}
			ordinate := new(big.Rat).Sub(square(limit), square(abs))
			if limit > radius {
				// At the ends of the shared Y span the wide coordinate
				// still has positive width. A cap almost tangent there
				// creates a strip narrower than the sewing tolerance.
				minWidth2 := new(big.Rat).Sub(square(limit), square(radius))
				if c2 := square(abs); c2.Cmp(minWidth2) != 0 {
					low := new(big.Rat).Sub(q(abs), pad)
					if low.Sign() < 0 {
						low.SetInt64(0)
					}
					high := new(big.Rat).Add(q(abs), pad)
					if new(big.Rat).Mul(low, low).Cmp(minWidth2) <= 0 && new(big.Rat).Mul(high, high).Cmp(minWidth2) >= 0 {
						return zero, geom.ErrConvergence
					}
				}
			}
			if ordinate.Cmp(square(radius)) >= 0 {
				continue
			}
			key := ordinate.RatString()
			if seen[key] {
				continue
			}
			angle, err := steinmetzRadiusSplit(ctx, radius, ordinate, tolerance)
			if err != nil {
				return zero, err
			}
			seen[key] = true
			cuts = append(cuts, -angle, angle)
		}
	}
	sort.Float64s(cuts)
	out := FiniteCylinderPairCommonResult{Construction: construction}
	chartLarge, chartWide := large, wide
	if large == radius {
		chartLarge, chartWide = 0, 0
	}
	var faces []Face
	for i := 0; i+1 < len(cuts); i++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		u := Interval{cuts[i], cuts[i+1]}
		if !u.valid() {
			return zero, geom.ErrConvergence
		}
		lo, hi := steinmetzCosBounds(radius, (u.Min+u.Max)/2)
		if lo.Sign() < 0 {
			lo = new(big.Rat)
		}
		lo2, hi2 := new(big.Rat).Mul(lo, lo), new(big.Rat).Mul(hi, hi)
		var fixed [2][2]bool
		active := true
		for axis, window := range windows {
			limit := widths[axis]
			diff := new(big.Rat).Sub(square(limit), square(radius))
			low, high := new(big.Rat).Add(lo2, diff), new(big.Rat).Add(hi2, diff)
			threshold := max(window[0], -window[1])
			if threshold > 0 && high.Cmp(square(threshold)) < 0 {
				active = false
				break
			}
			for side, cap := range window {
				if math.Abs(cap) == limit {
					continue
				}
				c2 := square(cap)
				if cap != 0 && low.Cmp(c2) <= 0 && high.Cmp(c2) >= 0 {
					return zero, geom.ErrConvergence
				}
				fixed[axis][side] = (side == 0 && cap >= 0 || side == 1 && cap <= 0) || low.Cmp(c2) > 0
			}
		}
		if !active {
			continue
		}
		for axis := range 2 {
			for side := range 2 {
				id := len(faces)
				faces = append(faces, Face{Surface: steinmetzDualCapChart{center: center, radius: radius, large: chartLarge, wideAxis: chartWide, caps: windows, fixed: fixed, axis: axis, side: side}, U: u, V: Interval{0, 1}, Reversed: (axis == 0) == (side == 0)})
				if !fixed[axis][side] {
					out.SideFaces[1-axis] = append(out.SideFaces[1-axis], id)
				} else if side == 0 {
					out.LowerCapFaces[axis] = append(out.LowerCapFaces[axis], id)
				} else {
					out.UpperCapFaces[axis] = append(out.UpperCapFaces[axis], id)
				}
			}
		}
	}
	if len(faces) == 0 {
		return zero, geom.ErrConvergence
	}
	var err error
	out.Solid, err = joinPrimitivePatches(faces, tolerance)
	if err != nil {
		return zero, err
	}
	return out, nil
}

type steinmetzDualCapData struct {
	Caps       [2]float64
	Fixed      [2]bool
	Axis, Side int
	Windows    *[2][2]float64 `json:",omitempty"`
	Clipped    *[2][2]bool    `json:",omitempty"`
	Large      float64        `json:",omitempty"`
	WideAxis   int            `json:",omitempty"`
}

// Both sin and cos coordinates of the angular split are algebraic. Reuse
// the established cap split enclosure with a bounded rational cosine proxy;
// also bound the proxy's difference from the exact positive cosine length.
func steinmetzRadiusSplit(ctx context.Context, radius float64, ordinate *big.Rat, tolerance float64) (float64, error) {
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	r := q(radius)
	cosine2 := new(big.Rat).Sub(new(big.Rat).Mul(r, r), ordinate)
	if cosine2.Sign() <= 0 || ordinate.Sign() <= 0 {
		return 0, geom.ErrConvergence
	}
	o := geom.ExactCurvePlaneEventOptions{}
	root := func(v *big.Rat) (geom.ExactParameterInterval, error) {
		a, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{new(big.Rat).Neg(v).RatString(), "0", "1"}, Lower: "0", Upper: r.RatString()}, geom.ExactRootCodecOptions{})
		if e != nil {
			return geom.ExactParameterInterval{}, e
		}
		b, e := a.Enclose(ctx, new(big.Rat).Quo(q(tolerance), big.NewRat(128, 1)), o)
		if e != nil {
			return geom.ExactParameterInterval{}, e
		}
		if !b.Converged {
			return geom.ExactParameterInterval{}, geom.ErrConvergence
		}
		return b.Interval, nil
	}
	a, e := root(ordinate)
	if e != nil {
		return 0, e
	}
	c, e := root(cosine2)
	if e != nil {
		return 0, e
	}
	mid := func(i geom.ExactParameterInterval) float64 {
		f, _ := new(big.Rat).Quo(new(big.Rat).Add(i.Lower, i.Upper), big.NewRat(2, 1)).Float64()
		return f
	}
	cv, av := mid(c), mid(a)
	if !finite(cv) || !finite(av) || cv <= 0 || av <= 0 {
		return 0, geom.ErrConvergence
	}
	angle, _, _, _, e := steinmetzSplitAngle(ctx, radius, cv, tolerance/2, av, a.Lower, a.Upper)
	if e != nil {
		return 0, e
	}
	limit := new(big.Rat).Quo(q(tolerance), big.NewRat(16, 1))
	for _, v := range []*big.Rat{c.Lower, c.Upper} {
		if new(big.Rat).Abs(new(big.Rat).Sub(q(cv), v)).Cmp(limit) > 0 {
			return 0, geom.ErrConvergence
		}
	}
	return angle, nil
}
