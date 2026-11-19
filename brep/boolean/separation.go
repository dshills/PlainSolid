package boolean

import (
	"math"

	"plainsolid/geom"
)

// clearOf is a proof of separation under SP2 for a face of one cylinder
// and another cylinder whose axis is not parallel to its own: every point
// of the face is farther from the other axis than the other radius, or
// every point is nearer, by more than twice the filter margin (T4). A
// face that is clear of the whole of the other cylinder meets no face of
// it.
//
// The distance to the other axis is least on the face either on an edge
// or at a point of the face's cylinder where it does not change along the
// cylinder's own axis and is least or zero round it; there are at most
// four such points (nearestToAxis). It is greatest on an edge, because
// along the cylinder's own axis its square is a quadratic that opens
// upwards.
func (a *arrangement) clearOf(op, f int, other cylinder) bool {
	face := a.ops[op].faces[f]
	own := face.s.(cylinder)
	gap := 2 * a.t.margin
	outside, inside := square(other.radius+gap), square(math.Max(other.radius-gap, 0))
	out, in := true, other.radius > gap
	for _, loop := range face.loops {
		for _, u := range loop {
			ed := a.ops[op].edges[u.edge]
			switch c := ed.c.(type) {
			case conic:
				g := axisDistance(c, other)
				out, in = out && g.above(ed.t0, ed.t1, outside), in && g.below(ed.t0, ed.t1, inside)
			case line:
				// The square of the distance along a line is a quadratic
				// in its parameter: greatest at an end, least at an end
				// or where it turns.
				from, along := other.across(c.origin.Sub(other.frame.Origin())), other.across(c.dir)
				at := func(t float64) float64 { return from.Add(along.Scale(t)).Dot(from.Add(along.Scale(t))) }
				least, most := math.Min(at(ed.t0), at(ed.t1)), math.Max(at(ed.t0), at(ed.t1))
				if k := along.Dot(along); k > 0 {
					if t := -from.Dot(along) / k; t > ed.t0 && t < ed.t1 {
						least = math.Min(least, at(t))
					}
				}
				out, in = out && least > outside, in && most < inside
			default:
				return false
			}
			if !out && !in {
				return false
			}
		}
	}
	if in {
		return true
	}
	for _, p := range nearestToAxis(own, other) {
		if other.offset(p) <= gap && a.inFace(op, f, p, a.t.margin) != without {
			return false
		}
	}
	return out
}

func square(x float64) float64 { return x * x }

// nearestToAxis gives the points of a cylinder at which the distance to
// the axis of another, not parallel to its own, can be least over a part
// of its surface without being so on the boundary of that part. Along a
// ruling the distance is least at one point. From ruling to ruling that
// least distance is |d + r·cos(θ − θ₀)|, where d is the signed distance
// between the axes and θ₀ the direction of their common perpendicular: it
// is least or greatest at θ₀ and opposite it, and zero where the ruling
// meets the other axis.
func nearestToAxis(own, other cylinder) []geom.Vec3 {
	z, w := own.frame.ZAxis(), other.frame.ZAxis()
	n, err := z.Cross(w).Unit()
	if err != nil {
		return nil
	}
	d := own.frame.Origin().Sub(other.frame.Origin()).Dot(n)
	alpha, beta := own.frame.XAxis().Dot(n), own.frame.YAxis().Dot(n)
	towards := math.Atan2(beta, alpha)
	angles := []float64{towards, towards + math.Pi}
	if k := -d / own.radius; math.Abs(k) <= 1 {
		angles = append(angles, towards+math.Acos(k), towards-math.Acos(k))
	}
	along := other.across(z)
	var out []geom.Vec3
	for _, angle := range angles {
		s, c := math.Sincos(angle)
		foot := own.frame.Origin().Add(own.frame.XAxis().Scale(own.radius * c)).Add(own.frame.YAxis().Scale(own.radius * s))
		v := -other.across(foot.Sub(other.frame.Origin())).Dot(along) / along.Dot(along)
		out = append(out, foot.Add(z.Scale(v)))
	}
	return out
}
