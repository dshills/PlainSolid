package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// EllipticStation fixes the elliptical radii at an axial distance from the
// frame origin. Stations have a common axis and angular alignment.
type EllipticStation struct {
	Height float64
	Radii  geom.Vec2
}

// LoftEllipticStations joins two or more coaxial elliptical sections. The
// two radii are independently interpolated by slope-limited cubic Hermite
// polynomials, preserving positive radii and C1 axial continuity. Exact
// elliptical cross sections and two planar caps are retained.
func LoftEllipticStations(frame geom.Frame, stations []EllipticStation, tolerance float64) (Solid, error) {
	if len(stations) < 2 || len(stations) > 128 || !finite(tolerance) || tolerance <= 0 || frame.ZAxis().Length() == 0 {
		return Solid{}, geom.ErrInvalid
	}
	x, y, z := make([]float64, len(stations)), make([]float64, len(stations)), make([]float64, len(stations))
	for i, p := range stations {
		if !finite(p.Height) || !finite(p.Radii.X) || !finite(p.Radii.Y) ||
			p.Radii.X <= 8*tolerance || p.Radii.Y <= 8*tolerance || i > 0 && (p.Height-stations[i-1].Height <= 8*tolerance || !finite(p.Height-stations[i-1].Height)) {
			return Solid{}, geom.ErrInvalid
		}
		x[i], y[i], z[i] = p.Radii.X, p.Radii.Y, p.Height
	}
	tangents := func(r []float64) ([]float64, error) {
		n := len(r)
		d, m := make([]float64, n-1), make([]float64, n)
		for i := range d {
			d[i] = (r[i+1] - r[i]) / (z[i+1] - z[i])
			if !finite(d[i]) {
				return nil, geom.ErrInvalid
			}
		}
		m[0], m[n-1] = d[0], d[n-2]
		for i := 1; i+1 < n; i++ {
			a, b := d[i-1], d[i]
			if a == 0 || b == 0 || math.Signbit(a) != math.Signbit(b) {
				continue
			}
			m[i] = math.Copysign(min(math.Abs(a), math.Abs(b)), a)
		}
		return m, nil
	}
	sx, e := tangents(x)
	if e != nil {
		return Solid{}, e
	}
	sy, e := tangents(y)
	if e != nil {
		return Solid{}, e
	}
	var faces []Face
	for i := 0; i+1 < len(z); i++ {
		h := z[i+1] - z[i]
		s := ellipticBand{frame: frame, z: z[i], h: h,
			x: [4]float64{x[i], x[i] + sx[i]*(h/3), x[i+1] - sx[i+1]*(h/3), x[i+1]},
			y: [4]float64{y[i], y[i] + sy[i]*(h/3), y[i+1] - sy[i+1]*(h/3), y[i+1]},
		}
		if !s.valid() {
			return Solid{}, fmt.Errorf("parametric: invalid station loft controls")
		}
		for j := range 4 {
			u := Interval{Min: float64(j) * math.Pi / 2, Max: float64(j+1) * math.Pi / 2}
			faces = append(faces, Face{Surface: s, U: u, V: normalized})
		}
	}
	for _, i := range []int{0, len(z) - 1} {
		o := frame.ToWorld(geom.Vec3{Z: z[i]})
		f, e := geom.NewFrame(o, frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		cap := ellipticPatch{frame: f, upper: geom.Vec2{X: x[i], Y: y[i]}}
		for j := range 4 {
			u := Interval{Min: float64(j) * math.Pi / 2, Max: float64(j+1) * math.Pi / 2}
			faces = append(faces, Face{Surface: cap, U: u, V: normalized, Reversed: i != 0})
		}
	}
	return joinPrimitivePatches(faces, tolerance)
}

type ellipticBand struct {
	frame geom.Frame
	z, h  float64
	x, y  [4]float64
}

func (s ellipticBand) valid() bool {
	if !finite(s.z) || !finite(s.h) || s.h <= 0 || !finite(s.z+s.h) || s.frame.ZAxis().Length() == 0 {
		return false
	}
	for _, r := range [2][4]float64{s.x, s.y} {
		for _, v := range r {
			if !finite(v) || v <= 0 {
				return false
			}
		}
	}
	return true
}
func blend4(p [4]float64, t float64) (float64, float64) {
	a := (1-t)*p[0] + t*p[1]
	b := (1-t)*p[1] + t*p[2]
	c := (1-t)*p[2] + t*p[3]
	x, y := (1-t)*a+t*b, (1-t)*b+t*c
	return (1-t)*x + t*y, 3 * (y - x)
}
func (s ellipticBand) Eval(u, v float64) (geom.Vec3, error) {
	if !finite(u) || !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	x, _ := blend4(s.x, v)
	y, _ := blend4(s.y, v)
	si, c := math.Sincos(u)
	return checked(s.frame.ToWorld(geom.Vec3{X: x * c, Y: y * si, Z: s.z + s.h*v}))
}
func (s ellipticBand) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	x, dx := blend4(s.x, v)
	y, dy := blend4(s.y, v)
	si, c := math.Sincos(u)
	a := s.frame.XAxis().Scale(-x * si).Add(s.frame.YAxis().Scale(y * c))
	b := s.frame.XAxis().Scale(dx * c).Add(s.frame.YAxis().Scale(dy * si)).Add(s.frame.ZAxis().Scale(s.h))
	if !a.IsFinite() || !b.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return a, b, nil
}
