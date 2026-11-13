package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExtrudePolygonWithCircularHoles(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: 3, Z: 4}, geom.Vec3{Z: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	p := []geom.Vec2{{}, {X: 8}, {X: 8, Y: 4}, {Y: 4}}
	h := []ExtrusionCircle{{geom.Vec2{X: 2, Y: 2}, .75}, {geom.Vec2{X: 6, Y: 2}, .75}}
	for _, z := range []float64{-2, 3} {
		s, e := ExtrudePolygonWithCircularHoles(ctx, f, p, h, z)
		if e != nil || len(s.faces) != 14 || s.Topology().EulerCharacteristic != -2 || s.Topology().InnerWires != 4 {
			t.Fatal(z, s.Topology(), e)
		}
		v, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		want := math.Abs(z) * (32 - 2*math.Pi*.75*.75)
		if e != nil || math.Abs(v.Volume-want) > 1e-7 || math.Abs(v.Centroid.X-(2+z/2)) > 1e-8 {
			t.Fatal(z, v.Volume, want, v.Centroid, e)
		}
		var file bytes.Buffer
		if e := WriteJSON(&file, s); e != nil {
			t.Fatal(e)
		}
		q, e := ReadJSONContext(ctx, &file, JSONLimits{})
		if e != nil || q.Topology().EulerCharacteristic != -2 {
			t.Fatal(q.Topology(), e)
		}
		for _, c := range []struct {
			p    geom.Vec3
			want parametric.Location
		}{
			{geom.Vec3{X: 4, Y: 2, Z: z / 2}, parametric.Inside},
			{geom.Vec3{X: 2, Y: 2, Z: z / 2}, parametric.Outside},
			{geom.Vec3{X: 2.75, Y: 2, Z: z / 2}, parametric.Boundary},
			{geom.Vec3{X: 4, Y: 2}, parametric.Boundary},
		} {
			x, e := q.Classify(f.ToWorld(c.p), 1e-8)
			if e != nil || x != c.want {
				t.Fatal("retained analytic classification", c.p, x, c.want, e)
			}
		}
		m, e := q.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, MaxTriangles: 150000})
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-want) > .02*want {
			t.Fatal("closed holed extrusion mesh", len(m.Triangles), m.SignedVolume(), e)
		}
	}
	for _, h := range [][]ExtrusionCircle{{{geom.Vec2{X: .75, Y: 2}, .75}}, {{geom.Vec2{X: 2, Y: 2}, 1}, {geom.Vec2{X: 4, Y: 2}, 1}}} {
		if s, e := ExtrudePolygonWithCircularHoles(ctx, f, p, h, 2); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
			t.Fatal("tangent hole accepted", e)
		}
	}
}
