package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// SteinmetzSeam is an immutable analytic elliptical half-arc. Its ideal model
// is center+r*(sx*cos(u),sin(u),sz*cos(u)), u=π(t−1/2), optionally reversed.
// Thus both cylinder equations equal r² identically, and x/sx=z/sz along the
// seam. Eval is floating analytic evaluation, not an exact coordinate oracle.
type SteinmetzSeam struct {
	center   geom.Vec3
	radius   float64
	sx, sz   int
	reversed bool
}

func (s SteinmetzSeam) Parameters() (geom.Vec3, float64, int, int, bool) {
	return s.center, s.radius, s.sx, s.sz, s.reversed
}
func (s SteinmetzSeam) valid() bool {
	return s.center.IsFinite() && finite(s.radius) && s.radius > 0 && (s.sx == 1 || s.sx == -1) && (s.sz == 1 || s.sz == -1)
}
func (s SteinmetzSeam) Eval(t float64) (geom.Vec3, error) {
	if !s.valid() || !finite(t) || t < 0 || t > 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	if s.reversed {
		t = 1 - t
	}
	u := -math.Pi/2 + math.Pi*t
	sin, cos := math.Sincos(u)
	if t == 0 || t == 1 {
		cos = 0
	}
	p := s.center.Add(geom.Vec3{X: float64(s.sx) * s.radius * cos, Y: s.radius * sin, Z: float64(s.sz) * s.radius * cos})
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}
func (s SteinmetzSeam) Derivative(t float64) (geom.Vec3, error) {
	if !s.valid() || !finite(t) || t < 0 || t > 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	sign := 1.
	if s.reversed {
		t = 1 - t
		sign = -1
	}
	sin, cos := math.Sincos(-math.Pi/2 + math.Pi*t)
	if t == 0 || t == 1 {
		cos = 0
	}
	k := s.radius * math.Pi * sign
	p := geom.Vec3{X: -float64(s.sx) * k * sin, Y: k * cos, Z: -float64(s.sz) * k * sin}
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}

// IdealChordSquaredBound bounds BOTH directed distances between the ideal
// arc on [start,end] and its ideal endpoint chord by their common normalized
// parameter correspondence. The second derivative squared is ≤2r²π⁴, giving
// squared interpolation error ≤r²π⁴(end-start)⁴/32. The strict rational bound
// π<22/7 removes transcendental rounding. This excludes rounded endpoint or
// evaluation error; it is not a displayed-polyline certificate.
func (s SteinmetzSeam) IdealChordSquaredBound(start, end float64) (*big.Rat, error) {
	if !s.valid() || !finite(start) || !finite(end) || start < 0 || end > 1 || start > end {
		return nil, geom.ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	d := new(big.Rat).Sub(q(end), q(start))
	d.Mul(d, d)
	d.Mul(d, d)
	p := big.NewRat(22, 7)
	p.Mul(p, p)
	p.Mul(p, p)
	return new(big.Rat).Quo(new(big.Rat).Mul(new(big.Rat).Mul(q(s.radius), q(s.radius)), new(big.Rat).Mul(p, d)), big.NewRat(32, 1)), nil
}

// PerpendicularCylinderCommonExplicit retains the same four-cylinder topology
// as PerpendicularCylinderCommon while replacing surface-derived iso edges
// with explicit analytic elliptical seam models. Pole edges remain collapsed;
// all vertex, edge and face identities are unchanged.
func PerpendicularCylinderCommonExplicit(ctx context.Context, center geom.Vec3, radius, xHalfLength, zHalfLength, tolerance float64) (SteinmetzCommon, error) {
	out, e := PerpendicularCylinderCommon(ctx, center, radius, xHalfLength, zHalfLength, tolerance)
	if e != nil {
		return SteinmetzCommon{}, e
	}
	edges := out.Solid.Edges()
	for i, edge := range edges {
		if edge.Curve == nil {
			continue
		}
		iso, ok := edge.Curve.(iso)
		if !ok {
			return SteinmetzCommon{}, ErrTopology
		}
		chart, ok := iso.face.Surface.(steinmetzChart)
		if !ok || (iso.side != 0 && iso.side != 2) {
			return SteinmetzCommon{}, ErrTopology
		}
		other := -1
		if iso.side == 2 {
			other = 1
		}
		sx, sz := chart.sign, other
		if chart.axis == 1 {
			sx, sz = other, chart.sign
		}
		edges[i].Curve = SteinmetzSeam{chart.center, chart.radius, sx, sz, iso.side == 2}
	}
	out.Solid, e = NewSolid(out.Solid.Vertices(), edges, out.Solid.Faces(), tolerance)
	if e != nil {
		return SteinmetzCommon{}, e
	}
	if e = ctx.Err(); e != nil {
		return SteinmetzCommon{}, e
	}
	return out, nil
}
