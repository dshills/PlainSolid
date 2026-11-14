package step

import (
	"bytes"
	"plainsolid/geom"
	"testing"
)

func TestCapsuleExtrusionNativeGraph(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{X: 30, Y: 40, Z: 50}, geom.Vec3{Y: 1}, geom.Vec3{Z: -1})
	for _, inner := range []float64{0, 5} {
		m := CapsuleExtrusionModel{LengthUnit: .001, Bodies: []CapsuleExtrusionBody{{Name: "Arc rim", Frame: f, Left: geom.Vec2{X: -10}, Right: geom.Vec2{X: 10}, Radius: 6, InnerRadius: inner, Height: 5}}}
		var out bytes.Buffer
		if e := WriteCapsuleExtrusions(&out, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadCapsuleExtrusions(bytes.NewReader(out.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if got.Bodies[0].Radius != 6 || got.Bodies[0].InnerRadius != inner {
			t.Fatal(got)
		}
	}
}
