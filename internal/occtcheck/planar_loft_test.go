package occtcheck

import (
	"context"
	"testing"

	"plainsolid/csg/loft"
	"plainsolid/geom"
)

func loftRectangle(x, y, width, depth, z float64) []geom.Vec3 {
	return []geom.Vec3{{X: x, Y: y, Z: z}, {X: x + width, Y: y, Z: z},
		{X: x + width, Y: y + depth, Z: z}, {X: x, Y: y + depth, Z: z}}
}

func TestOCCTExactPlanarLofts(t *testing.T) {
	refs := runOracle(t, "planar-loft")
	ctx := context.Background()
	a, err := loft.Planar(ctx, loftRectangle(0, 0, 4, 3, 0), loftRectangle(.5, -.5, 2, 5, 6), loft.Options{})
	if err != nil || len(a.SideFaces) != 4 || a.ExactVolume.RatString() != "70" {
		t.Fatal("rectangular loft", err)
	}
	m, err := a.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["planar-rectangular-loft"], m.SurfaceArea, m.Volume, m.Centroid)
	rings := [][]geom.Vec3{loftRectangle(-1, -1, 2, 2, 0), loftRectangle(-2, -2, 4, 4, 1), loftRectangle(-1, -1, 2, 2, 2)}
	b, err := loft.PlanarStations(ctx, rings, loft.StationOptions{})
	if err != nil || len(b.RingEdges[1]) != 4 || len(b.BandFaces) != 2 || b.ExactVolume.RatString() != "56/3" {
		t.Fatal("station loft", err)
	}
	m, err = b.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["planar-convex-station-loft"], m.SurfaceArea, m.Volume, m.Centroid)
	rings[1] = loftRectangle(-.5, -.5, 1, 1, 1)
	c, err := loft.PlanarStationBands(ctx, rings, loft.StationOptions{})
	if err != nil || len(c.RingEdges[1]) != 4 || len(c.BandFaces) != 2 || c.ExactVolume.RatString() != "14/3" {
		t.Fatal("piecewise convex loft", err)
	}
	m, err = c.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["planar-waist-station-loft"], m.SurfaceArea, m.Volume, m.Centroid)
	elbow := [][]geom.Vec3{
		{{Y: -.5, Z: -.5}, {Y: .5, Z: -.5}, {Y: .5, Z: .5}, {Y: -.5, Z: .5}},
		{{X: 2.5, Y: -.5, Z: -.5}, {X: 1.5, Y: .5, Z: -.5}, {X: 1.5, Y: .5, Z: .5}, {X: 2.5, Y: -.5, Z: .5}},
		{{X: 2.5, Y: 3, Z: -.5}, {X: 1.5, Y: 3, Z: -.5}, {X: 1.5, Y: 3, Z: .5}, {X: 2.5, Y: 3, Z: .5}},
	}
	d, err := loft.MiteredStations(ctx, elbow, loft.StationOptions{})
	if err != nil || len(d.RingEdges[1]) != 4 || len(d.BandFaces) != 2 || d.ExactVolume.RatString() != "5" {
		t.Fatal("mitered elbow", err)
	}
	m, err = d.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, refs["planar-mitered-elbow"], m.SurfaceArea, m.Volume, m.Centroid)
}
