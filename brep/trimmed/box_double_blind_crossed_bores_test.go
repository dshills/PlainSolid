package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoxDoubleBlindCrossedBores(t *testing.T) {
	for ni, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, xo := range []bool{false, true} {
			for _, zo := range []bool{false, true} {
				f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
				dim, c := geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}
				xd, zd := 13.2, 9.2
				r, e := BoxDoubleBlindCrossedBores(context.Background(), f, dim, c, 2, xd, zd, xo, zo, 1e-9)
				if e != nil {
					t.Fatal(n, xo, zo, e)
				}
				m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				common := 128. / 3
				vx, vz := 4*math.Pi*xd, 4*math.Pi*zd
				v := 3840 - vx - vz + common
				a := 1504 + 4*math.Pi*(xd+zd) - 64
				mx, mz := xd/2, zd/2
				if xo {
					mx = 20 - mx
				}
				if zo {
					mz = 12 - mz
				}
				centroid := geom.Vec3{X: (3840*10 - vx*mx - vz*10 + common*10) / v, Y: 8, Z: (3840*6 - vx*6 - vz*mz + common*6) / v}
				if math.Abs(m.Volume-v) > 1e-7 || math.Abs(m.Area-a) > 1e-7 || m.Centroid.Distance(f.ToWorld(centroid)) > 1e-8 || math.Abs(r.RemovedVolume-(3840-v)) > 1e-8 || math.Abs(r.SurfaceAreaDelta-(a-1504)) > 1e-8 {
					t.Fatal(n, xo, zo, m, v, a)
				}
				for _, site := range []struct {
					id   int
					axis geom.Vec3
					op   bool
				}{{6, f.ZAxis(), zo}, {15, f.XAxis(), xo}} {
					floor := r.Solid.Faces()[site.id]
					plane, ok := floor.Surface.(geom.PlaneSurface)
					want := site.axis.Scale(-1)
					if site.op {
						want = site.axis
					}
					if !ok || plane.Frame().ZAxis() != want || len(floor.Holes) != 0 {
						t.Fatal("floor", site)
					}
				}
				mesh, e := r.Solid.TessellateBlindCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxSubdivisions: 128, MaxTriangles: 100000})
				if e != nil {
					t.Fatal(e)
				}
				if e = mesh.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				if ni == 0 {
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
	}
}
func TestBoxDoubleBlindCrossedBoresRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, op := range []bool{false, true} {
		for _, d := range []float64{0, 12, math.Nextafter(12, 13), 20, math.NaN()} {
			if _, e := BoxDoubleBlindCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, d, 9, op, false, 1e-9); e == nil {
				t.Fatal(d, op)
			}
		}
	}
}
