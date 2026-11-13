package intersect

import (
	"math/big"

	"plainsolid/geom"
)

// SplineChartMap describes a correspondence between two positive-area
// parameter rectangles. Transposed interchanges A's U and V before mapping
// into B. ReverseU and ReverseV reverse the respective coordinates in B.
type SplineChartMap struct {
	DomainA, DomainB               geom.ParameterRectangle
	Transposed, ReverseU, ReverseV bool
}

// Orientation returns the sign of the parameter Jacobian. On regular surface
// points it is also the relative sign of the two chart normals. Zero denotes
// an invalid map.
func (m SplineChartMap) Orientation() int {
	if !m.DomainA.Valid() || !m.DomainB.Valid() {
		return 0
	}
	if (m.Transposed != m.ReverseU) != m.ReverseV {
		return -1
	}
	return 1
}

// Inverse exchanges the two parameter rectangles and inverts the map.
func (m SplineChartMap) Inverse() SplineChartMap {
	m.DomainA, m.DomainB = m.DomainB, m.DomainA
	if m.Transposed {
		m.ReverseU, m.ReverseV = m.ReverseV, m.ReverseU
	}
	return m
}

// Map returns the corresponding parameter on B, rounded to the nearest
// float64. The exact affine expression is evaluated before rounding.
func (m SplineChartMap) Map(uv geom.Vec2) (geom.Vec2, error) {
	p, _, e := m.mapExact(uv)
	return p, e
}

// MapExact returns the corresponding B parameter only if both coordinates
// can be represented exactly as float64. It is suitable for matching vertices
// where rounding could otherwise create a false coincidence.
func (m SplineChartMap) MapExact(uv geom.Vec2) (geom.Vec2, error) {
	p, exact, e := m.mapExact(uv)
	if e == nil && !exact {
		e = geom.ErrUnrepresentable
	}
	return p, e
}

func (m SplineChartMap) mapExact(uv geom.Vec2) (geom.Vec2, bool, error) {
	if m.Orientation() == 0 || !finite(uv.X) || !finite(uv.Y) {
		return geom.Vec2{}, false, geom.ErrInvalid
	}
	a, b := m.DomainA, m.DomainB
	if uv.X < a.UMin || uv.X > a.UMax || uv.Y < a.VMin || uv.Y > a.VMax {
		return geom.Vec2{}, false, ErrDomain
	}
	u, v := exactChartFraction(uv.X, a.UMin, a.UMax), exactChartFraction(uv.Y, a.VMin, a.VMax)
	if m.Transposed {
		u, v = v, u
	}
	x, ex := interpolateChartFraction(u, b.UMin, b.UMax, m.ReverseU)
	y, ey := interpolateChartFraction(v, b.VMin, b.VMax, m.ReverseV)
	return geom.Vec2{X: x, Y: y}, ex && ey, nil
}

func exactChartFraction(x, lo, hi float64) *big.Rat {
	r := new(big.Rat).SetFloat64(lo)
	n := new(big.Rat).Sub(new(big.Rat).SetFloat64(x), r)
	d := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), r)
	return n.Quo(n, d)
}

func interpolateChartFraction(t *big.Rat, lo, hi float64, reverse bool) (float64, bool) {
	if reverse {
		t.Sub(big.NewRat(1, 1), t)
	}
	r := new(big.Rat).SetFloat64(lo)
	d := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), r)
	d.Mul(d, t).Add(d, r)
	return d.Float64()
}
