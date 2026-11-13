package step

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func splineLensFixture(t testing.TB) SplineLensBody {
	var supports [2]*geom.BSplineSurface
	for side, sign := range []float64{1, -1} {
		p := []geom.Vec3{}
		sq := []float64{.25, -.25, .25}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 2 + sign*(1./16-sq[i]-sq[j])/2})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		supports[side] = s
	}
	return SplineLensBody{Name: "exact elliptical lens", FaceNames: [2]string{"upper bowl", "lower bowl"}, Upper: supports[0], Lower: supports[1], Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: .25, Y: .25}, LengthUnit: .001}
}
func TestSplineLensRoundtrip(t *testing.T) {
	m := splineLensFixture(t)
	var b bytes.Buffer
	if e := WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.FaceNames != m.FaceNames || got.Center != m.Center || got.Radii != m.Radii || got.LengthUnit != m.LengthUnit {
		t.Fatal(got)
	}
	p, e := got.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(p.Volume-math.Pi/512) > 1e-10 {
		t.Fatal(p, e)
	}
	if _, e = ReadCurvedOvalFace(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("open reader accepted lens")
	}
}
