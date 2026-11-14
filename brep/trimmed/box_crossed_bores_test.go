package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoxCrossedBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		r, e := BoxCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 1e-9)
		if e != nil {
			t.Fatal(n, e)
		}
		m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v := 20.*16*12 - 128*math.Pi + 128./3
		area := 2.*(20*16+20*12+16*12) + 112*math.Pi - 64
		if math.Abs(m.Volume-v) > 1e-7 || math.Abs(m.Area-area) > 1e-7 || m.Centroid.Sub(f.ToWorld(geom.Vec3{X: 10, Y: 8, Z: 6})).Length() > 1e-8 {
			t.Fatal(m, v, area)
		}
		if math.Abs(20.*16*12-r.RemovedVolume-v) > 1e-9 || math.Abs(2.*(20*16+20*12+16*12)+r.SurfaceAreaDelta-area) > 1e-9 {
			t.Fatal(r)
		}
		mesh, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
		if e != nil {
			t.Fatal("mesh", n, e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if math.Abs(mesh.SignedVolume()-v) > 0.002*v {
			t.Fatal("mesh volume", mesh.SignedVolume(), v)
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
				report, err := collision.SelfIntersectionsBVHContext(context.Background(), candidate, collision.Options{})
				if err != nil || !report.Complete || len(report.Contacts) > 0 {
					t.Fatal("embedding", binary, report, err)
				}
			}
		}

	}
}

func TestBoxCrossedBoresOffCenterAndBounds(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{X: -1})
	d, c := geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 8, Y: 7, Z: 5}
	r, e := BoxCrossedBores(context.Background(), f, d, c, 2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	stock := d.X * d.Y * d.Z
	vx, vz, intersection := 4*math.Pi*d.X, 4*math.Pi*d.Z, 128./3
	want := d.Scale(stock / 2).Sub(geom.Vec3{X: d.X / 2, Y: c.Y, Z: c.Z}.Scale(vx)).Sub(geom.Vec3{X: c.X, Y: c.Y, Z: d.Z / 2}.Scale(vz)).Add(c.Scale(intersection)).Scale(1 / (stock - vx - vz + intersection))
	if m.Centroid.Sub(f.ToWorld(want)).Length() > 1e-8 {
		t.Fatal(m.Centroid, f.ToWorld(want))
	}
	for _, p := range r.Solid.Vertices() {
		q := f.ToLocal(p)
		if q.X < 0 || q.X > d.X || q.Y < 0 || q.Y > d.Y || q.Z < 0 || q.Z > d.Z {
			t.Fatal(q)
		}
	}
	for _, o := range []parametric.TessellationOptions{{Deflection: .1, Angle: .1, MaxTriangles: 2}, {Deflection: 1e-9, Angle: 1e-9, MaxSubdivisions: 1, MaxTriangles: 200000}} {
		if _, e = r.Solid.Tessellate(context.Background(), o); e == nil {
			t.Fatal("ignored mesh budget")
		}
	}
}

func TestBoxCrossedBoresRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	d := geom.Vec3{X: 20, Y: 16, Z: 12}
	c := geom.Vec3{X: 10, Y: 8, Z: 6}
	for _, r := range []float64{0, -1, 6, math.Inf(1), math.NaN()} {
		if _, e := BoxCrossedBores(context.Background(), f, d, c, r, 1e-9); e == nil {
			t.Fatal(r)
		}
	}
	for _, q := range []geom.Vec3{{X: 2, Y: 8, Z: 6}, {X: 10, Y: 2, Z: 6}, {X: 10, Y: 8, Z: 2}, {X: 18, Y: 8, Z: 6}} {
		if _, e := BoxCrossedBores(context.Background(), f, d, q, 2, 1e-9); e == nil {
			t.Fatal(q)
		}
	}
	oblique, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{X: 1, Z: 1})
	if _, e := BoxCrossedBores(context.Background(), oblique, d, c, 2, 1e-9); e == nil {
		t.Fatal("oblique frame")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxCrossedBores(ctx, f, d, c, 2, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
