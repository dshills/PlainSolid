package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestObliqueEqualCylinderCommon(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	a, b := geom.Vec3{X: 3, Y: 4}, geom.Vec3{X: 4, Y: 3}
	caps := [2]float64{-3, 3}
	out, e := ObliqueEqualCylinderCommon(ctx, center, a, b, 2, caps, caps, 1e-9)
	if e != nil || !out.Measure.CapsCover || len(out.Solid.Vertices()) != 2 || len(out.CylinderFaces[0]) != 2 || len(out.CylinderFaces[1]) != 2 {
		t.Fatal(e, out)
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	want := 128. / 3 * 25 / 7
	if e != nil || math.Abs(p.Volume-want) > 1e-7 || p.Centroid.Sub(center).Length() > 1e-8 {
		t.Fatal(p, want, e)
	}
	for i, faces := range out.CylinderFaces {
		n := [2]geom.Vec3{a, b}[i]
		for _, id := range faces {
			q, e := out.Solid.Faces()[id].Surface.Eval(.4, .3)
			d := q.Sub(center)
			if e != nil || math.Abs(d.Dot(d)-d.Dot(n)*d.Dot(n)/25-4) > 1e-11 {
				t.Fatal(q, e)
			}
		}
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
		t.Fatal("native oblique reload changed mass", r, e)
	}
	m, e := s.Tessellate(ctx, TessellationOptions{Deflection: 1, AnalyticBound: true})
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
		t.Fatal("oblique certified mesh invalid", e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("oblique mesh contacts", contacts, e)
	}
	if bad, e := ObliqueEqualCylinderCommon(ctx, center, a, b, 2, [2]float64{-1, 1}, caps, 1e-9); !errors.Is(e, geom.ErrConvergence) || len(bad.Solid.Faces()) != 0 {
		t.Fatal("clipped cap accepted", e)
	}
}
