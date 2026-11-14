package trimmed

import (
	"context"
	"plainsolid/geom"
	"testing"
)

// Exact determinant-one model warp z'=z+(x²+xy)/8. Unlike the earlier
// bilinear fixture, original and section edges varying x have curved images.
func makeNativeCurvedTubeBox(t *testing.T, lo, hi [3]float64) Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		for side, fixed := range []float64{lo[axis], hi[axis]} {
			poles := make([]geom.Vec3, 0, 9)
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					p := [3]float64{}
					p[axis] = fixed
					p[u] = lo[u] + (hi[u]-lo[u])*float64(i)/2
					p[v] = lo[v] + (hi[v]-lo[v])*float64(j)/2
					x2 := fixed * fixed
					if axis != 0 {
						index := i
						if v == 0 {
							index = j
						}
						x2 = []float64{lo[0] * lo[0], lo[0] * hi[0], hi[0] * hi[0]}[index]
					}
					p[2] += (x2 + p[0]*p[1]) / 8
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
				}
			}
			ku := []float64{lo[u], lo[u], lo[u], hi[u], hi[u], hi[u]}
			kv := []float64{lo[v], lo[v], lo[v], hi[v], hi[v], hi[v]}
			s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, ku, kv, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces = append(surfaces, s)
			domains = append(domains, geom.ParameterRectangle{UMin: lo[u], UMax: hi[u], VMin: lo[v], VMax: hi[v]})
			reversed = append(reversed, side == 0)
		}
	}
	result, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return result
}
