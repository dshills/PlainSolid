package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// CylinderSector constructs an exact solid spanning angle radians from the
// frame's positive X axis toward Y. Angle is strictly between zero and 2π.
// The boundary has one cylindrical wall, two radial planes and two polar caps.
func CylinderSector(frame geom.Frame, radius, height, angle float64) (Solid, error) {
	return buildRadialSector(frame, radius, radius, height, angle)
}

// ConicalSector constructs the exact partial sweep of a narrowing frustum
// with positive base and top radii. Its sides are two planar trapezoids.
func ConicalSector(frame geom.Frame, baseRadius, topRadius, height, angle float64) (Solid, error) {
	if !finite(baseRadius) || !finite(topRadius) || topRadius <= 0 || baseRadius <= topRadius {
		return Solid{}, geom.ErrInvalid
	}
	return buildRadialSector(frame, baseRadius, topRadius, height, angle)
}

func buildRadialSector(frame geom.Frame, radius, topRadius, height, angle float64) (Solid, error) {
	if !finite(radius) || radius <= 0 || !finite(height) || height <= 0 || !finite(angle) || angle <= 0 || angle >= 2*math.Pi {
		return Solid{}, geom.ErrInvalid
	}
	var wallSurface geom.Surface
	var e error
	axial := Interval{Max: height}
	conical := radius != topRadius
	if radius == topRadius {
		wallSurface, e = geom.NewCylinderSurface(frame, radius)
	} else {
		slope := (radius - topRadius) / height
		apex := frame.ToWorld(geom.Vec3{Z: radius / slope})
		axisFrame, err := geom.NewFrame(frame.Origin(), frame.XAxis(), frame.ZAxis())
		if err != nil {
			return Solid{}, err
		}
		cone, err := geom.NewConeSurfaceSlope(axisFrame, slope)
		if err != nil {
			return Solid{}, err
		}
		n := frame.ZAxis()
		reflection := geom.Identity()
		xyz := [3]float64{n.X, n.Y, n.Z}
		shift := apex.Sub(frame.Origin()).Add(n.Scale(2 * frame.Origin().Dot(n)))
		for i := range 3 {
			for j := range 3 {
				reflection[i][j] -= 2 * xyz[i] * xyz[j]
			}
			reflection[i][3] = [3]float64{shift.X, shift.Y, shift.Z}[i]
		}
		wallSurface = MapSurface(cone, reflection)
		axial = Interval{Min: topRadius / slope, Max: radius / slope}
	}
	if e != nil {
		return Solid{}, e
	}
	scale := max(radius, height)
	tolerance := scale * 1e-12
	if !finite(tolerance) || tolerance <= 0 || min(angle, 2*math.Pi-angle)*radius <= 8*tolerance || height <= 8*tolerance {
		return Solid{}, fmt.Errorf("parametric: cylindrical sector has unresolved sides")
	}
	top, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: height}), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return Solid{}, e
	}
	radial := func(u float64) (geom.PlaneSurface, error) {
		x := frame.XAxis().Scale(math.Cos(u)).Add(frame.YAxis().Scale(math.Sin(u)))
		f, e := geom.NewFrame(frame.Origin(), x, x.Cross(frame.ZAxis()))
		if e != nil {
			return geom.PlaneSurface{}, e
		}
		return geom.NewPlaneSurface(f)
	}
	start, e := radial(0)
	if e != nil {
		return Solid{}, e
	}
	end, e := radial(angle)
	if e != nil {
		return Solid{}, e
	}
	d, z, r := Interval{Max: angle}, Interval{Max: height}, Interval{Max: radius}
	wall := Face{Surface: wallSurface, U: d, V: axial, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {3, false}}}
	if conical {
		wall.Boundary = [4]Use{{2, true}, {1, true}, {0, true}, {3, true}}
		wall.Reversed = true
	}
	lo := Face{Surface: disk{frame}, U: d, V: r, Boundary: [4]Use{{4, false}, {5, false}, {0, true}, {6, false}}}
	hi := Face{Surface: disk{top}, U: d, V: Interval{Max: topRadius}, Boundary: [4]Use{{7, false}, {8, false}, {2, false}, {9, false}}, Reversed: true}
	first := Face{Surface: start, U: r, V: z, Boundary: [4]Use{{6, true}, {3, true}, {9, false}, {10, false}}}
	last := Face{Surface: end, U: r, V: z, Boundary: [4]Use{{5, false}, {1, false}, {8, true}, {10, false}}, Reversed: true}
	v := []geom.Vec3{frame.Origin(), top.Origin()}
	wallSides := [4]int{0, 1, 2, 3}
	if conical {
		wallSides = [4]int{2, 1, 0, 3}
		for i, f := range []geom.PlaneSurface{start, end} {
			ff := f.Frame()
			corners := []geom.Vec3{
				ff.ToWorld(geom.Vec3{}), ff.ToWorld(geom.Vec3{Y: height}),
				ff.ToWorld(geom.Vec3{X: radius}), ff.ToWorld(geom.Vec3{X: topRadius, Y: height}),
			}
			k := []float64{0, 0, 1, 1}
			s, e := geom.NewBSplineSurface(1, 1, 2, 2, corners, k, k, nil)
			if e != nil {
				return Solid{}, e
			}
			target := &first
			if i == 1 {
				target = &last
			}
			target.Surface, target.U, target.V = s, normalized, normalized
		}
	}
	for _, x := range [][3]float64{{radius, 0, 0}, {radius, angle, 0}, {topRadius, angle, height}, {topRadius, 0, height}} {
		v = append(v, frame.ToWorld(geom.Vec3{X: x[0] * math.Cos(x[1]), Y: x[0] * math.Sin(x[1]), Z: x[2]}))
	}
	curves := [4]geom.Curve{iso{wall, wallSides[0]}, iso{wall, wallSides[1]}, iso{wall, wallSides[2]}, iso{wall, wallSides[3]}}
	if conical {
		// A model curve can use the same geometric parameter as its
		// incident use without storing a reversing curve wrapper.
		curves[0], curves[1], curves[2], curves[3] = iso{lo, 2}, iso{last, 1}, iso{hi, 2}, iso{first, 1}
		lo.Boundary[2].Reversed = false
		wall.Boundary[3].Reversed = false
		wall.Boundary[1].Reversed = true
		wall.Boundary[2].Reversed = false
		first.Boundary[1].Reversed = false
	}
	edges := []Edge{
		{Start: 2, End: 3, Curve: curves[0], Range: normalized},
		{Start: 3, End: 4, Curve: curves[1], Range: normalized},
		{Start: 4, End: 5, Curve: curves[2], Range: normalized},
		{Start: 5, End: 2, Curve: curves[3], Range: normalized},
		{Start: 0, End: 0},
		{Start: 0, End: 3, Curve: iso{lo, 1}, Range: normalized},
		{Start: 2, End: 0, Curve: iso{lo, 3}, Range: normalized},
		{Start: 1, End: 1},
		{Start: 1, End: 4, Curve: iso{hi, 1}, Range: normalized},
		{Start: 5, End: 1, Curve: iso{hi, 3}, Range: normalized},
		{Start: 1, End: 0, Curve: iso{first, 3}, Range: normalized},
	}
	if conical {
		edges[0].Start, edges[0].End = 3, 2
		edges[3].Start, edges[3].End = 2, 5
	}
	return NewSolid(v, edges, []Face{wall, lo, hi, first, last}, tolerance)
}
