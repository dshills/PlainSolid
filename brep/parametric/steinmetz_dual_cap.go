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
	caps       [2]float64
	fixed      [2]bool
	axis, side int
}

func (s steinmetzDualCapChart) valid() bool {
	return s.center.IsFinite() && finite(s.radius) && s.radius > 0 &&
		finite(s.caps[0]) && finite(s.caps[1]) && math.Abs(s.caps[0]) < s.radius && math.Abs(s.caps[1]) < s.radius &&
		s.axis >= 0 && s.axis < 2 && s.side >= 0 && s.side < 2
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
	p[constant], du[constant] = -r, -dr
	if s.side == 1 {
		p[constant], du[constant] = r, dr
		if s.fixed[s.axis] {
			p[constant], du[constant] = s.caps[s.axis], 0
		}
	}
	end, dEnd := r, dr
	if s.fixed[1-s.axis] {
		end, dEnd = s.caps[1-s.axis], 0
	}
	p[free], du[free], dv[free] = -r+(end+r)*v, -dr+(dEnd+dr)*v, end+r
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
// The two other raw caps have already been proved nonclipping. Coincident
// absolute cap offsets share a split, preserving the original cap owners.
func finiteSteinmetzDualUpperCaps(ctx context.Context, center geom.Vec3, radius float64, caps [2]float64, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	var zero FiniteCylinderPairCommonResult
	if ctx == nil || !center.IsFinite() || radius <= 0 || !finite(radius) || !finite(2*radius) || !finite(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	far := 2 * radius
	cuts := []float64{-math.Pi / 2, math.Pi / 2}
	seen := map[float64]bool{}
	for _, cap := range caps {
		abs := math.Abs(cap)
		if !finite(abs) || abs >= radius {
			return zero, geom.ErrDomain
		}
		if cap == 0 || seen[abs] {
			continue
		}
		proof, err := PerpendicularCylinderAlgebraicCap(ctx, geom.Vec3{}, radius, abs, far, far, tolerance, geom.ExactCurvePlaneEventOptions{})
		if err != nil {
			return zero, err
		}
		seen[abs] = true
		cuts = append(cuts, -proof.SplitAngle, proof.SplitAngle)
	}
	sort.Float64s(cuts)
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	out := FiniteCylinderPairCommonResult{Construction: CylinderCommonPerpendicularDualCap}
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
		var fixed [2]bool
		active := true
		for axis, cap := range caps {
			abs := q(math.Abs(cap))
			if cap < 0 && hi.Cmp(abs) < 0 {
				active = false
				break
			}
			if cap != 0 && lo.Cmp(abs) <= 0 && hi.Cmp(abs) >= 0 {
				return zero, geom.ErrConvergence
			}
			fixed[axis] = cap <= 0 || lo.Cmp(abs) > 0
		}
		if !active {
			continue
		}
		for axis := range 2 {
			for side := range 2 {
				id := len(faces)
				faces = append(faces, Face{Surface: steinmetzDualCapChart{center, radius, caps, fixed, axis, side}, U: u, V: Interval{0, 1}, Reversed: (axis == 0) == (side == 0)})
				if side == 1 && fixed[axis] {
					out.UpperCapFaces[axis] = append(out.UpperCapFaces[axis], id)
				} else {
					out.SideFaces[1-axis] = append(out.SideFaces[1-axis], id)
				}
			}
		}
	}
	if len(faces) == 0 || len(out.UpperCapFaces[0]) == 0 || len(out.UpperCapFaces[1]) == 0 {
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
}
