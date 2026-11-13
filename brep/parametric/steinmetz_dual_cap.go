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
	radius     float64
	caps       [2][2]float64
	fixed      [2][2]bool
	axis, side int
}

func (s steinmetzDualCapChart) valid() bool {
	if !s.center.IsFinite() || !finite(s.radius) || s.radius <= 0 || s.axis < 0 || s.axis >= 2 || s.side < 0 || s.side >= 2 {
		return false
	}
	for axis, pair := range s.caps {
		if !finite(pair[0]) || !finite(pair[1]) || pair[0] < -s.radius || pair[1] > s.radius || pair[0] >= pair[1] {
			return false
		}
		for side, cap := range pair {
			if s.fixed[axis][side] && math.Abs(cap) >= s.radius {
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
	constant, free := 2*s.axis, 2*(1-s.axis)
	endpoint := func(axis, side int) (float64, float64) {
		if s.fixed[axis][side] {
			return s.caps[axis][side], 0
		}
		if side == 0 {
			return -r, -dr
		}
		return r, dr
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
	return upMul(3, s.radius), upMul(2, s.radius), 0, nil
}

// finiteSteinmetzDualUpperCaps retains x<=caps[0] and z<=caps[1].
func finiteSteinmetzDualUpperCaps(ctx context.Context, center geom.Vec3, radius float64, caps [2]float64, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	return finiteSteinmetzCapWindows(ctx, center, radius, [2][2]float64{{-radius, caps[0]}, {-radius, caps[1]}}, tolerance, CylinderCommonPerpendicularDualCap)
}

// finiteSteinmetzCapWindows clips each rectangular Y slice by both raw axial
// windows. A sign-stable angular band has at most four native boundary charts.
func finiteSteinmetzCapWindows(ctx context.Context, center geom.Vec3, radius float64, windows [2][2]float64, tolerance float64, construction CylinderCommonConstruction) (FiniteCylinderPairCommonResult, error) {
	var zero FiniteCylinderPairCommonResult
	if ctx == nil || !center.IsFinite() || radius <= 0 || !finite(radius) || !finite(2*radius) || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	far := 2 * radius
	cuts := []float64{-math.Pi / 2, math.Pi / 2}
	seen := map[float64]bool{}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	for axis, window := range windows {
		if !finite(window[0]) || !finite(window[1]) || window[0] >= window[1] {
			return zero, geom.ErrInvalid
		}
		if window[0] >= radius || window[1] <= -radius {
			return zero, nil
		}
		windows[axis] = [2]float64{max(-radius, window[0]), min(radius, window[1])}
		w := windows[axis]
		if new(big.Rat).Sub(q(w[1]), q(w[0])).Cmp(pad) <= 0 {
			return zero, geom.ErrConvergence
		}
		for _, cap := range w {
			abs := math.Abs(cap)
			if abs == 0 || abs == radius || seen[abs] {
				continue
			}
			proof, err := PerpendicularCylinderAlgebraicCap(ctx, geom.Vec3{}, radius, abs, far, far, tolerance, geom.ExactCurvePlaneEventOptions{})
			if err != nil {
				return zero, err
			}
			seen[abs] = true
			cuts = append(cuts, -proof.SplitAngle, proof.SplitAngle)
		}
	}
	sort.Float64s(cuts)
	out := FiniteCylinderPairCommonResult{Construction: construction}
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
		var fixed [2][2]bool
		active := true
		for axis, window := range windows {
			// If the interval excludes zero, its nearer cap defines where
			// the slice vanishes; otherwise every interior Y is active.
			threshold := max(window[0], -window[1])
			if threshold > 0 && hi.Cmp(q(threshold)) < 0 {
				active = false
				break
			}
			for side, cap := range window {
				// Supporting caps at ±radius never own positive area.
				if math.Abs(cap) == radius {
					continue
				}
				abs := q(math.Abs(cap))
				if cap != 0 && lo.Cmp(abs) <= 0 && hi.Cmp(abs) >= 0 {
					return zero, geom.ErrConvergence
				}
				fixed[axis][side] = (side == 0 && cap >= 0 || side == 1 && cap <= 0) || lo.Cmp(abs) > 0
			}
		}
		if !active {
			continue
		}
		for axis := range 2 {
			for side := range 2 {
				id := len(faces)
				faces = append(faces, Face{Surface: steinmetzDualCapChart{center, radius, windows, fixed, axis, side}, U: u, V: Interval{0, 1}, Reversed: (axis == 0) == (side == 0)})
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
}
