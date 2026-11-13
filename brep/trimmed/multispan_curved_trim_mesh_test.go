package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedMultiSpanCurvedAnnulus(t *testing.T) {
	circle := func(radius float64, reverse bool) []SplineUV {
		x, y := .5, .5
		r := radius
		p := [][3]geom.Vec3{{{X: x + r, Y: y}, {X: x + r, Y: y + r}, {X: x, Y: y + r}}, {{X: x, Y: y + r}, {X: x - r, Y: y + r}, {X: x - r, Y: y}}, {{X: x - r, Y: y}, {X: x - r, Y: y - r}, {X: x, Y: y - r}}, {{X: x, Y: y - r}, {X: x + r, Y: y - r}, {X: x + r, Y: y}}}
		var loop []SplineUV
		for _, poles := range p {
			c, e := geom.NewBSpline(2, poles[:], []float64{0, 0, 0, 1, 1, 1}, []float64{.5, .5, 1})
			if e != nil {
				t.Fatal(e)
			}
			loop = append(loop, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
		}
		if reverse {
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
	for _, rational := range []bool{false, true} {
		t.Run(map[bool]string{false: "polynomial", true: "rational"}[rational], func(t *testing.T) {
			const knot = .4375
			xs, zs := []float64{0, knot / 2, knot, (1 + knot) / 2, 1}, []float64{0, 0, knot * knot, knot * knot, knot*knot + (1-knot)*(1-knot)}
			var poles []geom.Vec3
			var weights []float64
			for i, u := range xs {
				for j, v := range []float64{0, .5, 1} {
					z := zs[i]
					if j == 2 {
						z += .25
					}
					poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
					w := 1.
					if rational && (i+j)%3 == 0 {
						w = .9375
					}
					weights = append(weights, w)
				}
			}
			surface, e := geom.NewBSplineSurface(2, 2, 5, 3, poles, []float64{0, 0, 0, knot, knot, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, weights)
			if e != nil {
				t.Fatal(e)
			}
			patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			options := MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .05, MaxSubdivisions: 16}}
			options.Mesh.Boundary.Polygon.MaxEdges = 1024
			options.Mesh.Boundary.Polygon.MaxWork = 1000000
			probe := multiSpanMeshWork{options: MultiSpanCurvedTrimMeshOptions{MaxSpanCells: 100000}}
			l2, l1, err := probe.lipschitz(context.Background(), patch)
			if err != nil {
				t.Fatal(err)
			}
			f2, _ := l2.Float64()
			f1, _ := l1.Float64()
			t.Logf("Lipschitz squared %g sum %g", f2, f1)
			if !rational && (l2.Cmp(big.NewRat(225, 64)) != 0 || l1.Cmp(big.NewRat(29, 8)) != 0) {
				t.Fatal("independent polynomial Jacobian maximum", l2, l1)
			}
			for i := 0; i <= 8; i++ {
				for j := 0; j <= 8; j++ {
					jet, err := patch.EvaluateRational(context.Background(), big.NewRat(int64(i), 8), big.NewRat(int64(j), 8), geom.SurfacePatchEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					norm := new(big.Rat)
					for k := 0; k < 3; k++ {
						norm.Add(norm, new(big.Rat).Mul(jet.Du[k], jet.Du[k]))
						norm.Add(norm, new(big.Rat).Mul(jet.Dv[k], jet.Dv[k]))
					}
					if norm.Cmp(l2) > 0 {
						t.Fatal("exact Jacobian sample exceeds global bound")
					}
				}
			}

			result, e := TessellateMultiSpanCurvedTrimRegionCertified(context.Background(), patch, outer, holes, options)
			if e != nil {
				t.Fatal(e)
			}
			bound, _ := result.FillSquaredDistance.Float64()
			if bound > .05*.05 || result.SpanCellChecks <= result.TriangleChecks || result.SpanPieceChecks <= result.TriangleChecks {
				t.Fatal("missing cumulative multi-span certificate", bound, result.SpanCellChecks, result.SpanPieceChecks, result.TriangleChecks)
			}
			independent := func(u, v float64) geom.Vec3 {
				first := 0
				s := u / knot
				if u > knot {
					first = 2
					s = (u - knot) / (1 - knot)
				}
				bu := []float64{(1 - s) * (1 - s), 2 * s * (1 - s), s * s}
				bv := []float64{(1 - v) * (1 - v), 2 * v * (1 - v), v * v}
				p := geom.Vec3{}
				w := 0.
				for i, a := range bu {
					for j, b := range bv {
						index := (first+i)*3 + j
						q := a * b * weights[index]
						p = p.Add(poles[index].Scale(q))
						w += q
					}
				}
				return p.Scale(1 / w)
			}
			crossing := 0
			edges := map[[2]int]int{}
			for i, triangle := range result.Mesh.Triangles {
				u, v := 0., 0.
				minimum, maximum := 1., 0.
				model := geom.Vec3{}
				for j, a := range triangle {
					x, _ := result.ExactUV[a][0].Float64()
					y, _ := result.ExactUV[a][1].Float64()
					u += x / 3
					v += y / 3
					if x < minimum {
						minimum = x
					}
					if x > maximum {
						maximum = x
					}
					model = model.Add(result.Mesh.Vertices[a].Scale(1.0 / 3))
					b := triangle[(j+1)%3]
					if a > b {
						a, b = b, a
					}
					edges[[2]int{a, b}]++
				}
				if minimum < knot && maximum > knot {
					crossing++
				}
				delta := independent(u, v).Sub(model)
				limit, _ := result.SquaredErrors[i].Float64()
				if delta.Dot(delta) > limit+1e-13 {
					t.Fatal("independent C0 rational residual")
				}
			}
			if crossing == 0 || len(result.BoundaryLoops) != 2 || len(result.Mesh.Vertices)-len(edges)+len(result.Mesh.Triangles) != 0 {
				t.Fatal("missing cross-knot annulus topology", crossing)
			}
			for _, n := range edges {
				if n != 1 && n != 2 {
					t.Fatal("nonconforming refinement")
				}
			}
			if !rational {
				for _, limit := range []MultiSpanCurvedTrimMeshOptions{{Mesh: options.Mesh, MaxSpanCells: result.SpanCellChecks - 1}, {Mesh: options.Mesh, MaxSpanPieces: result.SpanPieceChecks - 1}, {Mesh: options.Mesh, Span: MultiSpanTriangleBoundOptions{MaxCells: 1}}} {
					failed, e := TessellateMultiSpanCurvedTrimRegionCertified(context.Background(), patch, outer, holes, limit)
					if e == nil || len(failed.Mesh.Vertices) != 0 {
						t.Fatal("partial result after cumulative span limit")
					}
				}
			}
			t.Logf("%d triangles/%d crossing knots; %d cells and %d pieces; squared bound %g", len(result.Mesh.Triangles), crossing, result.SpanCellChecks, result.SpanPieceChecks, bound)
		})
	}
}
