package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPerpendicularCylinderUnion(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: -1, Z: 2}
	out, e := PerpendicularCylinderUnion(ctx, center, 2, 3, 4, 1e-9)
	if e != nil || len(out.Solid.Faces()) != 16 || len(out.WallFaces[0]) != 4 || len(out.WallFaces[1]) != 4 {
		t.Fatal(e, out)
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	coef, _ := out.VolumePiCoefficient.Float64()
	overlap, _ := out.OverlapVolume.Float64()
	if e != nil || math.Abs(p.Volume-(math.Pi*coef-overlap)) > 1e-8 || p.Centroid.Sub(center).Length() > 1e-9 {
		t.Fatal(p, e)
	}
	wantArea := 2*math.Pi*2*(2*3+2*4) + 4*math.Pi*4 - 64
	if math.Abs(p.Area-wantArea) > 1e-8 {
		t.Fatal(p.Area, wantArea)
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, out.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || r != p {
		t.Fatal("reload changed union", r, e)
	}
	m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .6, AnalyticBound: true})
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
		t.Fatal("invalid union mesh", e)
	}
	c, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !c.Complete || len(c.Contacts) != 0 {
		t.Fatal(c, e)
	}
}
