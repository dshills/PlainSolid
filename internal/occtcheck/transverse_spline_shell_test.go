package occtcheck

import (
	"context"
	"math"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
	"time"
)

func oracleTransverseCaps(t *testing.T) (trimmed.Solid, trimmed.Solid, intersect.SplineSeedReport) {
	t.Helper()
	ctx := context.Background()
	const radius = 5.0 / 16
	build := func(x0, x1, floor float64, dome bool) (trimmed.Solid, *geom.BSplineSurface) {
		t.Helper()
		var surfaces []*geom.BSplineSurface
		var domains []geom.ParameterRectangle
		var reversed []bool
		mk := func(nu, nv int, f func(i, j int) geom.Vec3, rev bool) {
			t.Helper()
			var poles []geom.Vec3
			for i := range nu {
				for j := range nv {
					poles = append(poles, f(i, j))
				}
			}
			knots := func(n int) []float64 {
				k := make([]float64, 2*n)
				for i := n; i < len(k); i++ {
					k[i] = x1
				}
				for i := 0; i < n; i++ {
					k[i] = x0
				}
				return k
			}
			s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, poles, knots(nu), knots(nv), nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, reversed = append(surfaces, s), append(reversed, rev)
			domains = append(domains, geom.ParameterRectangle{})
		}
		z := func(i, j int) float64 {
			if !dome {
				return 1
			}
			q := (x0 - .5) * (x0 - .5)
			v := [3]float64{q, -q, q}
			return radius*radius - v[i] - v[j]
		}
		n := 2
		if dome {
			n = 3
		}
		at := func(i, n int) float64 { return x0 + (x1-x0)*float64(i)/float64(n-1) }
		mk(2, 2, func(i, j int) geom.Vec3 { return geom.Vec3{X: at(i, 2), Y: at(j, 2), Z: floor} }, true)
		mk(n, n, func(i, j int) geom.Vec3 { return geom.Vec3{X: at(i, n), Y: at(j, n), Z: z(i, j)} }, false)
		for _, edge := range []int{0, n - 1} {
			mk(n, 2, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: at(edge, n), Y: at(i, n), Z: []float64{floor, z(edge, i)}[j]}
			}, edge == 0)
		}
		for _, edge := range []int{0, n - 1} {
			mk(n, 2, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: at(i, n), Y: at(edge, n), Z: []float64{floor, z(i, edge)}[j]}
			}, edge != 0)
		}
		s, _, e := trimmed.SewSplineRectangles(ctx, surfaces, domains, reversed, trimmed.SewOptions{Tolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if dome {
			return s, surfaces[1]
		}
		return s, surfaces[0]
	}
	a, aFace := build(0, 1, 0, false)
	b, bFace := build(.125, .875, -.5, true)
	pa, _ := geom.NewSurfacePatch(aFace, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(bFace, geom.SurfacePatchOptions{})
	var hints intersect.SplineSeedReport
	var add func(int, float64, float64, float64, int)
	add = func(axis int, lo, hi, sign float64, depth int) {
		coord := func(x float64) float64 { return .5 + sign*math.Sqrt(radius*radius-(x-.5)*(x-.5)) }
		y0, y1 := min(coord(lo), coord(hi)), max(coord(lo), coord(hi))
		if lo <= .5 && hi >= .5 {
			y0, y1 = min(y0, .5+sign*radius), max(y1, .5+sign*radius)
		}
		pad := min((hi-lo)/2, .03125)
		d := geom.ParameterRectangle{UMin: lo, UMax: hi, VMin: y0 - pad, VMax: y1 + pad}
		g := d
		g.UMin, g.UMax = lo-pad, hi+pad
		if axis == 1 {
			d.UMin, d.UMax, d.VMin, d.VMax = d.VMin, d.VMax, d.UMin, d.UMax
			g.UMin, g.UMax, g.VMin, g.VMax = g.VMin, g.VMax, g.UMin, g.UMax
		}
		x, e := pa.Trim(d)
		if e != nil {
			t.Fatal(e)
		}
		y, e := pb.Trim(g)
		if e != nil {
			t.Fatal(e)
		}
		c, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal(e)
		}
		if !c.ProvenBranch {
			if depth == 8 {
				t.Fatal("no tube", d, g)
			}
			m := (lo + hi) / 2
			add(axis, lo, m, sign, depth+1)
			add(axis, m, hi, sign, depth+1)
			return
		}
		hints.Attempts = append(hints.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: g}, Tube: &c})
	}
	for _, sign := range []float64{-1, 1} {
		add(0, .5-3.0/16, .5+3.0/16, sign, 0)
		add(1, .5-4.0/16, .5+4.0/16, sign, 0)
	}
	return a, b, hints
}

// This expensive opt-in qualification uses the public complete coverage proof.
// It never bypasses that acceptance gate.
func TestOCCTTransverseSplineShells(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_TRANSVERSE") != "1" {
		t.Skip("expensive transverse qualification; set PLAINSOLID_OCCT_TRANSVERSE=1")
	}
	refs := runOracle(t, "transverse-spline-shell")
	want := math.Pi * math.Pow(5./16, 4) / 2
	if math.Abs(refs["transverse-common"].Volume-want) > 1e-8 {
		t.Fatal("OCCT analytic overlap mismatch")
	}
	a, b, hints := oracleTransverseCaps(t)

	o := trimmed.SplineShellTransverseOptions{TubeCells: hints}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Shells.MaxRays = 12
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		name := []string{"transverse-union", "transverse-common", "transverse-cut"}[i]
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			s, _, e := trimmed.CombineTransverseSplineShells(ctx, a, b, 0, 1, op, o)
			if e != nil {
				t.Fatal("public coverage/assembly", e)
			}
			if len(s.Topology().Shells) != 1 {
				t.Fatal("shell count")
			}
			m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .002, MaxSubdivisions: 32, MaxTriangles: 200000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			properties, e := m.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			ref := refs[name]
			if math.Abs(properties.Volume-ref.Volume) > .004 || math.Abs(properties.SurfaceArea-ref.Area) > .03 || properties.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > .01 {
				t.Fatalf("mesh %+v reference %+v", properties, ref)
			}
		})
	}
}
