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

func rings() ([]geom.Vec3, []geom.Vec3) {
	return []geom.Vec3{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 3}, {X: 0, Y: 3}}, []geom.Vec3{{X: .5, Y: -.5, Z: 6}, {X: 2.5, Y: -.5, Z: 6}, {X: 2.5, Y: 4.5, Z: 6}, {X: .5, Y: 4.5, Z: 6}}
}
func TestPlanarIndependentIntegralAndPlacement(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, placed := range []bool{false, true} {
			a, b := rings()
			if reverse {
				for i, j := 0, 3; i < j; i, j = i+1, j-1 {
					a[i], a[j] = a[j], a[i]
					b[i], b[j] = b[j], b[i]
				}
			}
			if placed {
				for _, ring := range [][]geom.Vec3{a, b} {
					for i, p := range ring {
						ring[i] = geom.Vec3{X: 2 + p.X + p.Z/2, Y: 3 + p.Y, Z: 1 + p.Z}
					}
				}
			}
			r, e := Planar(context.Background(), a, b, Options{})
			if e != nil {
				t.Fatal(e)
			}
			if r.ExactVolume.Cmp(big.NewRat(70, 1)) != 0 {
				t.Fatal(r.ExactVolume)
			}
			m, e := r.Solid.MassProperties()
			if e != nil || math.Abs(m.Volume-70) > 1e-10 {
				t.Fatal(m, e)
			}
			top := r.Solid.Topology()
			if top.Shells != 1 || top.EulerCharacteristic != 2 {
				t.Fatal(top)
			}
			for i := range a {
				if r.Solid.Vertices()[r.LowerVertices[i]] != a[i] || r.Solid.Vertices()[r.UpperVertices[i]] != b[i] {
					t.Fatal("vertex provenance")
				}
				f := r.Solid.Faces()[r.SideFaces[i]]
				if len(f.Boundary) != 4 {
					t.Fatal("side triangulated")
				}
				found := false
				for _, u := range f.Boundary {
					if u.Edge == r.LowerEdges[i] {
						found = true
					}
				}
				if !found {
					t.Fatal("edge provenance")
				}
			}
		}
	}
}
func TestPlanarRejectsAndLimits(t *testing.T) {
	a, b := rings()
	bad := append([]geom.Vec3(nil), b...)
	bad[1].X += .125
	if r, e := Planar(context.Background(), a, bad, Options{}); e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("nonplanar side accepted", e)
	}
	bad = append([]geom.Vec3(nil), b...)
	bad[1], bad[3] = bad[3], bad[1]
	if _, e := Planar(context.Background(), a, bad, Options{}); e == nil {
		t.Fatal("crossed correspondence")
	}
	if _, e := Planar(context.Background(), a, a, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if _, e := Planar(context.Background(), a, b, Options{MaxVertices: 3}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e := Planar(context.Background(), a, b, Options{Support: support.Options{MaxWork: 1}}); !errors.Is(e, support.ErrLimit) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Planar(ctx, a, b, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
