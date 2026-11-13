package occtcheck

import (
	"context"
	"testing"

	"plainsolid/brep"
	"plainsolid/csg/draft"
	"plainsolid/geom"
)

func TestOCCTTaperedPlanarChamfer(t *testing.T) {
	ref := runOracle(t, "feature")["tapered-planar-chamfer"]
	stock, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 2})
	if err != nil {
		t.Fatal(err)
	}
	vs, es := stock.Vertices(), stock.Edges()
	edge, face := -1, -1
	for i, e := range es {
		a, b := vs[e.Start], vs[e.End]
		if a.X == 0 && a.Y == 0 && b.X == 0 && b.Y == 0 {
			edge = i
		}
	}
	for i, f := range stock.Faces() {
		onX := true
		for _, u := range f.Boundary {
			onX = onX && vs[es[u.Edge].Start].X == 0
		}
		if onX {
			face = i
		}
	}
	if edge < 0 || face < 0 {
		t.Fatal("original vertical edge or X=0 support missing")
	}
	sel := draft.TaperedEdgeChamfer{Edge: edge, Face: face, StartSetback: .25, StartOtherSetback: .5, EndSetback: .5, EndOtherSetback: 1}
	if vs[es[edge].Start].Z != 0 {
		sel.StartSetback, sel.EndSetback = sel.EndSetback, sel.StartSetback
		sel.StartOtherSetback, sel.EndOtherSetback = sel.EndOtherSetback, sel.StartOtherSetback
	}
	r, err := draft.ChamferTapered(context.Background(), stock, []draft.TaperedEdgeChamfer{sel}, false, draft.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.EdgeFaces) != 1 || r.EdgeFaces[0] < 0 || r.ExactVolume.RatString() != "569/24" {
		t.Fatal("expected original bevel ancestry and independent exact volume", r.EdgeFaces, r.ExactVolume)
	}
	m, err := r.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, ref, m.SurfaceArea, m.Volume, m.Centroid)
}
