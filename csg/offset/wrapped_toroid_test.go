//go:build research

package offset

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestCircularWrappedToroidWall(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCircularToroid(ctx, geom.Vec3{}, 2, .5, .5, o)
	if e != nil {
		t.Fatal(e)
	}
	s, e := WrappedCircularToroidWall(ctx, w, -.25, .125, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	h, e := s.ValidateNestedSplineComponents(ctx, trimmed.SplineShellBooleanOptions{})
	if e != nil || len(h) != 2 || h[1].Parent != 0 || h[1].Depth != 1 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
	wantV, wantA := 4*math.Pi*math.Pi*(.625*.625-.25*.25), 8*math.Pi*math.Pi*(.625+.25)
	if e != nil || math.Abs(p.Volume-wantV) > 1e-5 || math.Abs(p.Area-wantA) > 1e-5 || p.Centroid.Length() > 1e-6 {
		t.Fatal(p, wantV, wantA, e)
	}
	for _, q := range []struct {
		x float64
		l parametric.Location
	}{{2, parametric.Outside}, {2.5, parametric.Inside}, {2.625, parametric.Boundary}} {
		r, e := s.LocateNestedSplineMaterial(ctx, geom.Vec3{X: q.x}, trimmed.SplineShellBooleanOptions{})
		if e != nil || r != q.l {
			t.Fatal(q, r, e)
		}
	}
}
