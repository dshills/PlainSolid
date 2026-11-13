package intersect

import (
	"context"

	"plainsolid/geom"
	"testing"
)

func TestTubeModelIntervalIncludesBoundaryAnchor(t *testing.T) {
	ctx := context.Background()
	makePatch := func(p []geom.Vec3, x0, x1, y0, y1 float64) Patch {
		t.Helper()
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p,
			[]float64{x0, x0, x1, x1}, []float64{y0, y0, y1, y1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Patch{Surface: s, Min: geom.Vec2{X: x0, Y: y0}, Max: geom.Vec2{X: x1, Y: y1}}
	}
	a := makePatch([]geom.Vec3{{}, {Y: .25}, {X: .125}, {X: .125, Y: .25, Z: 1.0 / 32}}, 0, .125, 0, .25)
	b := makePatch([]geom.Vec3{{X: -.25, Z: -.25}, {X: -.25, Y: .25, Z: -.5}, {X: .25, Z: .25}, {X: .25, Y: .25}}, -.25, .25, 0, .25)
	da, db := geom.ParameterRectangle{UMax: .125, VMax: .25}, geom.ParameterRectangle{UMin: -.25, UMax: .25, VMax: .25}
	anchor := &TubeBoundaryAnchor{}
	c := geom.SurfaceTubeCertificate{FreeAxis: 0, ProvenBranch: true, FreeParameter: geom.CertifiedScalarRange{Min: 0, Max: .125}}
	s := SplineSeedReport{Attempts: []SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &c, BoundaryAnchor: anchor}}}
	o := SplinePairTraceOptions{Coverage: SplineTubeCoverageOptions{Isolation: geom.SurfaceSurfaceIsolationOptions{ParameterTolerance: 1}}}
	r, e := CertifySplinePairBranches(ctx, a, b, s, o)
	if e != nil || !r.Complete || !r.Coverage.CoversAllContacts || len(r.Curves) != 1 || len(r.Curves[0]) != 1 {
		t.Fatal(r.Issues, r.Coverage.Unresolved, e)
	}
	seg := r.Curves[0][0]

	box, e := seg.EncloseFreeInterval(ctx, 0, .125, geom.SurfaceTubeIntervalOptions{})
	if !box.HasBounds {
		t.Fatal("missing anchor enclosure", box, e)
	}
	for i := 0; i <= 16; i++ {
		u := float64(i) / 128
		v := u / (1 + u)
		p := geom.Vec3{X: u, Y: v, Z: u * v}
		if p.X < box.Bounds.Min.X || p.X > box.Bounds.Max.X || p.Y < box.Bounds.Min.Y || p.Y > box.Bounds.Max.Y || p.Z < box.Bounds.Min.Z || p.Z > box.Bounds.Max.Z {
			t.Fatal("anchor branch escaped", p, box.Bounds)
		}
	}
}
