package mesh

import (
	"math"
	"testing"

	"plainsolid/geom"
)

func TestInertiaBoxRotated(t *testing.T) {
	m, e := Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	r, e := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, .6)
	if e != nil {
		t.Fatal(e)
	}
	m, e = m.Transformed(geom.Translation(geom.Vec3{X: 10, Y: -4}).Mul(r))
	if e != nil {
		t.Fatal(e)
	}
	for _, sign := range []float64{1, -1} {
		if sign < 0 {
			for i := range m.Triangles {
				m.Triangles[i][1], m.Triangles[i][2] = m.Triangles[i][2], m.Triangles[i][1]
			}
		}
		ix, e := m.CentroidalInertia()
		if e != nil {
			t.Fatal(e)
		}
		for i := range 3 {
			for j := range 3 {
				want := 0.
				for k, w := range [3]float64{50, 40, 26} {
					want += r[i][k] * r[j][k] * w
				}
				if math.Abs(ix[i][j]-sign*want) > 1e-10 {
					t.Fatalf("[%d,%d] %g != %g", i, j, ix[i][j], sign*want)
				}
			}
		}
	}
}
