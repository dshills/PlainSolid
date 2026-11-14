package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoxBlindCrossedBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, op := range []bool{false, true} {
			f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			r, e := BoxBlindCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 9, op, 1e-9)
			if e != nil {
				t.Fatal(n, op, e)
			}
			m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			v := 3840. - 116*math.Pi + 128./3
			area := 1504. + 108*math.Pi - 64
			if math.Abs(m.Volume-v) > 1e-7 || math.Abs(m.Area-area) > 1e-7 {
				t.Fatal(m, v, area)
			}
			blindCenter := 4.5
			if op {
				blindCenter = 7.5
			}
			cz := (3840*6 - (80*math.Pi*6 + 36*math.Pi*blindCenter - 128.0/3*6)) / v
			if m.Centroid.Distance(f.ToWorld(geom.Vec3{X: 10, Y: 8, Z: cz})) > 1e-8 {
				t.Fatal("centroid", m.Centroid, cz)
			}
			floor := r.Solid.Faces()[BlindCrossedBoreFloorFace]
			ps, ok := floor.Surface.(geom.PlaneSurface)
			if !ok || len(floor.Holes) != 0 {
				t.Fatal("floor")
			}
			want := f.ZAxis().Scale(-1)
			if op {
				want = f.ZAxis()
			}
			if ps.Frame().ZAxis() != want {
				t.Fatal("floor normal")
			}
			mesh, e := r.Solid.TessellateBlindCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
			if e != nil {
				t.Fatal("mesh", e)
			}
			if e = mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestBoxBlindCrossedBoresLimits(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	d, c := geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}
	for _, op := range []bool{false, true} {
		for _, depth := range []float64{0, 8, math.Nextafter(8, 9), 12, 13, math.NaN(), math.Inf(1)} {
			if _, e := BoxBlindCrossedBores(context.Background(), f, d, c, 2, depth, op, 1e-9); e == nil {
				t.Fatal("accepted", depth, op)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxBlindCrossedBores(ctx, f, d, c, 2, 9, false, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestBoxBlindCrossedBoresEmbedding(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{X: -1})
	r, e := BoxBlindCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 9.25, true, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.TessellateBlindCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxSubdivisions: 128, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	for _, binary := range []bool{false, true} {
		candidate := m
		candidate.Vertices = append([]geom.Vec3(nil), m.Vertices...)
		if binary {
			for i, p := range candidate.Vertices {
				candidate.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
			}
		}
		if e := candidate.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		report, e := collision.SelfIntersectionsBVHContext(context.Background(), candidate, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) > 0 {
			t.Fatal(binary, report, e)
		}
	}
	if _, e = r.Solid.TessellateBlindCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 2}); e == nil {
		t.Fatal("budget")
	}
}
