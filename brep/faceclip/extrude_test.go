package faceclip

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExtrudeHoledRegion(t *testing.T) {
	b := boundaryFor(t)
	for _, z := range []float64{2, -2} {
		r, e := ExtrudeRegion(context.Background(), b, geom.Vec3{X: .5, Z: z}, ExtrusionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := r.Solid.MassProperties()
		if e != nil || math.Abs(m.Volume-24) > 1e-10 || r.ExactVolume.Cmp(big.NewRat(24, 1)) != 0 {
			t.Fatal(e, m)
		}
		if r.Solid.Topology().EulerCharacteristic != 0 || r.Solid.Topology().Shells != 1 {
			t.Fatal("lost hole")
		}
		walls := map[[2]int]int{}
		caps := 0
		for _, src := range r.FaceSources {
			if src.Role == LoopWall {
				walls[[2]int{src.Loop, src.Segment}]++
			} else {
				caps++
			}
		}
		if caps != 2*len(r.Sheet.Mesh.Triangles) {
			t.Fatal("cap ancestry")
		}
		for _, n := range walls {
			if n != 2 {
				t.Fatal("wall ancestry")
			}
		}
	}
}
func TestExtrudeDisconnectedRegions(t *testing.T) {
	b := Boundary{Axis: 2, ProjectedArea: big.NewRat(2, 1)}
	for _, x := range []float64{0, 3} {
		l := Loop{Parent: -1}
		for _, p := range []geom.Vec3{{X: x}, {X: x + 1}, {X: x + 1, Y: 1}, {X: x, Y: 1}} {
			l.Vertices = append(l.Vertices, vec(p))
		}
		b.Loops = append(b.Loops, l)
	}
	r, e := ExtrudeRegion(context.Background(), b, geom.Vec3{Z: 1}, ExtrusionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Solid.Topology().Shells != 2 || r.ExactVolume.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("components")
	}
}
func TestExtrudeStrictConversion(t *testing.T) {
	b := boundaryFor(t)
	if _, e := ExtrudeRegion(context.Background(), b, geom.Vec3{Z: .1}, ExtrusionOptions{}); !errors.Is(e, ErrConversion) {
		t.Fatal(e)
	}
	if _, e := ExtrudeRegion(context.Background(), b, geom.Vec3{X: 1}, ExtrusionOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	r, e := Clip(context.Background(), holed(t), 1, Halfspace{geom.Vec3{X: 3}, 1}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	rational, e := MergeLoops(context.Background(), r, LoopOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ExtrudeRegion(context.Background(), rational, geom.Vec3{Z: 1}, ExtrusionOptions{}); !errors.Is(e, ErrConversion) {
		t.Fatal(e)
	}
	if _, e = ExtrudeRegion(context.Background(), b, geom.Vec3{Z: 1}, ExtrusionOptions{Sheet: SheetOptions{MaxTriangles: 20}}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
