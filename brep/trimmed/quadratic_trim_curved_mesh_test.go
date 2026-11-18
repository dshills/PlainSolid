package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedCurvedRationalAnnulusMesh(t *testing.T) {
	slowtest.Skip(t)
	circle := func(r float64, clockwise bool) []SplineUV {
		x, y := .5, .5
		controls := [][3]geom.Vec3{{{X: x + r, Y: y}, {X: x + r, Y: y + r}, {X: x, Y: y + r}}, {{X: x, Y: y + r}, {X: x - r, Y: y + r}, {X: x - r, Y: y}}, {{X: x - r, Y: y}, {X: x - r, Y: y - r}, {X: x, Y: y - r}}, {{X: x, Y: y - r}, {X: x + r, Y: y - r}, {X: x + r, Y: y}}}
		var loop []SplineUV
		for _, p := range controls {
			c, e := geom.NewBSpline(2, p[:], []float64{0, 0, 0, 1, 1, 1}, []float64{.5, .5, 1})
			if e != nil {
				t.Fatal(e)
			}
			loop = append(loop, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
		}
		if clockwise {
			for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
				loop[i], loop[j] = loop[j], loop[i]
			}
			for i := range loop {
				loop[i].Reversed = true
			}
		}
		return loop
	}
	outer, holes := circle(.375, false), [][]SplineUV{circle(.125, true)}
	for _, fixture := range []struct {
		name     string
		rational bool
		height   float64
	}{{"polynomial", false, .125}, {"rational", true, .125}, {"refined", false, 2}} {
		rational := fixture.rational
		t.Run(fixture.name, func(t *testing.T) {
			k := []float64{0, 0, 1, 1}
			poles := []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: fixture.height}}
			weights := []float64{1, 1, 1, 1}
			if rational {
				weights[3] = .875
			}
			surface, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, k, k, weights)
			if e != nil {
				t.Fatal(e)
			}
			if fixture.height > 1 {
				var net []geom.Vec3
				for i := 0; i < 3; i++ {
					for j := 0; j < 3; j++ {
						z := 0.
						if i == 2 {
							z += fixture.height
						}
						if j == 2 {
							z += fixture.height
						}
						net = append(net, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
					}
				}
				knots := []float64{0, 0, 0, 1, 1, 1}
				surface, e = geom.NewBSplineSurface(2, 2, 3, 3, net, knots, knots, nil)
				if e != nil {
					t.Fatal(e)
				}
			}
			patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			options := CurvedQuadraticTrimMeshOptions{Deflection: .05, MaxSubdivisions: 16}
			result, e := TessellateCurvedQuadraticTrimRegionCertified(context.Background(), patch, outer, holes, options)
			if e != nil {
				t.Fatal(e)
			}
			if fixture.height > 1 && result.Subdivisions == 1 {
				t.Fatal("fixture did not exercise rational midpoint refinement")
			}
			total, _ := result.FillSquaredDistance.Float64()
			if total > .05*.05 {
				t.Fatal("requested total not met", total)
			}
			evaluate := func(u, v float64) geom.Vec3 {
				if fixture.height > 1 {
					return geom.Vec3{X: u, Y: v, Z: fixture.height * (u*u + v*v)}
				}
				b := []float64{(1 - u) * (1 - v), (1 - u) * v, u * (1 - v), u * v}
				point := geom.Vec3{}
				weight := 0.
				for i, q := range b {
					point = point.Add(poles[i].Scale(q * weights[i]))
					weight += q * weights[i]
				}
				return point.Scale(1 / weight)
			}
			for i, triangle := range result.Mesh.Triangles {
				u, v := 0., 0.
				model := geom.Vec3{}
				for _, id := range triangle {
					x, _ := result.ExactUV[id][0].Float64()
					y, _ := result.ExactUV[id][1].Float64()
					u += x / 3
					v += y / 3
					model = model.Add(result.Mesh.Vertices[id].Scale(1.0 / 3))
				}
				delta := evaluate(u, v).Sub(model)
				bound, _ := result.SquaredErrors[i].Float64()
				if delta.Dot(delta) > bound+1e-14 {
					t.Fatal("independent rational barycentric residual")
				}
			}
			// Independent true circular source boundaries on the nonplanar chart.
			for li, radius := range []float64{.375, .125} {
				loop := result.BoundaryLoops[li]
				for i := 0; i < 32; i++ {
					angle := 2 * math.Pi * (float64(i) + .37) / 32
					source := evaluate(.5+radius*math.Cos(angle), .5+radius*math.Sin(angle))
					nearest := math.Inf(1)
					for j, a := range loop {
						p, _, e := geom.ClosestOnSegment(source, result.Mesh.Vertices[a], result.Mesh.Vertices[loop[(j+1)%len(loop)]])
						if e != nil {
							t.Fatal(e)
						}
						delta := source.Sub(p)
						nearest = math.Min(nearest, delta.Dot(delta))
					}
					if nearest > total+1e-12 {
						t.Fatal("curved source boundary exceeds composed fill bound", nearest, total)
					}
				}
			}
			edges := map[[2]int]int{}
			for _, triangle := range result.Mesh.Triangles {
				for j, a := range triangle {
					b := triangle[(j+1)%3]
					if a > b {
						a, b = b, a
					}
					edges[[2]int{a, b}]++
				}
			}
			if len(result.Mesh.Vertices)-len(edges)+len(result.Mesh.Triangles) != 0 {
				t.Fatal("annulus topology lost")
			}
			for _, n := range edges {
				if n != 1 && n != 2 {
					t.Fatal("nonconforming edge")
				}
			}
			t.Logf("%d triangles, level %d, total squared bound %g", len(result.Mesh.Triangles), result.Subdivisions, total)
			options.MaxTriangleChecks = 1
			failed, e := TessellateCurvedQuadraticTrimRegionCertified(context.Background(), patch, outer, holes, options)
			if e == nil || len(failed.Mesh.Vertices) != 0 {
				t.Fatal("partial result on budget failure")
			}
		})
	}
}
