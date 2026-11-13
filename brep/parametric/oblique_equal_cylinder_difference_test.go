package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestObliqueEqualCylinderDifference(t *testing.T) {
	ctx := context.Background()
	u, v := geom.Vec3{X: 3, Y: 4}, geom.Vec3{X: 4, Y: 3}
	caps := [2]float64{-4, 4}
	o, e := ObliqueEqualCylinderDifference(ctx, geom.Vec3{}, u, v, 2, caps, caps, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	w, _ := o.CommonVolumeSquared.Float64()
	want := (math.Pi*4*40 - math.Sqrt(w)) / 2
	var centroids [2]geom.Vec3
	for i, s := range o.Solids {
		if len(s.Faces()) != 5 || len(o.StockWallFaces[i]) != 2 || len(o.StockCapFaces[i]) != 2 || len(o.CutterWallFaces[i]) != 1 {
			t.Fatal(i, o)
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		n, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := n.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(p.Volume-want) > 1e-8 || (p.Centroid.Dot(u) > 0) != (i == 1) {
			t.Fatal(i, p, want, e)
		}
		centroids[i] = p.Centroid
		m, e := n.Tessellate(ctx, TessellationOptions{Deflection: 3, AnalyticBound: true})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid component display", i, e)
		}
		c, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !c.Complete || len(c.Contacts) != 0 {
			t.Fatal(i, c, e)
		}
	}
	if centroids[0].Add(centroids[1]).Length() > 1e-8 {
		t.Fatal(centroids)
	}
}
