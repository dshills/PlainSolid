package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestHollowPortsThreeIndependentVolume(t *testing.T) {
	ctx := context.Background()
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	var cuts []EdgeChamfer
	vs := box.Vertices()
	for i, ed := range box.Edges() {
		a, b := vs[ed.Start], vs[ed.End]
		if a.X != b.X || a.Y != b.Y || a.Z == b.Z || (a.X == 4 && a.Y == 4) {
			continue
		}
		face := -1
		for j, f := range box.Faces() {
			for _, u := range f.Boundary {
				if u.Edge == i {
					face = j
					break
				}
			}
			if face >= 0 {
				break
			}
		}
		cuts = append(cuts, EdgeChamfer{Edge: i, Face: face, Setback: 1, OtherSetback: 1})
	}
	stock, e := Chamfer(ctx, box, cuts, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	ports := append([]int(nil), stock.EdgeFaces...)
	thick := make([]float64, len(stock.Solid.Faces()))
	for i := range thick {
		thick[i] = .25
	}
	for _, id := range ports {
		thick[id] = 0
	}
	r, e := HollowPorts(ctx, stock.Solid, thick, PortHollowOptions{OpenFaces: ports})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExactVolume.Cmp(big.NewRat(263, 16)) != 0 || r.Interior.ExactVolume.Cmp(big.NewRat(665, 16)) != 0 {
		t.Fatal(r.ExactVolume, r.Interior.ExactVolume)
	}
	m, e := r.Solid.MassProperties()
	if e != nil || math.Abs(m.Volume-263.0/16) > 1e-10 {
		t.Fatal(m, e)
	}
	top := r.Solid.Topology()
	if top.Shells != 1 || top.EulerCharacteristic != -2 {
		t.Fatal(top)
	}
	for i, id := range ports {
		if r.OpenFaces[i] != id || r.InteriorOpeningFaces[i] != r.Interior.OldToNew[id] {
			t.Fatal("ancestry")
		}
	}
	r.OpenFaces[0] = -1
	if ports[0] < 0 {
		t.Fatal("alias")
	}
	for _, bad := range [][]int{{ports[0], ports[0]}, {ports[0], -1}} {
		out, e := HollowPorts(ctx, stock.Solid, thick, PortHollowOptions{OpenFaces: bad})
		if !errors.Is(e, ErrInvalid) || len(out.Solid.Faces()) != 0 {
			t.Fatal(e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := HollowPorts(canceled, stock.Solid, thick, PortHollowOptions{OpenFaces: ports}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := HollowPorts(ctx, stock.Solid, thick, PortHollowOptions{OpenFaces: ports, Reconstruction: Options{MaxWork: 1}}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}

func TestHollowPortsRejectAdjacentAndCount(t *testing.T) {
	s, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	thick := []float64{0, 0, 0, .25, .25, .25}
	r, e := HollowPorts(context.Background(), s, thick, PortHollowOptions{OpenFaces: []int{0, 1, 2}})
	if !errors.Is(e, ErrUnsupported) || len(r.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
	_, e = HollowPorts(context.Background(), s, thick, PortHollowOptions{OpenFaces: make([]int, 33)})
	if !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
