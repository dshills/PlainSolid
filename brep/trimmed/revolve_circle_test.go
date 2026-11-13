package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRevolveCircle(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: -3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	for _, angle := range []float64{math.Pi / 2, -math.Pi / 2, 2 * math.Pi} {
		s, e := RevolveCircle(ctx, frame, 3, 1, .5, angle)
		if e != nil {
			t.Fatal(angle, e)
		}
		wantFaces, chi := 3, 2
		if angle == 2*math.Pi {
			wantFaces, chi = 1, 0
		}
		if len(s.faces) != wantFaces || s.Topology().EulerCharacteristic != chi {
			t.Fatal(angle, s.Topology())
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		want := math.Abs(angle) * math.Pi * .25 * 3
		if e != nil || math.Abs(m.Volume-want) > want*1e-8 || math.Abs(m.Centroid.X-2) > 1e-8 {
			t.Fatal(angle, m, want, e)
		}
		if angle != 2*math.Pi && (m.Centroid.Z+3)*angle <= 0 {
			t.Fatal("centroid lies on wrong sweep side", angle, m.Centroid)
		}
		var b bytes.Buffer
		if e := WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		q, e := ReadJSONContext(ctx, &b, JSONLimits{})
		if e != nil || q.Topology().EulerCharacteristic != chi {
			t.Fatal(angle, q.Topology(), e)
		}
	}
}
