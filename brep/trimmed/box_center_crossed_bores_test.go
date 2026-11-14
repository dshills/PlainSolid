package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoxCenterCrossedBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		r, e := BoxCenterCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 1e-9)
		if e != nil {
			t.Fatal(n, e)
		}
		v := 104*math.Pi - 64.0/3
		area := 104*math.Pi - 32 - 12*math.Pi
		if math.Abs(r.RemovedVolume-v) > 1e-7 || math.Abs(r.SurfaceAreaDelta-area) > 1e-7 || len(r.Solid.Faces()) != 16 {
			t.Fatal(r, v, area)
		}
		m, e := r.Solid.TessellateCenterCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(n, e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if n == (geom.Vec3{Z: 1}) {
			for _, binary := range []bool{false, true} {
				candidate := m
				candidate.Vertices = append([]geom.Vec3(nil), m.Vertices...)
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
