package iges

import (
	"context"
	"io"
	"math"
	"math/big"
	"plainsolid/geom"
)

// QuarterEllipseConic retains one positively oriented local cardinal quarter.
// StartQuarter 0..3 selects +X→+Y, +Y→−X, −X→−Y, or −Y→+X. A proper signed
// placement may reverse apparent world-plane winding while preserving its own
// normal. Swapping endpoints alone denotes a complementary arc and is rejected.
type QuarterEllipseConic struct {
	EllipseConicModel
	StartQuarter int
}

// AngularRange gives the natural conic-radian interval. Returned π multiples
// are numerical values; the accepted endpoint roles are exact cardinal axes.
func (q QuarterEllipseConic) AngularRange() (float64, float64, error) {
	if q.StartQuarter < 0 || q.StartQuarter > 3 {
		return 0, 0, geom.ErrInvalid
	}
	return float64(q.StartQuarter) * math.Pi / 2, float64(q.StartQuarter+1) * math.Pi / 2, nil
}

// ExactCurve returns an immutable exact rational quadratic on [0,1]. Its
// angular parameter is qπ/2+2atan(t); the weights (1,1,2) establish the complete
// quadrant identity. This is a parameter change, not an angle-linear spline.
func (q QuarterEllipseConic) ExactCurve(ctx context.Context, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	if q.StartQuarter < 0 || q.StartQuarter > 3 {
		return nil, geom.ErrInvalid
	}
	if _, e := q.Curve.Geometry(); e != nil {
		return nil, e
	}
	a, b, e := ellipseConicRadii(q.Curve)
	if e != nil {
		return nil, e
	}
	m := geom.Identity()
	if q.Curve.Placement != nil {
		m = *q.Curve.Placement
	}
	h := make([][4]*big.Rat, 3)
	for i, p := range [][2]int64{{1, 0}, {1, 1}, {0, 1}} {
		x, y := p[0], p[1]
		for j := 0; j < q.StartQuarter; j++ {
			x, y = -y, x
		}
		local := [3]*big.Rat{new(big.Rat).Mul(tetraRat(a), big.NewRat(x, 1)), new(big.Rat).Mul(tetraRat(b), big.NewRat(y, 1)), tetraRat(q.Curve.Z)}
		w := big.NewRat(1, 1)
		if i == 2 {
			w = big.NewRat(2, 1)
		}
		for j := 0; j < 3; j++ {
			v := tetraRat(m[j][3])
			for k := 0; k < 3; k++ {
				v.Add(v, new(big.Rat).Mul(tetraRat(m[j][k]), local[k]))
			}
			h[i][j] = new(big.Rat).Mul(v, w)
		}
		h[i][3] = new(big.Rat).Set(w)
	}
	return geom.NewExactHomogeneousBezier(ctx, h, o)
}

// ReadQuarterEllipseConic authenticates original full coefficient and placement
// ownership plus exact adjacent cardinal endpoint roles; no tolerance is used.
func ReadQuarterEllipseConic(r io.Reader, l Limits) (QuarterEllipseConic, error) {
	q := 0
	m, e := readEllipseConic(r, l, &q)
	if e != nil {
		return QuarterEllipseConic{}, e
	}
	return QuarterEllipseConic{m, q}, nil
}

// WriteQuarterEllipseConic stages and re-proves a single supported quarter arc.
func WriteQuarterEllipseConic(w io.Writer, q QuarterEllipseConic) error {
	if q.StartQuarter < 0 || q.StartQuarter > 3 {
		return geom.ErrInvalid
	}
	return writeEllipseConic(w, q.EllipseConicModel, q.StartQuarter)
}
