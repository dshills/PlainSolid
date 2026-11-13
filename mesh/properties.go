package mesh

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// Properties describes a closed oriented mesh assuming uniform unit density.
// Volume is signed; reversed orientation changes its sign but not the centroid.
// A cavity must be oriented oppositely to the enclosing shell.
type Properties struct {
	Volume      float64
	SurfaceArea float64
	Centroid    geom.Vec3
}

// MassProperties integrates signed tetrahedra around a local origin. It validates
// topology but does not detect intersecting shells or incorrect cavity nesting.
// Zero total signed volume and nonfinite results are errors.
func (m Mesh) MassProperties() (Properties, error) {
	if err := m.ValidateSolid(); err != nil {
		return Properties{}, err
	}
	if len(m.Triangles) == 0 {
		return Properties{}, fmt.Errorf("mesh: empty solid has no centroid")
	}
	origin := m.Vertices[m.Triangles[0][0]]
	var sum, correction [4]float64
	for _, t := range m.Triangles {
		a, b, c := m.Vertices[t[0]].Sub(origin), m.Vertices[t[1]].Sub(origin), m.Vertices[t[2]].Sub(origin)
		v := a.Dot(b.Cross(c)) / 6
		center := a.Scale(0.25).Add(b.Scale(0.25)).Add(c.Scale(0.25))
		values := [4]float64{v, v * center.X, v * center.Y, v * center.Z}
		for i, x := range values {
			y := x - correction[i]
			next := sum[i] + y
			correction[i] = (next - sum[i]) - y
			sum[i] = next
		}
	}
	if !finite(sum[0]) || sum[0] == 0 {
		return Properties{}, fmt.Errorf("mesh: zero or nonfinite signed volume")
	}
	center := origin.Add(geom.Vec3{X: sum[1] / sum[0], Y: sum[2] / sum[0], Z: sum[3] / sum[0]})
	area := m.Area()
	if !center.IsFinite() || !finite(area) {
		return Properties{}, fmt.Errorf("mesh: mass properties overflow")
	}
	return Properties{Volume: sum[0], SurfaceArea: area, Centroid: center}, nil
}

// CentroidalInertia integrates the signed inertia tensor at unit density.
// Axes are the mesh coordinate axes with the origin at the mass centroid.
// Reversing all triangles reverses the sign of the tensor.
func (m Mesh) CentroidalInertia() ([3][3]float64, error) {
	var tensor [3][3]float64
	p, e := m.MassProperties()
	if e != nil {
		return tensor, e
	}
	scale := 0.
	for _, tr := range m.Triangles {
		for _, j := range tr {
			d := m.Vertices[j].Sub(p.Centroid)
			scale = math.Max(scale, math.Max(math.Abs(d.X), math.Max(math.Abs(d.Y), math.Abs(d.Z))))
		}
	}
	if scale <= 0 || !finite(scale) {
		return tensor, fmt.Errorf("mesh: invalid inertia scale")
	}
	normalize := func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.X / scale, Y: p.Y / scale, Z: p.Z / scale} }
	var correction [3][3]float64
	arr := func(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }
	for _, tr := range m.Triangles {
		a, b, c := normalize(m.Vertices[tr[0]].Sub(p.Centroid)), normalize(m.Vertices[tr[1]].Sub(p.Centroid)), normalize(m.Vertices[tr[2]].Sub(p.Centroid))
		mass := a.Dot(b.Cross(c)) / 6
		x, y, z, s := arr(a), arr(b), arr(c), arr(a.Add(b).Add(c))
		for i := range 3 {
			for j := i; j < 3; j++ {
				// Barycentric second moments of a tetrahedron with fourth
				// vertex at zero are V/10 for squared and V/20 for mixed terms.
				f := (x[i]*x[j] + y[i]*y[j] + z[i]*z[j] + s[i]*s[j]) * mass / 20
				q := f - correction[i][j]
				next := tensor[i][j] + q
				correction[i][j] = (next - tensor[i][j]) - q
				tensor[i][j] = next
			}
		}
	}
	x, y, z := tensor[0][0], tensor[1][1], tensor[2][2]
	tensor[0][0], tensor[1][1], tensor[2][2] = y+z, x+z, x+y
	for i := range 3 {
		for j := i; j < 3; j++ {
			v := tensor[i][j]
			if i != j {
				v = -v
			}
			for range 5 {
				v *= scale
			}
			if !finite(v) || i == j && v == 0 {
				return [3][3]float64{}, fmt.Errorf("mesh: inertia outside float64 range")
			}
			tensor[i][j], tensor[j][i] = v, v
		}
	}
	return tensor, nil
}
