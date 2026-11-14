package parametric

import (
	"math"

	"plainsolid/geom"
)

// AnnularSector constructs a coaxially bored cylindrical sector with two
// annular caps and two radial walls.
func AnnularSector(frame geom.Frame, inner, outer, height, angle float64) (Solid, error) {
	if !finite(inner) || !finite(outer) || !finite(height) || !finite(angle) ||
		inner <= 0 || outer <= inner || height <= 0 || angle <= 0 || angle >= 2*math.Pi {
		return Solid{}, geom.ErrDomain
	}
	tol := max(outer, height) * 1e-12
	if min(angle, 2*math.Pi-angle)*inner <= 8*tol || min(outer-inner, height) <= 8*tol {
		return Solid{}, geom.ErrUnrepresentable
	}
	meridian := [4]geom.Vec2{{X: inner}, {X: outer}, {X: outer, Y: height}, {X: inner, Y: height}}
	var vertices [8]geom.Vec3
	var edges [12]Edge
	var faces [6]Face
	for i, u := range []float64{0, angle} {
		x := frame.XAxis().Scale(math.Cos(u)).Add(frame.YAxis().Scale(math.Sin(u)))
		chart, err := geom.NewFrame(frame.Origin(), x, x.Cross(frame.ZAxis()))
		if err != nil {
			return Solid{}, err
		}
		plane, err := geom.NewPlaneSurface(chart)
		if err != nil {
			return Solid{}, err
		}
		face := Face{Surface: plane, U: Interval{inner, outer}, V: Interval{Max: height}, Reversed: i == 1}
		for j := range 4 {
			face.Boundary[j] = Use{Edge: 4 + i*4 + j}
		}
		for j, p := range meridian {
			vertices[i*4+j] = frame.ToWorld(geom.Vec3{X: p.X * math.Cos(u), Y: p.X * math.Sin(u), Z: p.Y})
			edges[4+i*4+j] = Edge{Start: i*4 + j, End: i*4 + (j+1)%4, Curve: iso{face, j}, Range: normalized}
		}
		faces[4+i] = face
	}
	for i, a := range meridian {
		j := (i + 1) % 4
		surface, err := RevolutionSide(frame, a, meridian[j])
		if err != nil {
			return Solid{}, err
		}
		face := Face{Surface: surface, U: Interval{Max: angle}, V: normalized,
			Boundary: [4]Use{{i, false}, {8 + i, false}, {j, true}, {4 + i, true}}}
		faces[i] = face
		edges[i] = Edge{Start: i, End: i + 4, Curve: iso{face, 0}, Range: normalized}
	}
	return NewSolid(vertices[:], edges[:], faces[:], tol)
}
