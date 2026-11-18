package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func unequalCrossedOracle(a, b float64) (volume, smallArea, largeArea float64) {
	const n = 4096
	step := math.Pi / 2 / n
	for i := 0; i <= n; i++ {
		u := float64(i) * step
		c := math.Cos(u)
		h := math.Sqrt(b*b - a*a*math.Sin(u)*math.Sin(u))
		w := 2.
		if i%2 == 1 {
			w = 4
		}
		if i == 0 || i == n {
			w = 1
		}
		volume += w * c * c * h
		smallArea += w * h
		largeArea += w * c * c / h
	}
	return volume * step / 3 * 8 * a * a, smallArea * step / 3 * 8 * a, largeArea * step / 3 * 8 * b * a * a
}
func TestBoxUnequalCrossedBores(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	d, c := geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 12, Y: 10, Z: 8}
	result, e := BoxUnequalCrossedBores(context.Background(), f, d, c, 5, 3, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	mass, e := result.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	intersection, s1, s2 := unequalCrossedOracle(3, 5)
	volume := d.X*d.Y*d.Z - math.Pi*(25*d.X+9*d.Z) + intersection
	area := 2*(d.X*d.Y+d.X*d.Z+d.Y*d.Z) - 2*math.Pi*(25+9) + 2*math.Pi*(5*d.X+3*d.Z) - s1 - s2
	if math.Abs(mass.Volume-volume) > 1e-6 || math.Abs(mass.Area-area) > 1e-6 || mass.Centroid.Distance(c) > 1e-7 {
		t.Fatal(mass, volume, area)
	}
	mesh, e := result.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
	if e != nil {
		t.Fatal("mesh", e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	t.Log(len(mesh.Triangles), mass.Volume, mass.Area)
}

func TestBoxUnequalCrossedBoresSignedAndSwapped(t *testing.T) {
	slowtest.Skip(t)
	d, c := geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 11, Y: 9, Z: 7}
	for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		for _, radii := range [][2]float64{{5, 3}, {3, 5}, {3, 2}} {
			r, e := BoxUnequalCrossedBores(context.Background(), f, d, c, radii[0], radii[1], 1e-9)
			if e != nil {
				t.Fatal(n, radii, e)
			}
			m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			intersection, s1, s2 := unequalCrossedOracle(min(radii[0], radii[1]), max(radii[0], radii[1]))
			stock := d.X * d.Y * d.Z
			vx, vz := math.Pi*radii[0]*radii[0]*d.X, math.Pi*radii[1]*radii[1]*d.Z
			volume := stock - vx - vz + intersection
			area := 2*(d.X*d.Y+d.X*d.Z+d.Y*d.Z) - 2*math.Pi*(radii[0]*radii[0]+radii[1]*radii[1]) + 2*math.Pi*(radii[0]*d.X+radii[1]*d.Z) - s1 - s2
			center := d.Scale(stock / 2).Sub(geom.Vec3{X: d.X / 2, Y: c.Y, Z: c.Z}.Scale(vx)).Sub(geom.Vec3{X: c.X, Y: c.Y, Z: d.Z / 2}.Scale(vz)).Add(c.Scale(intersection)).Scale(1 / volume)
			if math.Abs(m.Volume-volume) > 1e-6 || math.Abs(m.Area-area) > 1e-6 || m.Centroid.Distance(f.ToWorld(center)) > 1e-7 || math.Abs(r.RemovedVolume-(stock-volume)) > 1e-6 || math.Abs(r.SurfaceAreaDelta-(area-2*(d.X*d.Y+d.X*d.Z+d.Y*d.Z))) > 1e-6 {
				t.Fatal(n, radii, m, volume, area, center)
			}
			mesh, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
			if e != nil {
				t.Fatal(n, radii, e)
			}
			if e = mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			if math.Abs(mesh.SignedVolume()-volume) > 0.002*volume {
				t.Fatal("mesh volume", mesh.SignedVolume(), volume)
			}
		}
	}
}

func TestBoxUnequalCrossedBoresRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	d, c := geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 12, Y: 10, Z: 8}
	for _, r := range [][2]float64{{3, 3}, {3, math.Nextafter(3, 4)}, {0, 3}, {3, -1}, {math.Inf(1), 3}, {3, math.NaN()}, {8, 3}} {
		if _, e := BoxUnequalCrossedBores(context.Background(), f, d, c, r[0], r[1], 1e-9); e == nil {
			t.Fatal(r)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxUnequalCrossedBores(ctx, f, d, c, 5, 3, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
	r, e := BoxUnequalCrossedBores(context.Background(), f, d, c, 5, 3, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxTriangles: 2}); e == nil {
		t.Fatal("ignored mesh budget")
	}
}

func TestBoxUnequalCrossedBoresEmbedding(t *testing.T) {
	slowtest.Skip(t)
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Z: 1})
	r, e := BoxUnequalCrossedBores(context.Background(), f, geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 11, Y: 9, Z: 7}, 3, 2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
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
		if e = candidate.ValidateSolid(); e != nil {
			t.Fatal(binary, e)
		}
		report, e := collision.SelfIntersectionsBVHContext(context.Background(), candidate, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) > 0 {
			t.Fatal(binary, report, e)
		}
	}
}

func TestUnequalBoreTrimPairedSupportAndDerivative(t *testing.T) {
	sf, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	lf, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	small, _ := geom.NewCylinderSurface(sf, 2)
	large, _ := geom.NewCylinderSurface(lf, 3)
	for _, sign := range []float64{-1, 1} {
		for quarter := 0; quarter < 4; quarter++ {
			s := unequalBoreTrim{a: 2, b: 3, sign: sign, from: float64(quarter) * math.Pi / 2, to: float64(quarter+1) * math.Pi / 2}
			l := s
			l.large = true
			for i := 0; i <= 16; i++ {
				t0 := float64(i) / 16
				uv, e := s.raw(t0)
				if e != nil {
					t.Fatal(e)
				}
				xy, e := l.raw(t0)
				if e != nil {
					t.Fatal(e)
				}
				p, e := small.Eval(uv.X, uv.Y)
				if e != nil {
					t.Fatal(e)
				}
				q, e := large.Eval(xy.X, xy.Y)
				if e != nil || p.Distance(q) > 1e-12 {
					t.Fatal("paired support", p, q, e)
				}
				ds, e := s.DerivativeUV(t0)
				if e != nil {
					t.Fatal(e)
				}
				dl, e := l.DerivativeUV(t0)
				if e != nil {
					t.Fatal(e)
				}
				su, sv, e := small.Partials(uv.X, uv.Y)
				if e != nil {
					t.Fatal(e)
				}
				lu, lv, e := large.Partials(xy.X, xy.Y)
				if e != nil {
					t.Fatal(e)
				}
				if su.Scale(ds.X).Add(sv.Scale(ds.Y)).Distance(lu.Scale(dl.X).Add(lv.Scale(dl.Y))) > 1e-11 {
					t.Fatal("paired tangent")
				}
				for _, pair := range []struct {
					c unequalBoreTrim
					p geom.Vec2
				}{{s, uv}, {l, xy}} {
					bounds, e := pair.c.BoundsUV()
					if e != nil || pair.p.X < bounds.Min.X || pair.p.X > bounds.Max.X || pair.p.Y < bounds.Min.Y || pair.p.Y > bounds.Max.Y {
						t.Fatal("bounds", pair, bounds, e)
					}
				}
			}
		}
	}
}

func TestBoxUnequalCrossedBoresAdaptive(t *testing.T) {
	slowtest.Skip(t)
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Z: 1})
	r, e := BoxUnequalCrossedBores(context.Background(), f, geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 11, Y: 9, Z: 7}, 4, 2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []parametric.TessellationOptions{{Deflection: .5, Angle: .25, MaxTriangles: 100000}, {Deflection: .5, Angle: .05, MaxTriangles: 100000}, {Deflection: .1, Angle: .12, MaxTriangles: 100000}} {
		m, e := r.Solid.TessellateUnequalCrossedBores(context.Background(), o)
		if e != nil {
			t.Fatal(o, e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		t.Log(o.Deflection, o.Angle, len(m.Triangles))
		if o.Angle == .05 {
			for _, binary := range []bool{false, true} {
				candidate := m
				candidate.Vertices = append([]geom.Vec3(nil), m.Vertices...)
				if binary {
					for i, p := range candidate.Vertices {
						candidate.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
					}
				}
				if e = candidate.ValidateSolid(); e != nil {
					t.Fatal(binary, e)
				}
				report, e := collision.SelfIntersectionsBVHContext(context.Background(), candidate, collision.Options{})
				if e != nil || !report.Complete || len(report.Contacts) > 0 {
					t.Fatal("adaptive embedding", binary, report, e)
				}
			}
		}

	}
	if _, e = r.Solid.TessellateUnequalCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxTriangles: 2}); e == nil {
		t.Fatal("adaptive triangle budget")
	}
	if _, e = r.Solid.TessellateUnequalCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, AnalyticBound: true}); e != parametric.ErrDeflectionUnsupported {
		t.Fatal("unsupported certification", e)
	}
	if _, e = r.Solid.TessellateHelical(context.Background(), parametric.TessellationOptions{Deflection: .1}); e == nil {
		t.Fatal("helical eligibility widened")
	}

}
