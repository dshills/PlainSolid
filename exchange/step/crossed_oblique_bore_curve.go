package step

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func crossedObliqueBoreEdge(a, b geom.Vec3, owners []longEdgeOwner, surfaces []longEdgeSurface) (longEdgeCurve, error) {
	fail := func() (longEdgeCurve, error) { return longEdgeCurve{}, geom.ErrDomain }
	var out longEdgeCurve
	var sharedCos, sharedSin geom.Vec3
	first := surfaces[owners[0].face]
	r := first.radius
	second := surfaces[owners[1].face]
	dot := math.Abs(first.frame.ZAxis().Dot(second.frame.ZAxis()))
	reciprocal := math.Abs(dot-.8) <= 8*0x1p-52
	if !reciprocal && math.Abs(dot-.6) > 8*0x1p-52 {
		return fail()
	}
	positive, negative := 2*r, -.5*r
	if reciprocal {
		positive, negative = 3*r, (-1./3)*r
	}
	tol := 1e-9 + 128*0x1p-52*max(a.Length(), b.Length(), r)
	if !finiteOblique(tol) || tol >= r/16 {
		return fail()
	}
	for i, owner := range owners {
		cyl, u := surfaces[owner.face], owner.use
		if cyl.radius != r || cyl.frame.Origin() != first.frame.Origin() {
			return fail()
		}
		curve, ok := u.Curve.(interface{ EllipseParameters() trimmed.EllipseUV })
		if !ok {
			return fail()
		}
		c := curve.EllipseParameters()
		if c.Center != (geom.Vec2{}) || c.Linear != (geom.Vec2{X: 1}) || c.Sin != (geom.Vec2{}) ||
			c.Cos.X != 0 || math.Abs(c.To-c.From) != math.Pi/2 ||
			(c.Cos.Y != positive && c.Cos.Y != negative) {
			return fail()
		}
		center := cyl.frame.Origin()
		cv := cyl.frame.XAxis().Scale(r).Add(cyl.frame.ZAxis().Scale(c.Cos.Y))
		sv := cyl.frame.YAxis().Scale(r)
		major := r * math.Sqrt(5)
		if c.Cos.Y < 0 {
			major *= .5
		}
		if reciprocal {
			major = r * math.Sqrt(10)
			if c.Cos.Y < 0 {
				major /= 3
			}
		}
		from, to := c.From, c.To
		if u.Reversed {
			from, to = to, from
		}
		point := func(angle float64) geom.Vec3 {
			return center.Add(cv.Scale(math.Cos(angle))).Add(sv.Scale(math.Sin(angle)))
		}
		if point(from).Distance(a) > tol || point(to).Distance(b) > tol || math.Abs(cv.Length()-major) > tol ||
			math.Abs(sv.Length()-r) > tol || math.Abs(cv.Dot(sv)) > tol*r {
			return fail()
		}
		if i == 0 {
			f, err := geom.NewFrame(center, cv, cv.Cross(sv))
			if err != nil {
				return longEdgeCurve{}, err
			}
			out = longEdgeCurve{ellipse: true, frame: f, radius: major, minorRadius: r, same: to > from}
			sharedCos, sharedSin = cv, sv
		} else if min(cv.Distance(sharedCos), cv.Distance(sharedCos.Scale(-1)))+
			min(sv.Distance(sharedSin), sv.Distance(sharedSin.Scale(-1))) > tol {
			return fail()
		}
	}
	return out, nil
}
