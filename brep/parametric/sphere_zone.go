package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// SphereZone keeps the portion of a sphere between two latitude planes in
// frame coordinates. Both latitudes are in radians, strictly between the
// poles. The boundary retains one spherical face and two exact disks.
func SphereZone(frame geom.Frame, radius, lowerLatitude, upperLatitude, tolerance float64) (Solid, error) {
	if !finite(lowerLatitude) || !finite(upperLatitude) || lowerLatitude <= -math.Pi/2 ||
		upperLatitude >= math.Pi/2 || lowerLatitude >= upperLatitude || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, geom.ErrInvalid
	}
	surface, e := geom.NewSphereSurface(frame, radius)
	if e != nil {
		return Solid{}, e
	}
	side := Face{Surface: surface, U: circleDomain, V: Interval{lowerLatitude, upperLatitude},
		Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}}
	vertices := make([]geom.Vec3, 4)
	faces := []Face{side}
	edges := []Edge{
		{Start: 0, End: 0, Curve: iso{side, 0}, Range: normalized},
		{Start: 0, End: 1, Curve: iso{side, 1}, Range: normalized},
		{Start: 1, End: 1, Curve: iso{side, 2}, Range: normalized},
	}
	z := [2]float64{radius * math.Sin(lowerLatitude), radius * math.Sin(upperLatitude)}
	if z[1]-z[0] <= tolerance {
		return Solid{}, fmt.Errorf("parametric: spherical zone height is unresolved at tolerance")
	}
	for i, latitude := range [2]float64{lowerLatitude, upperLatitude} {
		r := radius * math.Cos(latitude)
		if r <= tolerance {
			return Solid{}, fmt.Errorf("parametric: spherical zone rim is unresolved at tolerance")
		}
		center := frame.ToWorld(geom.Vec3{Z: z[i]})
		f, e := geom.NewFrame(center, frame.XAxis(), frame.ZAxis())
		if e != nil {
			return Solid{}, e
		}
		rim, e := surface.Eval(0, latitude)
		if e != nil {
			return Solid{}, e
		}
		vertices[i], vertices[i+2] = rim, center
		n := len(edges)
		cap := Face{Surface: disk{f}, U: circleDomain, V: Interval{0, r},
			Boundary: [4]Use{{n, false}, {n + 1, false}, {2 * i, i == 0}, {n + 1, true}}, Reversed: i == 1}
		edges = append(edges, Edge{Start: i + 2, End: i + 2},
			Edge{Start: i + 2, End: i, Curve: iso{cap, 1}, Range: normalized})
		faces = append(faces, cap)
	}
	return NewSolid(vertices, edges, faces, tolerance)
}
