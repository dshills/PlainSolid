package trimmed

import (
	"context"
	"plainsolid/brep/parametric"

	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestHelicalReliefNativeDisplay(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, tube := range []bool{true, false} {
		r := HelicalGrooveRecipe{Frame: f, OuterRadius: 4, Height: 2, Lead: 1, Start: .25, End: .75, Hand: geom.RightHanded, Starts: 1}
		var s Solid
		var e error
		if tube {
			r.BoreRadius = 2
			r.Profile = []geom.Vec2{{X: 2, Y: .25}, {X: 2.5, Y: .375}, {X: 2, Y: .5}}
			x, err := GrooveTubeWithEndReliefs(context.Background(), r, HelicalReliefSpec{2.75, .125}, HelicalReliefSpec{3, .25})
			s, e = x.Solid, err
		} else {
			r.Profile = []geom.Vec2{{X: 3.5, Y: .375}, {X: 4, Y: .25}, {X: 4, Y: .5}}
			x, err := GrooveShaftWithTipRelief(context.Background(), r, 3, .125)
			s, e = x.Solid, err
		}
		if e != nil {
			t.Fatal(e)
		}
		target := .2
		if !tube {
			target = .4
		}
		checked, e := s.TessellateEmbeddedDisplay(context.Background(), EmbeddedDisplayOptions{Tessellation: parametric.TessellationOptions{Deflection: target, MaxSubdivisions: 256, MaxTriangles: 50000}})
		if e != nil {
			t.Fatal(tube, e)
		}
		m := checked.Mesh
		if checked.SignedVolume.Sign() <= 0 || !checked.Collision.Complete || len(checked.Collision.Contacts) != 0 {
			t.Fatal("incomplete display certificate")
		}
		edges := map[[2]int]bool{}
		for _, tri := range m.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}] = true
			}
		}
		want := 2
		if tube {
			want = 0
		}
		if len(m.Vertices)-len(edges)+len(m.Triangles) != want {
			t.Fatal("genus")
		}
		t.Log(tube, len(m.Vertices), len(m.Triangles), m.SignedVolume())

	}
}

func TestEmbeddedDisplayAtomicLimits(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	native, e := parametric.Cylinder(f, 1, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromParametric(native)
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []EmbeddedDisplayOptions{{Tessellation: parametric.TessellationOptions{Deflection: .2, MaxTriangles: 4}}, {Tessellation: parametric.TessellationOptions{Deflection: .2, MaxTriangles: 100001}}, {Tessellation: parametric.TessellationOptions{Deflection: .2}, Collision: collision.Options{MaxPairs: 1}}} {
		r, e := s.TessellateEmbeddedDisplay(context.Background(), o)
		if e == nil || len(r.Mesh.Vertices) != 0 || r.SignedVolume != nil {
			t.Fatal("partial", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.TessellateEmbeddedDisplay(ctx, EmbeddedDisplayOptions{Tessellation: parametric.TessellationOptions{Deflection: .2}}); e == nil {
		t.Fatal("cancel")
	}
}
