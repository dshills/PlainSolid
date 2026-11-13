package loft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/support"
	"plainsolid/geom"
	"testing"
)

func TestPiecewiseConvexWaist(t *testing.T) {
	rings := [][]geom.Vec3{squareStation(2, 0), squareStation(1, 1), squareStation(2, 2)}
	r, e := PlanarStationBands(context.Background(), rings, StationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExactVolume.Cmp(big.NewRat(14, 3)) != 0 || r.Supports.Convex {
		t.Fatal(r.ExactVolume)
	}
	for i, v := range r.BandVolumes {
		if v.Cmp(big.NewRat(7, 3)) != 0 || !r.BandSupports[i].Convex {
			t.Fatal(v)
		}
	}
	m, e := r.Solid.MassProperties()
	if e != nil || math.Abs(m.Volume-14.0/3) > 1e-10 || m.Centroid.Sub(geom.Vec3{Z: 1}).Length() > 1e-12 {
		t.Fatal(m, e)
	}
	if r.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal("topology")
	}
	for i, id := range r.RingEdges[1] {
		count := 0
		for _, fi := range []int{r.BandFaces[0][i], r.BandFaces[1][i]} {
			for _, u := range r.Solid.Faces()[fi].Boundary {
				if u.Edge == id {
					count++
				}
			}
		}
		if count != 2 {
			t.Fatal("seam")
		}
	}
	rings[1][0].X = 100
	if r.Solid.Vertices()[r.RingVertices[1][0]].X != -.5 {
		t.Fatal("input alias")
	}
	r.BandVolumes[0].SetInt64(0)
	if r.ExactVolume.Cmp(big.NewRat(14, 3)) != 0 {
		t.Fatal("volume alias")
	}
}
func TestPiecewiseRejectsAndKeepsGlobalContract(t *testing.T) {
	ctx := context.Background()
	rings := [][]geom.Vec3{squareStation(2, 0), squareStation(1, 1), squareStation(2, 2)}
	if _, e := PlanarStations(ctx, rings, StationOptions{}); !errors.Is(e, support.ErrNonconvex) {
		t.Fatal(e)
	}
	if r, e := PlanarStationBands(ctx, rings, StationOptions{Options: Options{Support: support.Options{MaxWork: 1}}}); !errors.Is(e, support.ErrLimit) || len(r.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
	rings[1][1].X += .125
	if r, e := PlanarStationBands(ctx, rings, StationOptions{}); e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("twisted accepted")
	}
	// Concave identical rings give planar sides but no convex slab certificate.
	concave := []geom.Vec3{{}, {X: 2}, {X: 1, Y: .5}, {X: 2, Y: 2}, {Y: 2}}
	upper := append([]geom.Vec3(nil), concave...)
	for i := range upper {
		upper[i].Z = 1
	}
	if r, e := PlanarStationBands(ctx, [][]geom.Vec3{concave, upper}, StationOptions{}); !errors.Is(e, support.ErrNonconvex) || len(r.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
}
