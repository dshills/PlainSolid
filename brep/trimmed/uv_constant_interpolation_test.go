package trimmed

import (
	"plainsolid/geom"
	"testing"
)

func TestConstantTrimCoordinatePreserved(t *testing.T) {
	for _, value := range []float64{1.51, 1.19, -1.51, 1e20, 1e-20} {
		for i := 0; i <= 1024; i++ {
			u := float64(i) / 1024
			a, b := geom.Vec2{X: 0, Y: value}, geom.Vec2{X: 1, Y: value}
			p := mix(a, b, u)
			if p.Y != value || p.X != u {
				t.Fatalf("constant trim changed at %g: %+v", u, p)
			}
			a, b = geom.Vec2{X: value, Y: 0}, geom.Vec2{X: value, Y: 1}
			p = mix(a, b, u)
			if p.X != value || p.Y != u {
				t.Fatalf("constant trim changed at %g: %+v", u, p)
			}
		}
	}
}
