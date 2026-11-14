package occtcheck

import (
	"context"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/csg/offset"
	"plainsolid/geom"
)

func TestOCCTFrustumErosion(t *testing.T) {
	ref := runOracle(t, "frustum-erosion")
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	stock, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: -1, Y: 2, Z: 3}, 2, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, name := range []string{"frustum-erosion-rising", "frustum-erosion-falling"} {
		delta := [2]float64{}
		delta[1-i] = 3
		w, _, e := geom.DraftWrappedFrustum(ctx, stock, delta[0], delta[1], o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := offset.ErodeWrappedCircularFrustum(ctx, w, .5, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || len(s.Topology().Shells) != 1 {
			t.Fatal(s.Topology(), e)
		}
		compare(t, ref[name], p.Area, p.Volume, p.Centroid)
		wall, _, e := offset.WrappedCircularFrustumWall(ctx, w, .5, 1e-9, o)
		if e != nil || len(wall.Topology().Shells) != 2 {
			t.Fatal(wall.Topology(), e)
		}
		m, e := wall.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		compare(t, ref[[]string{"frustum-wall-rising", "frustum-wall-falling"}[i]], m.Area, m.Volume, m.Centroid)
	}
}

func TestOCCTFrustumOpenWall(t *testing.T) {
	ref := runOracle(t, "frustum-open-wall")
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: -1, Y: 2, Z: 3}, 2, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, taper := range []string{"rising", "falling"} {
		d := [2]float64{}
		d[1-i] = 3
		outer, _, e := geom.DraftWrappedFrustum(ctx, base, d[0], d[1], o)
		if e != nil {
			t.Fatal(e)
		}
		for j, end := range []string{"lower", "upper"} {
			s, _, e := offset.WrappedCircularFrustumOpenWall(ctx, outer, .5, 1e-9, j == 1, o)
			if e != nil || len(s.Topology().Shells) != 1 || s.Topology().InnerWires != 1 {
				t.Fatal(s.Topology(), e)
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, ref["frustum-open-wall-"+taper+"-"+end], p.Area, p.Volume, p.Centroid)
		}
	}
}
