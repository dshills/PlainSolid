package trimmed

import (
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestCompactObliqueBandRequiresPositiveInteriorGap(t *testing.T) {
	stock, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 20, Z: 20})
	if err != nil {
		t.Fatal(err)
	}
	solid, history, err := BoreCylinderAlong(stock, 0, 1, geom.Vec3{X: 4, Y: 4}, geom.Vec3{X: 1, Y: 1, Z: 4}, 1, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	face := solid.Faces()[history.WallFaces[0]]
	if !compactObliqueBand(face) {
		t.Fatal("separated bore wall did not qualify")
	}
	lower := face.Outer[0].Curve.(EllipseUV)
	upper := face.Outer[2].Curve.(EllipseUV)
	upper.Center.Y = lower.Center.Y + 1.125
	upper.Cos.Y = lower.Cos.Y - 1
	upper.Sin.Y = lower.Sin.Y - 1
	face.Outer[2].Curve = upper
	for i := range 2 {
		end, err := upper.EvalUV(float64(i))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			face.Outer[2].UVStart = end
			face.Outer[1].UVEnd = end
		} else {
			face.Outer[2].UVEnd = end
			face.Outer[3].UVStart = end
		}
	}
	if upper.From != math.Pi/2 || compactObliqueBand(face) {
		t.Fatal("positive endpoint gaps hid an interior crossing")
	}
}
