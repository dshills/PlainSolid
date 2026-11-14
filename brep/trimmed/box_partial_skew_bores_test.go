package trimmed

import (
	"context"
	"fmt"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func partialSkewOracle(a, b, d float64) (volume, yMoment, xArea, zArea float64) {
	const n = 4096
	l := a + b - d
	step := math.Pi / 2 / n
	for i := 0; i <= n; i++ {
		u := float64(i) * step
		s, c := math.Sin(u), math.Cos(u)
		aa, bb := 2*a-l*c*c, 2*b-l*s*s
		w := 2.
		if i%2 == 1 {
			w = 4
		}
		if i == 0 || i == n {
			w = 1
		}
		v := 8 * l * l * s * s * c * c * math.Sqrt(aa*bb)
		volume += w * v
		yMoment += w * v * (a - l*c*c)
		xArea += w * 8 * a * l * s * s * math.Sqrt(bb/aa)
		zArea += w * 8 * b * l * c * c * math.Sqrt(aa/bb)
	}
	return volume * step / 3, yMoment * step / 3, xArea * step / 3, zArea * step / 3
}
func checkPartialSkew(t *testing.T, axis geom.Vec3, a, b, d float64, display bool) {
	t.Helper()
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, axis)
	dims, c := geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 16, Z: 20}
	r, e := BoxPartialSkewBores(context.Background(), f, dims, c, a, b, d, 1e-9)
	if e != nil {
		t.Fatal(a, b, d, e)
	}
	m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	iv, iy, sx, sz := partialSkewOracle(a, b, math.Abs(d))
	sign := 1.
	if d < 0 {
		sign = -1
	}
	stock := 64000.
	vx, vz := math.Pi*a*a*40, math.Pi*b*b*40
	volume := stock - vx - vz + iv
	area := 9600. - 2*math.Pi*(a*a+b*b) + 80*math.Pi*(a+b) - sx - sz
	moment := dims.Scale(stock / 2).Sub(geom.Vec3{X: 20, Y: c.Y, Z: c.Z}.Scale(vx)).Sub(geom.Vec3{X: c.X, Y: c.Y + d, Z: 20}.Scale(vz)).Add(geom.Vec3{X: c.X * iv, Y: c.Y*iv + sign*iy, Z: c.Z * iv})
	center := f.ToWorld(moment.Scale(1 / volume))
	if math.Abs(m.Volume-volume) > 2e-6 || math.Abs(m.Area-area) > 2e-6 || m.Centroid.Distance(center) > 1e-7 || math.Abs(r.RemovedVolume-(stock-volume)) > 2e-6 || math.Abs(r.SurfaceAreaDelta-(area-9600)) > 2e-6 {
		t.Fatal(m, volume, area, center)
	}
	if display {
		mesh, e := r.Solid.TessellatePartialSkewBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if math.Abs(mesh.SignedVolume()-volume) > .002*volume {
			t.Fatal("mesh volume", mesh.SignedVolume(), volume)
		}
		t.Log(a, b, d, len(mesh.Triangles))
	}
}
func TestBoxPartialSkewBores(t *testing.T) {
	for i, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, d := range []float64{5, -5} {
			t.Run(fmt.Sprintf("%d/%g", i, d), func(t *testing.T) { checkPartialSkew(t, n, 5, 4, d, i == 0 || i == 2 && d == 5) })
		}
	}
	for _, c := range [][3]float64{{5, 5, 5}, {4, 5, 5}, {5, 4, 4.7}} {
		t.Run(fmt.Sprint(c), func(t *testing.T) { checkPartialSkew(t, geom.Vec3{Z: 1}, c[0], c[1], c[2], true) })
	}
}
func TestBoxPartialSkewBoresEmbedding(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{X: -1})
	r, e := BoxPartialSkewBores(context.Background(), f, geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 16, Z: 20}, 5, 4, 4.7, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.TessellatePartialSkewBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	for _, binary := range []bool{false, true} {
		q := m
		q.Vertices = append([]geom.Vec3(nil), m.Vertices...)
		if binary {
			for i, p := range q.Vertices {
				q.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
			}
		}
		if e = q.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		report, e := collision.SelfIntersectionsBVHContext(context.Background(), q, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) > 0 {
			t.Fatal(binary, report, e)
		}
	}
}
func TestBoxPartialSkewBoresRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	dims, c := geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 16, Z: 20}
	for _, d := range []float64{0, 1, -1, math.Nextafter(1, 2), 9, -9, math.Nextafter(9, 8), 10, math.NaN(), math.Inf(1)} {
		if _, e := BoxPartialSkewBores(context.Background(), f, dims, c, 5, 4, d, 1e-9); e == nil {
			t.Fatal(d)
		}
	}
	if _, e := BoxPartialSkewBores(context.Background(), f, dims, geom.Vec3{X: 4, Y: 16, Z: 20}, 5, 4, 5, 1e-9); e == nil {
		t.Fatal("stock boundary")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxPartialSkewBores(ctx, f, dims, c, 5, 4, 5, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
	r, e := BoxPartialSkewBores(context.Background(), f, dims, c, 5, 4, 5, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Solid.TessellatePartialSkewBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxTriangles: 2}); e == nil {
		t.Fatal("mesh budget")
	}
}

func TestPartialSkewTrimPairedSupportAndDerivative(t *testing.T) {
	xf, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	zf, _ := geom.NewFrame(geom.Vec3{Y: 5}, geom.Vec3{Y: -1}, geom.Vec3{Z: 1})
	xs, _ := geom.NewCylinderSurface(xf, 5)
	zs, _ := geom.NewCylinderSurface(zf, 4)
	for _, sx := range []float64{-1, 1} {
		for _, sz := range []float64{-1, 1} {
			for _, reverse := range []bool{false, true} {
				c := partialSkewTrim{a: 5, b: 4, d: 5, sx: sx, sz: sz, from: 0, to: math.Pi / 2}
				if reverse {
					c.from, c.to = c.to, c.from
				}
				for i := 0; i <= 32; i++ {
					t0 := float64(i) / 32
					p, v, e := c.point(t0)
					if e != nil {
						t.Fatal(e)
					}
					for _, xChart := range []bool{false, true} {
						c.xChart = xChart
						uv, e := c.raw(t0)
						if e != nil {
							t.Fatal(e)
						}
						surface := zs
						if xChart {
							surface = xs
						}
						q, e := surface.Eval(uv.X, uv.Y)
						if e != nil || p.Distance(q) > 1e-12 {
							t.Fatal("support", p, q, e)
						}
						du, dv, e := surface.Partials(uv.X, uv.Y)
						if e != nil {
							t.Fatal(e)
						}
						tangent, e := c.DerivativeUV(t0)
						if e != nil || du.Scale(tangent.X).Add(dv.Scale(tangent.Y)).Distance(v) > 1e-11 {
							t.Fatal("tangent", v, e)
						}
						bounds, e := c.BoundsUV()
						if e != nil || uv.X < bounds.Min.X || uv.X > bounds.Max.X || uv.Y < bounds.Min.Y || uv.Y > bounds.Max.Y {
							t.Fatal("bounds", uv, bounds, e)
						}
					}
				}
			}
		}
	}
}
