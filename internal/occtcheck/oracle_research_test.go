//go:build research

package occtcheck

import (
	"context"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestOCCTSphericalPocket(t *testing.T) {
	r := runOracle(t)
	ctx := context.Background()
	s, _, e := trimmed.CutSphereFromBox(geom.Vec3{}, geom.Vec3{X: 8, Y: 6, Z: 4}, parametric.Ball{Center: geom.Vec3{X: 4, Y: 3, Z: 4.5}, Radius: 1}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, r["spherical-pocket"], p.Area, p.Volume, p.Centroid)
}
