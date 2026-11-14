package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func skewCrossedOracle(a, b, d float64) (v, my, s1, s2 float64) {
	const n = 4096
	step := math.Pi / n
	for i := 0; i <= n; i++ {
		u := -math.Pi/2 + float64(i)*step
		co, si := math.Cos(u), math.Sin(u)
		y := d + a*si
		h := math.Sqrt(b*b - y*y)
		w := 2.
		if i%2 == 1 {
			w = 4
		}
		if i == 0 || i == n {
			w = 1
		}
		v += w * co * co * h
		my += w * y * co * co * h
	}
	v *= step / 3 * 4 * a * a
	my *= step / 3 * 4 * a * a
	step = 2 * math.Pi / n
	for i := 0; i <= n; i++ {
		u := float64(i) * step
		y := d + a*math.Sin(u)
		h := math.Sqrt(b*b - y*y)
		w := 2.
		if i%2 == 1 {
			w = 4
		}
		if i == 0 || i == n {
			w = 1
		}
		s1 += w * h
		s2 += w * math.Cos(u) * math.Cos(u) / h
	}
	s1 *= step / 3 * 2 * a
	s2 *= step / 3 * 2 * b * a * a
	return
}
func TestBoxSkewCrossedBores(t *testing.T) {
	dims, c := geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 11, Y: 9, Z: 7}
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, axis)
		for _, offset := range []float64{-1, 1, .3} {
			for _, radii := range [][2]float64{{4, 2}, {2, 4}} {
				result, e := BoxSkewCrossedBores(context.Background(), f, dims, c, radii[0], radii[1], offset, 1e-9)
				if e != nil {
					t.Fatal(axis, offset, radii, e)
				}
				mass, e := result.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil {
					t.Fatal(e)
				}
				d, largeY := offset, c.Y
				if radii[0] < radii[1] {
					d = -offset
					largeY = c.Y + offset
				}
				iv, iy, s1, s2 := skewCrossedOracle(2, 4, d)
				stock := dims.X * dims.Y * dims.Z
				vx, vz := math.Pi*radii[0]*radii[0]*dims.X, math.Pi*radii[1]*radii[1]*dims.Z
				volume := stock - vx - vz + iv
				area := 2*(dims.X*dims.Y+dims.X*dims.Z+dims.Y*dims.Z) - 2*math.Pi*(radii[0]*radii[0]+radii[1]*radii[1]) + 2*math.Pi*(radii[0]*dims.X+radii[1]*dims.Z) - s1 - s2
				moment := dims.Scale(stock / 2).Sub(geom.Vec3{X: dims.X / 2, Y: c.Y, Z: c.Z}.Scale(vx)).Sub(geom.Vec3{X: c.X, Y: c.Y + offset, Z: dims.Z / 2}.Scale(vz)).Add(geom.Vec3{X: c.X * iv, Y: largeY*iv + iy, Z: c.Z * iv})
				center := f.ToWorld(moment.Scale(1 / volume))
				if math.Abs(mass.Volume-volume) > 1e-6 || math.Abs(mass.Area-area) > 1e-6 || mass.Centroid.Distance(center) > 1e-7 || math.Abs(result.RemovedVolume-(stock-volume)) > 1e-6 {
					t.Fatal(axis, offset, radii, mass, volume, area, center)
				}
				if offset == 1 {
					m, e := result.Solid.TessellateUnequalCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000})
					if e != nil {
						t.Fatal(axis, radii, e)
					}
					if e = m.ValidateSolid(); e != nil {
						t.Fatal(e)
					}
					if math.Abs(m.SignedVolume()-volume) > .002*volume {
						t.Fatal("mesh volume", m.SignedVolume(), volume)
					}
				}
			}
		}
	}
}
func TestBoxSkewCrossedBoresEmbedding(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Z: 1})
	r, e := BoxSkewCrossedBores(context.Background(), f, geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 11, Y: 9, Z: 7}, 4, 2, .3, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.TessellateUnequalCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000})
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
		report, e := collision.SelfIntersectionsBVHContext(context.Background(), candidate, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) > 0 {
			t.Fatal(binary, report, e)
		}
	}
}

func TestBoxSkewCrossedBoresRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	dims, c := geom.Vec3{X: 24, Y: 20, Z: 16}, geom.Vec3{X: 11, Y: 9, Z: 7}
	for _, d := range []float64{-2, 2, math.Nextafter(2, 0), 3, math.Inf(1), math.NaN()} {
		if _, e := BoxSkewCrossedBores(context.Background(), f, dims, c, 4, 2, d, 1e-9); e == nil {
			t.Fatal(d)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxSkewCrossedBores(ctx, f, dims, c, 4, 2, 1, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e := BoxSkewCrossedBores(context.Background(), f, dims, geom.Vec3{X: 4, Y: 9, Z: 7}, 4, 2, 1, 1e-9); e == nil {
		t.Fatal("stock clearance")
	}
	r, e := BoxSkewCrossedBores(context.Background(), f, dims, c, 4, 2, 1, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Solid.TessellateUnequalCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 2}); e == nil {
		t.Fatal("mesh budget")
	}
}
func TestSkewBoreTrimPairedSupportAndDerivative(t *testing.T) {
	lf, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	large, _ := geom.NewCylinderSurface(lf, 4)
	for _, offset := range []float64{-1, .3, 1} {
		sf, _ := geom.FrameFromNormal(geom.Vec3{Y: offset}, geom.Vec3{Z: 1})
		small, _ := geom.NewCylinderSurface(sf, 2)
		for _, sign := range []float64{-1, 1} {
			for quarter := 0; quarter < 4; quarter++ {
				s := unequalBoreTrim{a: 2, b: 4, offset: offset, sign: sign, from: float64(quarter) * math.Pi / 2, to: float64(quarter+1) * math.Pi / 2}
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
}
