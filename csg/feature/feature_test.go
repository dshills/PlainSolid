package feature

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"testing"
)

func fixture(t *testing.T) (brep.Solid, []geom.Vec3) {
	t.Helper()
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	return s, []geom.Vec3{{X: 1, Y: 1, Z: 4}, {X: 3, Y: 1, Z: 4}, {X: 3, Y: 3, Z: 4}, {X: 1, Y: 3, Z: 4}}
}
func TestFeatureMassAndHistory(t *testing.T) {
	for _, kind := range []Kind{Protrusion, Pocket} {
		t.Run([]string{"protrusion", "pocket"}[kind], func(t *testing.T) {
			s, p := fixture(t)
			dz := 1.
			want := 68.
			z := (128. + 18.) / 68
			if kind == Pocket {
				dz = -1
				want = 60
				z = (128. - 14.) / 60
			}
			r, e := Extrude(context.Background(), s, 1, p, geom.Vec3{Z: dz}, kind, Options{})
			if e != nil {
				t.Fatal(e)
			}
			m, e := r.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(m.Volume-want) > 1e-11 || math.Abs(m.SurfaceArea-104) > 1e-11 || m.Centroid.Sub(geom.Vec3{X: 2, Y: 2, Z: z}).Length() > 1e-11 {
				t.Fatal(m)
			}
			top := r.Solid.Topology()
			if top.Shells != 1 || top.EulerCharacteristic != 2 {
				t.Fatal(top)
			}
			if len(r.SketchWalls) != 4 || len(r.History.Edges) != len(r.Solid.Edges()) {
				t.Fatal("history")
			}
			toolFaces := map[int]bool{}
			for _, src := range r.History.Faces {
				if src.Operand == poly.OperandB {
					toolFaces[src.Face] = true
					if src.Reversed != (kind == Pocket) {
						t.Fatal("tool orientation")
					}
				}
			}
			if toolFaces[r.Mouth] || !toolFaces[r.Terminal] || len(toolFaces) != 5 {
				t.Fatal(toolFaces)
			}
			for _, wall := range r.SketchWalls {
				if !toolFaces[wall] {
					t.Fatal("wall missing")
				}
			}
		})
	}
}
func TestConcaveObliqueReversedSketch(t *testing.T) {
	s, _ := fixture(t)
	p := []geom.Vec3{{X: 1, Y: 1, Z: 4}, {X: 3, Y: 1, Z: 4}, {X: 3, Y: 2, Z: 4}, {X: 2, Y: 2, Z: 4}, {X: 2, Y: 3, Z: 4}, {X: 1, Y: 3, Z: 4}}
	for _, kind := range []Kind{Protrusion, Pocket} {
		d := geom.Vec3{X: .25, Y: .125, Z: 1}
		want := 67.
		if kind == Pocket {
			d.Z = -1
			want = 61
		}
		for pass := 0; pass < 2; pass++ {
			r, e := Extrude(context.Background(), s, 1, p, d, kind, Options{})
			if e != nil {
				t.Fatal(e)
			}
			m, e := r.Solid.MassProperties()
			if e != nil || math.Abs(m.Volume-want) > 1e-10 {
				t.Fatal(e, m)
			}
			for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
			}
		}
	}
}
func TestFeatureRejections(t *testing.T) {
	s, p := fixture(t)
	tests := []struct {
		name string
		p    []geom.Vec3
		d    geom.Vec3
		k    Kind
		want error
	}{{"through", p, geom.Vec3{Z: -4}, Pocket, ErrThrough}, {"side", p, geom.Vec3{X: 2, Z: -1}, Pocket, ErrThrough}, {"wrong direction", p, geom.Vec3{Z: -1}, Protrusion, ErrInvalid}, {"parallel", p, geom.Vec3{X: 1}, Protrusion, ErrInvalid}, {"nonplanar", []geom.Vec3{{X: 1, Y: 1, Z: 4}, {X: 3, Y: 1, Z: 4}, {X: 2, Y: 2, Z: 3}}, geom.Vec3{Z: 1}, Protrusion, ErrInvalid}, {"touch", []geom.Vec3{{X: 0, Y: 1, Z: 4}, {X: 2, Y: 1, Z: 4}, {X: 1, Y: 2, Z: 4}}, geom.Vec3{Z: 1}, Protrusion, ErrInvalid}, {"crossing", []geom.Vec3{p[0], p[2], p[1], p[3]}, geom.Vec3{Z: 1}, Protrusion, ErrInvalid}, {"rounding", p, geom.Vec3{Z: .1}, Protrusion, ErrPrecision}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, e := Extrude(context.Background(), s, 1, tc.p, tc.d, tc.k, Options{})
			if !errors.Is(e, tc.want) {
				t.Fatal(e)
			}
		})
	}
	for _, o := range []Options{{MaxSketchVertices: 3}, {MaxWork: 1}, {MaxBaseFaces: 2}} {
		if _, e := Extrude(context.Background(), s, 1, p, geom.Vec3{Z: 1}, Protrusion, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Extrude(ctx, s, 1, p, geom.Vec3{Z: 1}, Protrusion, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
