package boolean

import (
	"math"
	"math/cmplx"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// The volume, the area and the centroid of a solid in the class have
// closed forms. By the divergence theorem each is a sum over the faces
// of a surface integral, and on a plane or a cylinder Green's theorem
// turns that into an integral along the boundary of the face in its
// chart. Along a line a chart coordinate is a polynomial of the
// parameter, and along a circle or an ellipse a trigonometric
// polynomial, so every integrand is a finite sum of terms tᵐ·e^{ikt},
// which has an antiderivative. Nothing here is a quadrature and nothing
// depends on what the faces belong to.

// series is a finite sum of terms c·tᵐ·e^{ikt}, with m below seriesPowers
// and k from −seriesWaves to seriesWaves. Its value is the real part.
type series struct {
	c    [seriesPowers][2*seriesWaves + 1]complex128
	lost bool // a product had a term outside the table
}

const (
	seriesPowers = 5
	seriesWaves  = 8
)

func constant(x float64) (s series) {
	s.c[0][seriesWaves] = complex(x, 0)
	return s
}

func (a series) plus(b series) series {
	for m := range a.c {
		for k := range a.c[m] {
			a.c[m][k] += b.c[m][k]
		}
	}
	a.lost = a.lost || b.lost
	return a
}

func (a series) scaled(x float64) series {
	for m := range a.c {
		for k := range a.c[m] {
			a.c[m][k] *= complex(x, 0)
		}
	}
	return a
}

func (a series) times(b series) series {
	out := series{lost: a.lost || b.lost}
	for m := range a.c {
		for k, x := range a.c[m] {
			if x == 0 {
				continue
			}
			for n := range b.c {
				for l, y := range b.c[n] {
					if y == 0 {
						continue
					}
					power, wave := m+n, k+l-seriesWaves
					if power >= seriesPowers || wave < 0 || wave > 2*seriesWaves {
						out.lost = true
						continue
					}
					out.c[power][wave] += x * y
				}
			}
		}
	}
	return out
}

// slope is the derivative by t.
func (a series) slope() series {
	out := series{lost: a.lost}
	for m := range a.c {
		for k, x := range a.c[m] {
			out.c[m][k] += x * complex(0, float64(k-seriesWaves))
			if m > 0 {
				out.c[m-1][k] += x * complex(float64(m), 0)
			}
		}
	}
	return out
}

// over is the integral from 0 to length. For a wave k other than zero,
// ∫ tᵐ·e^{ikt} dt is taken from that of tᵐ⁻¹ by parts.
func (a series) over(length float64) float64 {
	sum := 0.
	for k := range a.c[0] {
		wave := complex(0, float64(k-seriesWaves))
		var integral complex128
		for m := range a.c {
			top := complex(math.Pow(length, float64(m)), 0)
			switch {
			case wave == 0:
				integral = top * complex(length/float64(m+1), 0)
			case m == 0:
				integral = (cmplx.Exp(wave*complex(length, 0)) - 1) / wave
			default:
				integral = (top*cmplx.Exp(wave*complex(length, 0)) - complex(float64(m), 0)*integral) / wave
			}
			sum += real(a.c[m][k] * integral)
		}
	}
	return sum
}

// along gives the two chart coordinates of a path as series of its
// parameter counted from its start, and the length of that parameter,
// which is negative for a path walked against its carrier.
func (p path) along() (x, y series, length float64) {
	s0, c0 := math.Sincos(p.t0)
	coordinate := func(base, lin, cos, sin float64) series {
		// cos·cos(t₀+t) + sin·sin(t₀+t), as waves of t.
		c, s := cos*c0+sin*s0, sin*c0-cos*s0
		var out series
		out.c[0][seriesWaves] = complex(base+lin*p.t0, 0)
		out.c[1][seriesWaves] = complex(lin, 0)
		out.c[0][seriesWaves+1] = complex(c/2, -s/2)
		out.c[0][seriesWaves-1] = complex(c/2, s/2)
		return out
	}
	return coordinate(p.base.X, p.lin.X, p.cos.X, p.sin.X), coordinate(p.base.Y, p.lin.Y, p.cos.Y, p.sin.Y), p.t1 - p.t0
}

// moments are what one face adds to the measures of a solid whose
// outward normal on it is the carrier normal: its area, its share of the
// volume, and its share of the first moment ∫ x dV. ok is false when the
// closed form did not cover the face.
type moments struct {
	area, volume float64
	first        [3]float64
	ok           bool
}

func components(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }

// moments of a planar face. With chart coordinates x and y and the
// boundary walked with the face on its left, ∫∫ xᵃyᵇ dx dy is the
// boundary integral of xᵃ⁺¹yᵇ/(a+1) dy. The volume share is x·n/3
// integrated, and x·n is the same at every point of a plane. The first
// moment is the integral of xᵢ²nᵢ/2, and xᵢ is linear in the chart.
func (c planeChart) moments(loops [][]leg) moments {
	var m00, m10, m01, m20, m11, m02 float64
	out := moments{ok: true}
	for _, loop := range loops {
		for _, p := range chain(c, loop) {
			if p.free != nil {
				return moments{} // no carrier of the class lies in a plane without a form
			}
			x, y, length := p.along()
			dy := y.slope()
			take := func(s series) float64 {
				s = s.times(dy)
				out.ok = out.ok && !s.lost
				return s.over(length)
			}
			xx := x.times(x)
			m00 += take(x)
			m10 += take(xx.scaled(1. / 2))
			m01 += take(x.times(y))
			m20 += take(xx.times(x).scaled(1. / 3))
			m11 += take(xx.times(y).scaled(1. / 2))
			m02 += take(x.times(y).times(y))
		}
	}
	o, ax, ay, n := components(c.frame.Origin()), components(c.frame.XAxis()), components(c.frame.YAxis()), components(c.frame.ZAxis())
	out.area = m00
	out.volume = c.frame.Origin().Dot(c.frame.ZAxis()) * m00 / 3
	for i := range 3 {
		out.first[i] = n[i] / 2 * (o[i]*o[i]*m00 + 2*o[i]*(ax[i]*m10+ay[i]*m01) + ax[i]*ax[i]*m20 + 2*ax[i]*ay[i]*m11 + ay[i]*ay[i]*m02)
	}
	return out
}

// moments of a face on a cylinder. A point of it is o + v·z + r·e(θ),
// where o is on the axis, z is the axis and e turns about it, and the
// carrier normal is e. An integrand f(θ, v) is first integrated along v,
// which it is a polynomial of, to G; then ∫∫ f dθ dv is the boundary
// integral of −G dθ. A ruling has no dθ and adds nothing. Along a circle
// or an ellipse θ advances with the parameter and v is a trigonometric
// polynomial of it, so G is one too.
func (c cylinderChart) moments(loops [][]leg) moments {
	o, ax, ay, az := components(c.frame.Origin()), components(c.frame.XAxis()), components(c.frame.YAxis()), components(c.frame.ZAxis())
	r := c.radius
	out := moments{ok: true}
	for _, loop := range loops {
		for _, p := range chain(c, loop) {
			if p.free != nil {
				// M1 of specs/general-boolean-quartic: the same boundary
				// integrals by quadrature, with θ and v read off the
				// path. r·dθ is the step of the first chart coordinate.
				turn := math.Abs(p.at(p.t1).X-p.at(p.t0).X) / r
				p.gauss(p.pieces()+int(turn/(math.Pi/32)), func(q, d geom.Vec2, weight float64) {
					sine, cosine := math.Sincos(q.X / r)
					v, step := q.Y, -weight*d.X
					out.area += v * step
					out.volume += v * (c.frame.Origin().Dot(c.frame.XAxis())*cosine + c.frame.Origin().Dot(c.frame.YAxis())*sine + r) * step / 3
					for i := range 3 {
						e := ax[i]*cosine + ay[i]*sine
						a := o[i] + r*e
						out.first[i] += e * (a*a*v + a*az[i]*v*v + az[i]*az[i]*v*v*v/3) * step / 2
					}
				})
				continue
			}
			_, v, length := p.along()
			rate := p.lin.X / r
			if rate == 0 {
				continue
			}
			if rate != 1 && rate != -1 {
				return moments{} // θ does not advance with the parameter
			}
			// cos θ and sin θ, with θ = angle + rate·t.
			turn := cmplx.Exp(complex(0, (p.base.X+p.lin.X*p.t0)/r))
			var cosine, sine series
			w := int(rate)
			cosine.c[0][seriesWaves+w], cosine.c[0][seriesWaves-w] = turn/2, cmplx.Conj(turn)/2
			sine.c[0][seriesWaves+w], sine.c[0][seriesWaves-w] = turn/complex(0, 2), -cmplx.Conj(turn)/complex(0, 2)
			take := func(g series) float64 {
				out.ok = out.ok && !g.lost
				return -rate * r * g.over(length)
			}
			out.area += take(v)
			// x·e = o·e + r.
			reach := cosine.scaled(c.frame.Origin().Dot(c.frame.XAxis())).plus(sine.scaled(c.frame.Origin().Dot(c.frame.YAxis()))).plus(constant(r))
			out.volume += take(v.times(reach)) / 3
			vv := v.times(v)
			vvv := vv.times(v)
			for i := range 3 {
				// xᵢ = aᵢ + zᵢ·v with aᵢ = oᵢ + r·eᵢ, and
				// ∫ xᵢ² dv = aᵢ²v + aᵢzᵢv² + zᵢ²v³/3.
				e := cosine.scaled(ax[i]).plus(sine.scaled(ay[i]))
				a := e.scaled(r).plus(constant(o[i]))
				along := a.times(a).times(v).plus(a.times(vv).scaled(az[i])).plus(vvv.scaled(az[i] * az[i] / 3))
				out.first[i] += take(e.times(along)) / 2
			}
		}
	}
	return out
}

// measured sums the moments of n faces into the volume, the area and the
// centroid of the solid they bound. face gives each one's carrier,
// whether its outward normal is against the carrier normal, and its
// boundary. ok is false when a face is outside what the closed form
// covers; the caller then integrates numerically.
func measured(n int, face func(i int) (surface, bool, [][]leg)) (parametric.Properties, bool) {
	var p parametric.Properties
	var first [3]float64
	for i := 0; i < n; i++ {
		s, reversed, loops := face(i)
		m := s.chart().moments(loops)
		if !m.ok {
			return parametric.Properties{}, false
		}
		if reversed {
			m.volume, m.first = -m.volume, [3]float64{-m.first[0], -m.first[1], -m.first[2]}
		}
		p.Area += m.area
		p.Volume += m.volume
		for k := range first {
			first[k] += m.first[k]
		}
	}
	if p.Volume != 0 {
		p.Centroid = geom.Vec3{X: first[0] / p.Volume, Y: first[1] / p.Volume, Z: first[2] / p.Volume}
	}
	return p, true
}
