package plaincadbridge

import (
	"math"
	"testing"
)

func TestSingleBoreMeshKeepsVolumeWithScaleAwareSampling(t *testing.T) {
	for _, test := range []struct {
		name         string
		max          vector
		center       vector
		radius       float64
		maxTriangles int
	}{
		{"production benchmark", vector{85, 40, 12}, vector{10, 20, 0}, 3, 160},
		{"imported STEP proportions", vector{20, 10, 10}, vector{10, 5, 0}, 2, 540},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := New()
			base := box(t, b, vector{}, test.max)
			tool := circle(t, b, test.center, vector{0, 0, 1}, test.radius, float64(test.max[2]))
			body := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
			o := request{LinearDeflection: optionalNumber{value: .5, set: true}, AngularDeflection: optionalNumber{value: .2, set: true}}
			m, err := b.shapes[body.Handle].tessellate(o)
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Triangles) > test.maxTriangles {
				t.Fatalf("got %d triangles, want at most %d", len(m.Triangles), test.maxTriangles)
			}
			v := body.GeometryAssertions.Volume
			if delta := math.Abs(float32MeshVolume(m) - v); delta >= v*1e-4 {
				t.Fatalf("STL volume error %g for analytic volume %g", delta, v)
			}
		})
	}
}
