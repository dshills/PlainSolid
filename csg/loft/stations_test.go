package loft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/support"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func squareStation(side, z float64) []geom.Vec3 {
	h := side / 2
	return []geom.Vec3{{X: -h, Y: -h, Z: z}, {X: h, Y: -h, Z: z}, {X: h, Y: h, Z: z}, {X: -h, Y: h, Z: z}}
}
func TestStationsConvexCreaseAndCoplanarSeams(t *testing.T) {
	for _, sides := range [][3]float64{{2, 4, 2}, {2, 3, 4}} {
		rings := [][]geom.Vec3{squareStation(sides[0], 0), squareStation(sides[1], 1), squareStation(sides[2], 2)}
		r, e := PlanarStations(context.Background(), rings, StationOptions{})
		if e != nil {
			t.Fatal(sides, e)
		}
		if r.ExactVolume.Cmp(big.NewRat(56, 3)) != 0 {
			t.Fatal(r.ExactVolume)
		}
		m, e := r.Solid.MassProperties()
		if e != nil || math.Abs(m.Volume-56.0/3) > 1e-10 {
			t.Fatal(m, e)
		}
		top := r.Solid.Topology()
		if top.EulerCharacteristic != 2 || top.Shells != 1 || len(r.Solid.Faces()) != 10 {
			t.Fatal(top)
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
				t.Fatal("station seam lost")
			}
		}
		for k := range rings {
			for i, p := range rings[k] {
				if r.Solid.Vertices()[r.RingVertices[k][i]] != p {
					t.Fatal("ancestry")
				}
			}
		}
	}
}
func TestStationsTwoRingCompatibility(t *testing.T) {
	a, b := rings()
	x, e := Planar(context.Background(), a, b, Options{})
	if e != nil {
		t.Fatal(e)
	}
	y, e := PlanarStations(context.Background(), [][]geom.Vec3{a, b}, StationOptions{})
	if e != nil || !reflect.DeepEqual(x.Solid, y.Solid) || x.ExactVolume.Cmp(y.ExactVolume) != 0 {
		t.Fatal(e)
	}
}
func TestStationsRejectReentryOrderAndLimits(t *testing.T) {
	ctx := context.Background()
	a, b, c := squareStation(2, 0), squareStation(1, 1), squareStation(2, 2)
	if r, e := PlanarStations(ctx, [][]geom.Vec3{a, b, c}, StationOptions{}); !errors.Is(e, support.ErrNonconvex) || len(r.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
	b = squareStation(3, 1)
	for _, bad := range [][][]geom.Vec3{{a, c, b}, {a, b, b}, {a, b[:3], c}} {
		if _, e := PlanarStations(ctx, bad, StationOptions{}); e == nil {
			t.Fatal("bad station accepted")
		}
	}
	if _, e := PlanarStations(ctx, [][]geom.Vec3{a, b, c}, StationOptions{MaxStations: 2}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e := PlanarStations(ctx, [][]geom.Vec3{a, b, c}, StationOptions{MaxTotalVertices: 11}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := PlanarStations(cancelCtx, [][]geom.Vec3{a, b, c}, StationOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
