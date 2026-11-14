package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func crossedDisjointSites() []CardinalThroughBore {
	return []CardinalThroughBore{{Axis: 2, Center: geom.Vec3{X: 8, Y: 8}, Radius: 2}, {Axis: 1, Center: geom.Vec3{X: 32, Z: 8}, Radius: 2}, {Axis: 0, Center: geom.Vec3{Y: 32, Z: 32}, Radius: 2}}
}
func TestBoxCrossedDisjointBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		d, c := geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 20, Z: 20}
		sites := crossedDisjointSites()
		copySites := append([]CardinalThroughBore(nil), sites...)
		r, e := BoxCrossedBoresWithDisjoint(context.Background(), f, d, c, 3, sites, 1e-9)
		if e != nil {
			t.Fatal(n, e)
		}
		if !reflect.DeepEqual(sites, copySites) {
			t.Fatal("mutated caller recipe")
		}
		m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		removed, delta := 1200*math.Pi-144, 900*math.Pi-144
		if math.Abs(m.Volume-(64000-removed)) > 2e-7 || math.Abs(m.Area-(9600+delta)) > 2e-7 || m.Centroid.Distance(f.ToWorld(c)) > 1e-8 || math.Abs(r.RemovedVolume-removed) > 1e-8 || math.Abs(r.SurfaceAreaDelta-delta) > 1e-8 {
			t.Fatal(n, m, r.RemovedVolume, r.SurfaceAreaDelta)
		}
		original, e := BoxCrossedBores(context.Background(), f, d, c, 3, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		old, now := original.Solid.Faces(), r.Solid.Faces()
		if len(now) != 34 {
			t.Fatal(len(now))
		}
		for i := 0; i < 22; i++ {
			if !reflect.DeepEqual(old[i].Outer, now[i].Outer) || !reflect.DeepEqual(old[i].Surface, now[i].Surface) {
				t.Fatal("changed retained support", i)
			}
		}
		for i, want := range []int{2, 2, 1, 2, 1, 2} {
			if len(now[i].Holes) != want {
				t.Fatal("cap", i, len(now[i].Holes), want)
			}
		}
		sites[0], sites[2] = sites[2], sites[0]
		other, e := BoxCrossedBoresWithDisjoint(context.Background(), f, d, c, 3, sites, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(r.Solid.Vertices(), other.Solid.Vertices()) || !reflect.DeepEqual(r.Solid.Edges(), other.Solid.Edges()) || !reflect.DeepEqual(now, other.Solid.Faces()) {
			t.Fatal("input order changed graph")
		}
		mesh, e := r.Solid.TessellateCrossedWithDisjoint(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(n, e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if math.Abs(mesh.SignedVolume()-(64000-removed)) > removed*.005 {
			t.Fatal("mesh removal", mesh.SignedVolume(), removed)
		}
		if n == (geom.Vec3{Z: 1}) {
			for _, binary := range []bool{false, true} {
				candidate := mesh
				candidate.Vertices = append([]geom.Vec3(nil), mesh.Vertices...)
				if binary {
					for i, p := range candidate.Vertices {
						candidate.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
					}
				}
				if e = candidate.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				rep, e := collision.SelfIntersectionsBVHContext(context.Background(), candidate, collision.Options{})
				if e != nil || !rep.Complete || len(rep.Contacts) > 0 {
					t.Fatal(binary, e, rep)
				}
			}
		}
	}
}
func TestBoxCrossedDisjointBoresLimits(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	d, c := geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 20, Z: 20}
	cases := [][]CardinalThroughBore{nil, {{Axis: 3, Radius: 1}}, {{Axis: 2, Center: geom.Vec3{X: 2, Y: 8}, Radius: 2}}, {{Axis: 2, Center: geom.Vec3{X: 8, Y: 8, Z: 1}, Radius: 2}}, {{Axis: 1, Center: geom.Vec3{X: 20, Z: 20}, Radius: 2}}, {{Axis: 0, Center: geom.Vec3{Y: 23, Z: 23}, Radius: 1}}, {{Axis: 2, Center: geom.Vec3{X: 8, Y: 8}, Radius: 2}, {Axis: 2, Center: geom.Vec3{X: 8, Y: 8}, Radius: 2}}, make([]CardinalThroughBore, 17)}
	for _, sites := range cases {
		if _, e := BoxCrossedBoresWithDisjoint(context.Background(), f, d, c, 3, sites, 1e-9); e == nil {
			t.Fatal("accepted", sites)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxCrossedBoresWithDisjoint(ctx, f, d, c, 3, crossedDisjointSites(), 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestBoxCrossedDisjointBoresSixteen(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	sites := make([]CardinalThroughBore, 16)
	for i := range sites {
		sites[i] = CardinalThroughBore{Axis: 2, Center: geom.Vec3{X: 10 + 8*float64(i), Y: 10}, Radius: 1}
	}
	r, e := BoxCrossedBoresWithDisjoint(context.Background(), f, geom.Vec3{X: 200, Y: 200, Z: 40}, geom.Vec3{X: 100, Y: 100, Z: 20}, 3, sites, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) != 86 {
		t.Fatal(len(r.Solid.Faces()))
	}
}

func TestBoxCrossedDisjointSingleAndBudget(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Y: -1})
	site := CardinalThroughBore{Axis: 1, Center: geom.Vec3{X: 32, Z: 8}, Radius: 1.5}
	r, e := BoxCrossedBoresWithDisjoint(context.Background(), f, geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 20, Z: 20}, 3, []CardinalThroughBore{site}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) != 26 {
		t.Fatal(len(r.Solid.Faces()))
	}
	m, e := r.Solid.TessellateCrossedWithDisjoint(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Solid.TessellateCrossedWithDisjoint(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 2}); e == nil {
		t.Fatal("budget")
	}
}
