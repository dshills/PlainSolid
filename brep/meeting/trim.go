package meeting

import (
	"math"
	"sort"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// Trim is the p-curve of an arc of a Curve on one of its two cylinders,
// in the parameters of a cylinder surface with a given frame: the angle
// about the z axis from the x axis, and the height along the z axis. Its
// own parameter runs from 0 at one end of the arc to 1 at the other.
type Trim struct {
	c          Curve
	frame      geom.Frame
	t0, t1     float64
	shift      float64 // whole turns, added to the angle Turn gives
	start, end geom.Vec2
	pinned     bool
	turns      []float64
}

var _ trimmed.MonotoneUV = Trim{}

// Trim gives the p-curve of the arc from parameter t0 to t1, which may be
// the lesser, on the cylinder about the z axis of the frame. Its angle is
// the one Turn gives, moved by whole turns so that the arc starts within
// half a turn of near.
func (c Curve) Trim(f geom.Frame, t0, t1, near float64) Trim {
	out := Trim{c: c, frame: f, t0: t0, t1: t1}
	out.shift = 2 * math.Pi * math.Round((near-c.Turn(f, t0))/(2*math.Pi))
	// Where either coordinate turns back: where a component of the
	// derivative changes sign between two of 16 samples, placed by
	// bisection. A change within a billionth of an end is the rounding
	// of a derivative that is zero there.
	const parts, edge = 16, 1e-9
	for axis := range 2 {
		speed := func(u float64) float64 {
			d := out.rate(u)
			return [2]float64{d.X, d.Y}[axis]
		}
		from, before := 0., speed(0)
		for k := 1; k <= parts; k++ {
			to := float64(k) / parts
			after := speed(to)
			if before != 0 && after != 0 && (before > 0) != (after > 0) {
				lo, hi := from, to
				for range 60 {
					mid := lo + (hi-lo)/2
					if s := speed(mid); s != 0 && (s > 0) == (before > 0) {
						lo = mid
					} else {
						hi = mid
					}
				}
				if u := lo + (hi-lo)/2; u > edge && u < 1-edge {
					out.turns = append(out.turns, u)
				}
			}
			if after != 0 {
				from, before = to, after
			}
		}
	}
	sort.Float64s(out.turns)
	return out
}

// Pinned is the same p-curve, giving exactly these points at 0 and at 1:
// the ends its use declares, which its own evaluation meets only to
// within rounding.
func (c Trim) Pinned(start, end geom.Vec2) Trim {
	c.start, c.end, c.pinned = start, end, true
	return c
}

func (c Trim) EvalUV(u float64) (geom.Vec2, error) {
	if c.pinned {
		switch u {
		case 0:
			return c.start, nil
		case 1:
			return c.end, nil
		}
	}
	t := c.t0 + (c.t1-c.t0)*u
	p, _ := c.c.State(t)
	return geom.Vec2{X: c.c.Turn(c.frame, t) + c.shift, Y: p.Sub(c.frame.Origin()).Dot(c.frame.ZAxis())}, nil
}

// rate is the derivative of the p-curve by its own parameter.
func (c Trim) rate(u float64) geom.Vec2 {
	p, v := c.c.State(c.t0 + (c.t1-c.t0)*u)
	d := p.Sub(c.frame.Origin())
	x, y := d.Dot(c.frame.XAxis()), d.Dot(c.frame.YAxis())
	round := (x*v.Dot(c.frame.YAxis()) - y*v.Dot(c.frame.XAxis())) / (x*x + y*y)
	return geom.Vec2{X: round * (c.t1 - c.t0), Y: v.Dot(c.frame.ZAxis()) * (c.t1 - c.t0)}
}

// DerivativeUV is what trimmed.Solid.MassProperties integrates along.
func (c Trim) DerivativeUV(u float64) (geom.Vec2, error) { return c.rate(u), nil }

// TurnsUV lists where either coordinate turns back (trimmed.MonotoneUV).
func (c Trim) TurnsUV() []float64 { return c.turns }

// BoundsUV holds the curve: between two of its turns it is monotone in
// both coordinates, so it reaches no farther than its values at them and
// at its ends.
func (c Trim) BoundsUV() (geom.UVBox, error) {
	first, _ := c.EvalUV(0)
	box := geom.UVBox{Min: first, Max: first}
	for _, u := range append(append([]float64{}, c.turns...), 1) {
		q, _ := c.EvalUV(u)
		box.Min = geom.Vec2{X: math.Min(box.Min.X, q.X), Y: math.Min(box.Min.Y, q.Y)}
		box.Max = geom.Vec2{X: math.Max(box.Max.X, q.X), Y: math.Max(box.Max.Y, q.Y)}
	}
	return box, nil
}
