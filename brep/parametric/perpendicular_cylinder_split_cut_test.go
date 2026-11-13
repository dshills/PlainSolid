package parametric

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPerpendicularCylinderSplitCut(t *testing.T) {
	ctx := context.Background()
	c := geom.Vec3{X: 2, Y: -1, Z: 3}
	out, e := PerpendicularCylinderSplitCut(ctx, c, 2, 3, 5, 4, 1e-9)
	if e != nil || out.MinimumGapSquared.Cmp(big.NewRat(20, 1)) != 0 {
		t.Fatal(e, out)
	}
	common, e := PerpendicularCylinderCommonRadii(ctx, c, 2, 3, 5, 4, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	o := IntegrationOptions{RelativeTolerance: 1e-7}
	p, e := common.Solid.MassProperties(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	want := (40*math.Pi - p.Volume) / 2
	for i, s := range out.Solids {
		var data bytes.Buffer
		if e := WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := s.MassProperties(ctx, o)
		if e != nil || len(s.Faces()) != 5 || math.Abs(q.Volume-want) > 1e-8 || (q.Centroid.X > c.X) != (i == 1) {
			t.Fatal(i, q, want, e)
		}
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .8, AnalyticBound: true})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid component display", e)
		}
		x, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !x.Complete || len(x.Contacts) != 0 {
			t.Fatal(x, e)
		}
		for _, v := range m.Vertices {
			if (i == 0 && v.X > c.X-math.Sqrt(5)+1e-9) || (i == 1 && v.X < c.X+math.Sqrt(5)-1e-9) {
				t.Fatal("mesh leaves certified separation", v)
			}
		}
	}
}
