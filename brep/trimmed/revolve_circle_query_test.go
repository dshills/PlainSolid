package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

// These expectations use the swept disk equation, independently of surface
// projection and the quartic ray solver. They exercise the actual constructor
// and persisted boundary, including its signed-angle frame change.
func TestRevolveCircleAnalyticQueries(t *testing.T) {
	frame, _ := geom.NewFrame(geom.Vec3{X: 2, Y: -1, Z: 4}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, angle := range []float64{math.Pi / 2, -math.Pi / 2, 3 * math.Pi / 2, -3 * math.Pi / 2} {
		s, e := RevolveCircle(context.Background(), frame, 3, .25, .5, angle)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSONContext(context.Background(), &b, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		for _, solid := range []Solid{s, loaded} {
			world := func(u, r, z float64) geom.Vec3 {
				return frame.ToWorld(geom.Vec3{X: r * math.Cos(u), Y: r * math.Sin(u), Z: .25 + z})
			}
			for _, q := range []struct {
				p    geom.Vec3
				want parametric.Location
			}{
				{world(angle/2, 3, 0), parametric.Inside},
				{world(angle/2, 3, .75), parametric.Outside},
				{world(angle/2, 3, .5), parametric.Boundary},
				{world(0, 3, 0), parametric.Boundary},
				{world(angle, 3, 0), parametric.Boundary},
				{world(angle/2+math.Pi, 0, 0), parametric.Outside},
			} {
				got, err := solid.Classify(q.p, 1e-8)
				if err != nil || got != q.want {
					t.Fatalf("angle %g point %v: %v %v want %v", angle, q.p, got, err, q.want)
				}
			}
			// An axis-parallel line through the mid-sweep center cuts the torus
			// at z=+/-minor, never either end cap.
			hits, err := solid.Ray(world(angle/2, 3, -1), frame.ZAxis(), 2, 1e-8)
			if err != nil || len(hits) != 2 {
				t.Fatalf("side angle %g: %v %v", angle, hits, err)
			}
			for i, want := range []float64{.5, 1.5} {
				if math.Abs(hits[i].Distance-want) > 1e-7 || hits[i].Tangent || len(hits[i].Faces) != 1 || hits[i].Faces[0] != 0 {
					t.Fatalf("side hit %v", hits[i])
				}
			}
			// Approach each end disk along its inward angular tangent. The first
			// hit is exactly the cap center, a model unit from the ray origin.
			hand := math.Copysign(1, angle)
			for i, u := range []float64{0, angle} {
				sign := hand
				if i == 1 {
					sign = -sign
				}
				dir := frame.XAxis().Scale(-math.Sin(u) * sign).Add(frame.YAxis().Scale(math.Cos(u) * sign))
				center := world(u, 3, 0)
				hits, err := solid.Ray(center.Sub(dir), dir, 1.1, 1e-8)
				wantFace := i + 1
				if angle < 0 {
					wantFace = 2 - i
				}
				if err != nil || len(hits) != 1 || math.Abs(hits[0].Distance-1) > 1e-7 || hits[0].Tangent || len(hits[0].Faces) != 1 || hits[0].Faces[0] != wantFace {
					t.Fatalf("cap angle %g end %d: %v %v", angle, i, hits, err)
				}
			}
		}
	}
}
