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

func TestObliqueCylinderSplitCut(t *testing.T) {
	ctx := context.Background()
	u, v := geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 4}
	c := geom.Vec3{X: 1, Y: 2, Z: 3}
	caps := [2]float64{-4, 4}
	out, e := ObliqueCylinderSplitCut(ctx, c, u, v, 1, 2, caps, caps, 1e-9)
	if e != nil || out.MinimumGapSquared.Cmp(big.NewRat(12, 1)) != 0 || !out.CapCoverage.Covers {
		t.Fatal(out, e)
	}
	common, e := ObliqueCylinderCommon(ctx, c, u, v, 1, 2, caps, caps, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	o := IntegrationOptions{RelativeTolerance: 1e-7}
	p, e := common.Solid.MassProperties(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	var total float64
	n := geom.Vec3{X: .8, Y: -.6}
	for i, s := range out.Solids {
		var data bytes.Buffer
		if e := WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := s.MassProperties(ctx, o)
		if e != nil || len(s.Faces()) != 5 || (m.Centroid.Sub(c).Dot(n) > 0) != (i == 1) {
			t.Fatal(i, m, e)
		}
		total += m.Volume
		d, e := s.Tessellate(ctx, TessellationOptions{Deflection: .8, AnalyticBound: true})
		if e != nil || d.ValidateSolid() != nil || d.SignedVolume() <= 0 {
			t.Fatal(i, "invalid mesh", e)
		}
		x, e := collision.SelfIntersectionsBVHContext(ctx, d, collision.Options{})
		if e != nil || !x.Complete || len(x.Contacts) != 0 {
			t.Fatal(x, e)
		}
		for _, x := range d.Vertices {
			a := x.Sub(c).Dot(n)
			if (i == 0 && a > -math.Sqrt(3)+1e-9) || (i == 1 && a < math.Sqrt(3)-1e-9) {
				t.Fatal("gap violated", x, a)
			}
		}
	}
	if math.Abs(total+p.Volume-16*math.Pi) > 1e-8 {
		t.Fatal(total, p.Volume)
	}
}
