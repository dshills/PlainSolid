package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// SphereCylinder combines the sphere centered at frame.Origin with an
// infinite cylinder along the frame's Z axis. Intersection retains the core;
// Difference drills a through-bore. Both results are bounded. The result
// preserves exact spherical and cylindrical patches.
func SphereCylinder(frame geom.Frame, sphereRadius, boreRadius float64, op BooleanOp, tolerance float64) (Solid, error) {
	if sphereRadius <= 0 || boreRadius <= 0 || !finite(sphereRadius) || !finite(boreRadius) || !finite(tolerance) || tolerance <= 0 ||
		frame.ZAxis().Length() == 0 || op != Intersection && op != Difference {
		return Solid{}, geom.ErrInvalid
	}
	if boreRadius >= sphereRadius {
		if op == Difference {
			return Solid{}, nil
		}
		return Sphere(frame.Origin(), sphereRadius)
	}
	if sphereRadius-boreRadius <= tolerance || boreRadius <= tolerance {
		return Solid{}, fmt.Errorf("parametric: unresolved sphere/cylinder dimensions")
	}
	s, e := geom.NewSphereSurface(frame, sphereRadius)
	if e != nil {
		return Solid{}, e
	}
	c, e := geom.NewCylinderSurface(frame, boreRadius)
	if e != nil {
		return Solid{}, e
	}
	z := math.Sqrt(sphereRadius-boreRadius) * math.Sqrt(sphereRadius+boreRadius)
	v := math.Atan2(z, boreRadius)
	if !finite(z) || z <= tolerance || v <= 0 || v >= math.Pi/2 {
		return Solid{}, fmt.Errorf("parametric: unresolved sphere/cylinder section")
	}
	var faces []Face
	for i := range 4 {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		faces = append(faces, Face{Surface: c, U: u, V: Interval{-z, z}, Reversed: op == Difference})
		if op == Difference {
			faces = append(faces, Face{Surface: s, U: u, V: Interval{-v, v}})
		} else {
			faces = append(faces, Face{Surface: s, U: u, V: Interval{-math.Pi / 2, -v}}, Face{Surface: s, U: u, V: Interval{v, math.Pi / 2}})
		}
	}
	return joinPrimitivePatches(faces, tolerance)
}
