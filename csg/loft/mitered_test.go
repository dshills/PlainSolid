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

func elbowRings() [][]geom.Vec3 {
	return [][]geom.Vec3{
		{{X: 0, Y: -.5, Z: -.5}, {X: 0, Y: .5, Z: -.5}, {X: 0, Y: .5, Z: .5}, {X: 0, Y: -.5, Z: .5}},
		{{X: 2.5, Y: -.5, Z: -.5}, {X: 1.5, Y: .5, Z: -.5}, {X: 1.5, Y: .5, Z: .5}, {X: 2.5, Y: -.5, Z: .5}},
		{{X: 2.5, Y: 3, Z: -.5}, {X: 1.5, Y: 3, Z: -.5}, {X: 1.5, Y: 3, Z: .5}, {X: 2.5, Y: 3, Z: .5}},
	}
}
func TestMiteredElbowExact(t *testing.T) {
	for _, rev := range []bool{false, true} {
		rings := elbowRings()
		if rev {
			for _, ring := range rings {
				for i, j := 0, 3; i < j; i, j = i+1, j-1 {
					ring[i], ring[j] = ring[j], ring[i]
				}
			}
		}
		r, e := MiteredStations(context.Background(), rings, StationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if r.ExactVolume.Cmp(big.NewRat(5, 1)) != 0 || r.BandVolumes[0].Cmp(big.NewRat(2, 1)) != 0 || r.BandVolumes[1].Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatal(r.ExactVolume, r.BandVolumes)
		}
		m, e := r.Solid.MassProperties()
		if e != nil || math.Abs(m.Volume-5) > 1e-12 || m.Centroid.Sub(geom.Vec3{X: 13.0 / 8, Y: 7.0 / 8}).Length() > 1e-12 {
			t.Fatal(m, e)
		}
		if r.Solid.Topology().EulerCharacteristic != 2 || len(r.Solid.Faces()) != 10 {
			t.Fatal("topology")
		}
		for i, id := range r.RingEdges[1] {
			uses := 0
			for _, fi := range []int{r.BandFaces[0][i], r.BandFaces[1][i]} {
				for _, u := range r.Solid.Faces()[fi].Boundary {
					if u.Edge == id {
						uses++
					}
				}
			}
			if uses != 2 {
				t.Fatal("miter lost")
			}
		}
		if _, e := PlanarStations(context.Background(), rings, StationOptions{}); e == nil {
			t.Fatal("parallel contract changed")
		}
	}
}
func TestMiteredWholeVertexSeparation(t *testing.T) {
	rings := elbowRings()
	for i := range rings[2] {
		rings[2][i].Y = .25
	}
	if r, e := MiteredStations(context.Background(), rings, StationOptions{}); !errors.Is(e, ErrUnsupported) || len(r.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
	rings = elbowRings()
	if _, e := MiteredStations(context.Background(), rings, StationOptions{Options: Options{Support: support.Options{MaxWork: 1}}}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
