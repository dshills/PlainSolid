package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoxBlindTripleCrossedBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, op := range []bool{false, true} {
			for _, depth := range []float64{9, 9.2} {
				f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
				d, c := geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}
				radius := 2.
				r, e := BoxBlindTripleCrossedBores(context.Background(), f, d, c, radius, depth, op, 1e-9)
				if e != nil {
					t.Fatal(n, op, depth, e)
				}
				mass, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				correction := 16*radius*radius*radius - tripleCommonQuadrature(radius)
				removed := math.Pi*radius*radius*(d.X+d.Y+depth) - correction
				delta := 2*math.Pi*radius*(d.X+d.Y+depth) - 24*math.Sqrt2*radius*radius - 4*math.Pi*radius*radius
				zmid := depth / 2
				if op {
					zmid = d.Z - zmid
				}
				moment := geom.Vec3{X: 10, Y: 8, Z: 6}.Scale(math.Pi * 4 * (20 + 16)).Add(geom.Vec3{X: 10, Y: 8, Z: zmid}.Scale(math.Pi * 4 * depth)).Sub(c.Scale(correction))
				centroid := d.Scale(1920).Sub(moment).Scale(1 / (3840 - removed))
				if math.Abs(mass.Volume-(3840-removed)) > 1e-7 || math.Abs(mass.Area-(1504+delta)) > 1e-7 || mass.Centroid.Distance(f.ToWorld(centroid)) > 1e-8 || math.Abs(r.RemovedVolume-removed) > 1e-8 || math.Abs(r.SurfaceAreaDelta-delta) > 1e-8 {
					t.Fatal(n, op, depth, mass, removed, delta)
				}
				if len(r.Solid.Faces()) != 55 {
					t.Fatal(len(r.Solid.Faces()))
				}
				a, e := r.Solid.PlanarFaceArea(6)
				if e != nil || math.Abs(a-4*math.Pi) > 1e-8 {
					t.Fatal(a, e)
				}
				floor := r.Solid.Faces()[6]
				want := f.ZAxis().Scale(-1)
				if op {
					want = f.ZAxis()
				}
				if floor.Surface.(geom.PlaneSurface).Frame().ZAxis() != want || len(floor.Outer) != 8 || len(floor.Holes) != 0 {
					t.Fatal("floor")
				}
				assertTripleVertexLinks(t, r.Solid)
				if depth == 9.2 {
					mesh, e := r.Solid.TessellateTripleCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000, MaxSubdivisions: 128})
					if e != nil {
						t.Fatal(e)
					}
					if e = mesh.ValidateSolid(); e != nil {
						t.Fatal(e)
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
		}
	}
}
func TestBoxBlindTripleCrossedBoresLimits(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, op := range []bool{false, true} {
		for _, depth := range []float64{0, 8, math.Nextafter(8, 9), 12, math.NaN()} {
			if _, e := BoxBlindTripleCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, depth, op, 1e-9); e == nil {
				t.Fatal(depth, op)
			}
		}
	}
}
