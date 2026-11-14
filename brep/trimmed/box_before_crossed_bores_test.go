package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func beforeCrossOracle(r, h, dx, depth float64) (removed, delta, floor float64) {
	a := -h
	q := math.Sqrt((r - a) * (r + a))
	theta := math.Acos(a / r)
	common := 2*q*q*q/3 - 2*r*r*a*theta + 2*r*r*q
	removed = math.Pi*r*r*(dx+depth) - common
	ax := 4 * r * q
	az := 4 * r * (q - a*theta)
	floor = 2 * (r*r*math.Asin(a/r) - q*a)
	delta = 2*math.Pi*r*(dx+depth) - ax - az - 3*math.Pi*r*r + floor
	return
}
func TestBoxBeforeCrossedBores(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, h := range []float64{-.5, -1, -1.2, -1.5} {
			f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
			r, e := BoxBeforeCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 6+h, 1e-9)
			if e != nil {
				t.Fatal(n, h, e)
			}
			removed, delta, floor := beforeCrossOracle(2, h, 20, 6+h)
			if math.Abs(r.RemovedVolume-removed) > 1e-7 || math.Abs(r.SurfaceAreaDelta-delta) > 1e-7 {
				t.Fatal(n, h, r.RemovedVolume, removed, r.SurfaceAreaDelta, delta)
			}
			sum := 0.
			for _, id := range []int{24, 25} {
				a, e := r.Solid.PlanarFaceArea(id)
				if e != nil {
					t.Fatal(e)
				}
				sum += a
				ff := r.Solid.Faces()[id]
				ps, ok := ff.Surface.(geom.PlaneSurface)
				if !ok || ps.Frame().ZAxis() != f.ZAxis().Scale(-1) || len(ff.Holes) != 0 {
					t.Fatal("floor", id)
				}
				for _, u := range ff.Outer {
					owners := 0
					for _, other := range r.Solid.Faces() {
						for _, v := range other.Outer {
							if v.Edge == u.Edge {
								owners++
							}
						}
					}
					if owners != 2 {
						t.Fatal("floor incidence", id, u.Edge, owners)
					}
				}
			}
			if math.Abs(sum-floor) > 1e-8 {
				t.Fatal("floor area", sum, floor)
			}
			if h == -1 {
				m, e := r.Solid.TessellateBeforeCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxSubdivisions: 128, MaxTriangles: 100000})
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
	}
}
func TestBoxBeforeCrossedBoresRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, d := range []float64{0, 4, math.Nextafter(4, 5), 6, math.Nextafter(6, 5), 7, math.NaN()} {
		if _, e := BoxBeforeCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, d, 1e-9); e == nil {
			t.Fatal(d)
		}
	}
}
func TestBoxBeforeCrossedBoresFineAndBudget(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Y: -1})
	r, e := BoxBeforeCrossedBores(context.Background(), f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 4.8, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.TessellateBeforeCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Solid.TessellateBeforeCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxTriangles: 2}); e == nil {
		t.Fatal("budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = BoxBeforeCrossedBores(ctx, f, geom.Vec3{X: 20, Y: 16, Z: 12}, geom.Vec3{X: 10, Y: 8, Z: 6}, 2, 5, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
