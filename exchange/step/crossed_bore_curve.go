package step

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// crossedBoreEdgeCurve is private to freshly reconstructed BoxCrossedBores
// topology. Its canonical cylinder trims are v=+/-r*cos(u), in quarter spans.
// Arbitrary user trim curves must not be passed through this recipe writer.
// The resulting analytic 3D ellipse deliberately has no approximate p-curve;
// independent STEP readers project it onto its two authoritative cylinders.
func crossedBoreEdgeCurve(a, b geom.Vec3, owners []longEdgeOwner, surfaces []longEdgeSurface) (longEdgeCurve, error) {
	fail := func() (longEdgeCurve, error) { return longEdgeCurve{}, geom.ErrDomain }
	if len(owners) != 2 {
		return fail()
	}
	first, second := surfaces[owners[0].face], surfaces[owners[1].face]
	if first.kind != "CYLINDRICAL_SURFACE" || second.kind != first.kind ||
		first.radius != second.radius || first.radius <= 0 || first.frame.Origin() != second.frame.Origin() ||
		first.frame.ZAxis().Dot(second.frame.ZAxis()) != 0 {
		return fail()
	}
	var result longEdgeCurve
	var firstCos, firstSin geom.Vec3
	for i, owner := range owners {
		surface, u := surfaces[owner.face], owner.use
		if u.Curve == nil {
			return fail()
		}
		start, end := u.UVStart, u.UVEnd
		if u.Reversed {
			start, end = end, start
		}
		q0, ok0 := crossedBoreQuarter(start.X)
		q1, ok1 := crossedBoreQuarter(end.X)
		if !ok0 || !ok1 || math.Abs(end.X-start.X) != math.Pi/2 {
			return splitCrossedBoreEdgeCurve(a, b, owners, surfaces)
		}
		cos := [4]float64{1, 0, -1, 0}
		sin := [4]float64{0, 1, 0, -1}
		coefficient := start.Y
		if cos[q0] != 0 {
			coefficient /= cos[q0]
		} else {
			coefficient = end.Y / cos[q1]
		}
		if math.Abs(coefficient) != surface.radius || start.Y != coefficient*cos[q0] || end.Y != coefficient*cos[q1] {
			return fail()
		}
		cv := surface.frame.XAxis().Scale(surface.radius).Add(surface.frame.ZAxis().Scale(coefficient))
		sv := surface.frame.YAxis().Scale(surface.radius)
		center := surface.frame.Origin()
		if center.Add(cv.Scale(cos[q0])).Add(sv.Scale(sin[q0])) != a ||
			center.Add(cv.Scale(cos[q1])).Add(sv.Scale(sin[q1])) != b {
			return fail()
		}
		if i == 0 {
			frame, err := geom.NewFrame(center, cv, cv.Cross(sv))
			if err != nil {
				return longEdgeCurve{}, err
			}
			result = longEdgeCurve{ellipse: true, frame: frame, radius: math.Sqrt2 * surface.radius, minorRadius: surface.radius, same: end.X > start.X}
			firstCos, firstSin = cv, sv
		} else {
			// Both charts must describe the same principal axes. The endpoints
			// above select the same quarter rather than its complementary arc.
			if cv != firstCos && cv != firstCos.Scale(-1) {
				return fail()
			}
			if sv != firstSin && sv != firstSin.Scale(-1) {
				return fail()
			}
		}
	}
	return result, nil
}

func crossedBoreQuarter(angle float64) (int, bool) {
	if math.IsNaN(angle) || math.IsInf(angle, 0) || angle < 0 || angle > 2*math.Pi {
		return 0, false
	}
	q := int(math.Round(angle / (math.Pi / 2)))
	return q % 4, angle == float64(q)*(math.Pi/2)
}

// The clipped floor splits some of the existing saddle ellipses. The
// restricted chart still describes the same exact conic on both owners.
func splitCrossedBoreEdgeCurve(a, b geom.Vec3, owners []longEdgeOwner, surfaces []longEdgeSurface) (longEdgeCurve, error) {
	var result longEdgeCurve
	var firstCos, firstSin geom.Vec3
	for i, owner := range owners {
		surface, u := surfaces[owner.face], owner.use
		p, ok := u.Curve.(interface{ EllipseParameters() trimmed.EllipseUV })
		if !ok {
			return longEdgeCurve{}, geom.ErrDomain
		}
		ellipse := p.EllipseParameters()
		if ellipse.Center != (geom.Vec2{}) || ellipse.Linear != (geom.Vec2{X: 1}) ||
			ellipse.Sin != (geom.Vec2{}) || ellipse.Cos.X != 0 ||
			math.Abs(ellipse.Cos.Y) != surface.radius ||
			ellipse.From != u.UVStart.X || ellipse.To != u.UVEnd.X ||
			math.Abs(ellipse.To-ellipse.From) >= math.Pi/2 {
			return longEdgeCurve{}, geom.ErrDomain
		}
		cv := surface.frame.XAxis().Scale(surface.radius).Add(surface.frame.ZAxis().Scale(ellipse.Cos.Y))
		sv := surface.frame.YAxis().Scale(surface.radius)
		center := surface.frame.Origin()
		from, to := ellipse.From, ellipse.To
		if u.Reversed {
			from, to = to, from
		}
		point := func(angle float64) geom.Vec3 {
			return center.Add(cv.Scale(math.Cos(angle))).Add(sv.Scale(math.Sin(angle)))
		}
		if point(from).Distance(a) > 1e-9 || point(to).Distance(b) > 1e-9 {
			return longEdgeCurve{}, geom.ErrDomain
		}
		if i == 0 {
			frame, err := geom.NewFrame(center, cv, cv.Cross(sv))
			if err != nil {
				return longEdgeCurve{}, err
			}
			result = longEdgeCurve{ellipse: true, frame: frame, radius: math.Sqrt2 * surface.radius, minorRadius: surface.radius, same: to > from}
			firstCos, firstSin = cv, sv
		} else if (cv != firstCos && cv != firstCos.Scale(-1)) || (sv != firstSin && sv != firstSin.Scale(-1)) {
			return longEdgeCurve{}, geom.ErrDomain
		}
	}
	return result, nil
}
