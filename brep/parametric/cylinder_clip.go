package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// ClipCylinder keeps the part of a finite cylinder on the negative side of
// plane. It supports cuts crossing every generator between the two end caps,
// including horizontal cuts, and cuts entirely outside. Side-wall-only cuts
// and cuts crossing an end-cap rim are outside this operation's domain.
func ClipCylinder(frame geom.Frame, radius, height float64, plane geom.Plane, tolerance float64) (Solid, error) {
	cyl, err := geom.NewCylinderSurface(frame, radius)
	if err != nil {
		return Solid{}, err
	}
	n := plane.Normal()
	if height <= 0 || !finite(height) || tolerance <= 0 || !finite(tolerance) || n.Length() == 0 {
		return Solid{}, geom.ErrInvalid
	}
	z := n.Dot(frame.ZAxis())
	x, y := n.Dot(frame.XAxis()), n.Dot(frame.YAxis())
	spread := radius * math.Hypot(x, y)
	d := plane.SignedDistance(frame.Origin())
	if !finite(d) || !finite(spread) {
		return Solid{}, geom.ErrInvalid
	}
	minD, maxD := d+math.Min(0, z*height)-spread, d+math.Max(0, z*height)+spread
	if !finite(minD) || !finite(maxD) {
		return Solid{}, geom.ErrInvalid
	}
	if maxD < -tolerance {
		return Cylinder(frame, radius, height)
	}
	if minD > tolerance {
		return Solid{}, nil
	}
	if math.Abs(z) <= 1e-12 {
		return Solid{}, fmt.Errorf("parametric: cut parallel to cylinder axis")
	}
	lo, hi := -d/z-spread/math.Abs(z), -d/z+spread/math.Abs(z)
	if !finite(lo) || !finite(hi) || lo <= tolerance || hi >= height-tolerance {
		return Solid{}, fmt.Errorf("parametric: cylinder cut reaches an end-cap rim")
	}
	r := slantedRim{cyl: cyl, center: -d / z, cosine: -radius * x / z, sine: -radius * y / z}
	var side cutCylinder
	if z > 0 {
		side = cutCylinder{r, 0, true}
	} else {
		side = cutCylinder{r, height, false}
	}
	cutCenter := frame.ToWorld(geom.Vec3{Z: r.center})
	capCenter := frame.Origin()
	capHeight := 0.
	if z < 0 {
		capHeight = height
		capCenter = frame.ToWorld(geom.Vec3{Z: height})
	}
	var faces []Face
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		faces = append(faces,
			Face{Surface: side, U: u, V: normalized},
			Face{Surface: slantedFan{r, cutCenter}, U: u, V: normalized, Reversed: z > 0},
			Face{Surface: arcFan{cyl, capCenter, capHeight}, U: u, V: normalized, Reversed: z < 0})
	}
	return joinPrimitivePatches(faces, tolerance)
}

type slantedRim struct {
	cyl                  geom.CylinderSurface
	center, cosine, sine float64
}

func (r slantedRim) height(u float64) (float64, float64) {
	c, s := math.Cos(u), math.Sin(u)
	return r.center + r.cosine*c + r.sine*s, -r.cosine*s + r.sine*c
}

type cutCylinder struct {
	rim      slantedRim
	constant float64
	cutTop   bool
}

func (s cutCylinder) coords(u, v float64) (float64, float64, float64, error) {
	if !finite(u) || !finite(v) || v < 0 || v > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	z, d := s.rim.height(u)
	if s.cutTop {
		return (1-v)*s.constant + v*z, v * d, z - s.constant, nil
	}
	return (1-v)*z + v*s.constant, (1 - v) * d, s.constant - z, nil
}
func (s cutCylinder) Eval(u, v float64) (geom.Vec3, error) {
	z, _, _, e := s.coords(u, v)
	if e != nil {
		return geom.Vec3{}, e
	}
	return s.rim.cyl.Eval(u, z)
}
func (s cutCylinder) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	z, du, dv, e := s.coords(u, v)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	a, b, e := s.rim.cyl.Partials(u, z)
	return a.Add(b.Scale(du)), b.Scale(dv), e
}

type slantedFan struct {
	rim    slantedRim
	center geom.Vec3
}

func (f slantedFan) Eval(u, v float64) (geom.Vec3, error) {
	if !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	z, _ := f.rim.height(u)
	p, e := f.rim.cyl.Eval(u, z)
	if e != nil {
		return p, e
	}
	return checked(f.center.Scale(1 - v).Add(p.Scale(v)))
}
func (f slantedFan) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	z, d := f.rim.height(u)
	p, e := f.rim.cyl.Eval(u, z)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	a, b, e := f.rim.cyl.Partials(u, z)
	return a.Add(b.Scale(d)).Scale(v), p.Sub(f.center), e
}
