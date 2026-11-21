package boolean

import (
	"math"

	meet "plainsolid/brep/meeting"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// quartic is the carrier of a closed curve in which two cylinders with
// axes that are not parallel meet (specs/general-boolean-quartic). Its
// geometry is that of meet.Curve, by whose parameter it runs.
type quartic struct{ meet.Curve }

// meetingCurves is the row of two cylinders whose axes are not parallel
// and that are not an equal pair. A pair in scope gives its curves; any
// other is open, with the bound it fails as the reason.
func meetingCurves(a, b cylinder, t tolerances) pairing {
	found, reason := meet.Between(a.frame, a.radius, b.frame, b.radius, t.angular, t.eps)
	if len(found) == 0 {
		return pairing{open: true, reason: reason}
	}
	var out pairing
	for _, c := range found {
		out.curves, out.touching = append(out.curves, &quartic{c}), append(out.touching, false)
	}
	return out
}

func (q *quartic) family() family  { return quarticFamily }
func (q *quartic) period() float64 { return 2 * math.Pi }
func (q *quartic) flipped() curve  { return &quartic{q.Reversed()} }

func (q *quartic) at(t float64) geom.Vec3 {
	p, _ := q.State(t)
	return p
}

func (q *quartic) velocity(t float64) geom.Vec3 {
	_, v := q.State(t)
	return v
}

func (q *quartic) tangent(t float64) geom.Vec3 {
	v := q.velocity(t)
	return v.Scale(1 / v.Length())
}

func (q *quartic) param(p geom.Vec3) float64 { return q.Param(p) }

// turn gives the angle of the carrier point at parameter t about the axis
// of one of its two cylinders, continuous in t (H2). s may be either
// cylinder under another frame about the same axis.
func (q *quartic) turn(s cylinder, t float64) float64 { return q.Turn(s.frame, t) }

// box holds the part of the carrier from t0 to t1: its samples at steps
// of at most a 64th of a turn, grown by what the carrier can leave the
// chord between two of them, which is less than half the length it runs
// between them.
func (q *quartic) box(t0, t1 float64) bounds {
	n := int(math.Ceil(math.Abs(t1-t0)/(math.Pi/32))) + 1
	step := (t1 - t0) / float64(n)
	first, fast := q.State(t0)
	out, speed := around(first), fast.Length()
	for k := 1; k <= n; k++ {
		p, v := q.State(t0 + step*float64(k))
		out, speed = out.with(around(p)), math.Max(speed, v.Length())
	}
	return out.grown(speed * math.Abs(step))
}

// value gives what an edge on the carrier stores: the curve itself, by
// its own parameter.
func (q *quartic) value(_, _ geom.Vec3, t0, t1 float64) (geom.Curve, parametric.Interval, error) {
	return q.Curve, parametric.Interval{Min: t0, Max: t1}, nil
}
