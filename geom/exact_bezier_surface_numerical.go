package geom

import "math/big"

// NumericalSurface provides an explicitly approximate low-degree homogeneous
// evaluator for numerical quadrature. The exact support remains unchanged and
// is returned for highly scaled or higher degree charts. This adapter must not
// be used for geometry certification or lossless spline export.
func (s *ExactBezierSurface) NumericalSurface() Surface {
	if s == nil || s.patch == nil {
		return nil
	}
	p := s.patch
	if p.degreeU > 3 || p.degreeV > 3 {
		return s
	}
	d := p.Domain()
	spanU, spanV := d.UMax-d.UMin, d.VMax-d.VMin
	if spanU < 1e-80 || spanV < 1e-80 || spanU > 1e80 || spanV > 1e80 {
		return s
	}
	q := &approximateHomogeneousBezier{source: s, domain: d, du: p.degreeU, dv: p.degreeV}
	maxWeight := p.net[0][3]
	for _, h := range p.net[1:] {
		if h[3].Cmp(maxWeight) > 0 {
			maxWeight = h[3]
		}
	}
	for i := 0; i <= p.degreeU; i++ {
		for j := 0; j <= p.degreeV; j++ {
			h := p.net[i*(p.degreeV+1)+j]
			for k := 0; k < 4; k++ {
				v := new(big.Rat).Quo(h[k], maxWeight)
				f, _ := v.Float64()
				if !finite(f) || f == 0 && v.Sign() != 0 || f < -1e80 || f > 1e80 || k == 3 && f < 1e-8 {
					return s
				}
				q.net[i][j][k] = f
			}
		}
	}
	return q
}

type approximateHomogeneousBezier struct {
	source *ExactBezierSurface
	domain ParameterRectangle
	du, dv int
	net    [4][4][4]float64
}

func approximateBernstein(d int, x float64) [4]float64 {
	y := 1 - x
	switch d {
	case 0:
		return [4]float64{1}
	case 1:
		return [4]float64{y, x}
	case 2:
		return [4]float64{y * y, 2 * x * y, x * x}
	default:
		return [4]float64{y * y * y, 3 * x * y * y, 3 * x * x * y, x * x * x}
	}
}

func (s *approximateHomogeneousBezier) homogeneous(u, v float64, partials bool) (point, uJet, vJet [4]float64, err error) {
	d := s.domain
	if !finite(u) || !finite(v) || u < d.UMin || u > d.UMax || v < d.VMin || v > d.VMax {
		return point, uJet, vJet, ErrDomain
	}
	x, y := (u-d.UMin)/(d.UMax-d.UMin), (v-d.VMin)/(d.VMax-d.VMin)
	a, b := approximateBernstein(s.du, x), approximateBernstein(s.dv, y)
	var da, db [4]float64
	if partials {
		da, db = approximateBernstein(s.du-1, x), approximateBernstein(s.dv-1, y)
	}
	for i := 0; i <= s.du; i++ {
		for j := 0; j <= s.dv; j++ {
			c := a[i] * b[j]
			for k, h := range s.net[i][j] {
				point[k] += c * h
				if partials && i < s.du {
					uJet[k] += da[i] * b[j] * (s.net[i+1][j][k] - h)
				}
				if partials && j < s.dv {
					vJet[k] += a[i] * db[j] * (s.net[i][j+1][k] - h)
				}
			}
		}
	}
	return point, uJet, vJet, nil
}

func (s *approximateHomogeneousBezier) Eval(u, v float64) (Vec3, error) {
	h, _, _, err := s.homogeneous(u, v, false)
	if err != nil {
		return s.source.Eval(u, v)
	}
	p := Vec3{h[0] / h[3], h[1] / h[3], h[2] / h[3]}
	if !p.IsFinite() {
		return s.source.Eval(u, v)
	}
	return p, nil
}

func (s *approximateHomogeneousBezier) Partials(u, v float64) (Vec3, Vec3, error) {
	h, a, b, err := s.homogeneous(u, v, true)
	if err != nil {
		return s.source.Partials(u, v)
	}
	derivative := func(q [4]float64, scale float64) Vec3 {
		return Vec3{
			(q[0] - (h[0]/h[3])*q[3]) / h[3] * scale,
			(q[1] - (h[1]/h[3])*q[3]) / h[3] * scale,
			(q[2] - (h[2]/h[3])*q[3]) / h[3] * scale,
		}
	}
	d := s.domain
	dx, dy := derivative(a, float64(s.du)/(d.UMax-d.UMin)), derivative(b, float64(s.dv)/(d.VMax-d.VMin))
	if !dx.IsFinite() || !dy.IsFinite() {
		return s.source.Partials(u, v)
	}
	return dx, dy, nil
}
