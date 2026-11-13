package trimmed

import (
	"context"
	"math"
	"math/big"
	"os"
	"plainsolid/intersect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// B is -45/16 <= z <= (x-.5)^2-1/16, -.5 <= x <=1.5,
// -.5 <=y<=1.5. Its roof crosses A's bottom twice, on two open
// arcs belonging to the same ordered face pair. All crossing edge
// parameters are dyadic, independently of numerical root isolation.
func repeatedPairSplineFixture(t *testing.T) (Solid, Solid) {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	add := func(du, dv, nu, nv int, p []geom.Vec3, reverse bool) {
		ku, kv := make([]float64, 2*(du+1)), make([]float64, 2*(dv+1))
		for i := du + 1; i < len(ku); i++ {
			ku[i] = 1
		}
		for i := dv + 1; i < len(kv); i++ {
			kv[i] = 1
		}
		if nu == 5 {
			ku = []float64{0, 0, 0, .5, .5, 1, 1, 1}
		}
		s, e := geom.NewBSplineSurface(du, dv, nu, nv, p, ku, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, s)
		domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
		reversed = append(reversed, reverse)
	}
	var floor, roof []geom.Vec3
	xs := []float64{-.5, .5, 1.5}
	zs := []float64{15. / 16, -17. / 16, 15. / 16}
	for i, x := range xs {
		for _, y := range []float64{-.5, 1.5} {
			floor = append(floor, geom.Vec3{X: x, Y: y, Z: -45. / 16})
			roof = append(roof, geom.Vec3{X: x, Y: y, Z: zs[i]})
		}
	}
	add(2, 1, 3, 2, floor, true)
	add(2, 1, 3, 2, roof, false)
	for i, x := range []float64{-.5, 1.5} {
		var p []geom.Vec3
		for _, y := range []float64{-.5, 1.5} {
			for _, z := range []float64{-45. / 16, 15. / 16} {
				p = append(p, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
		add(1, 1, 2, 2, p, i == 0)
	}
	for side, y := range []float64{-.5, 1.5} {
		var p []geom.Vec3
		for i, x := range xs {
			p = append(p, geom.Vec3{X: x, Y: y, Z: -45. / 16}, geom.Vec3{X: x, Y: y, Z: zs[i]})
		}
		add(2, 1, 3, 2, p, side == 1)
	}
	b, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	if _, e = b.ValidateSplineRectangleEmbedding(context.Background(), SplineRectangleEmbeddingOptions{}); e != nil {
		t.Fatal("quadratic prism embedding", e)
	}
	p, e := b.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-37./3) > 1e-10 {
		t.Fatal("input integral", p, e)
	}
	return a, b
}

func TestCombineRepeatedFacePairOpenSplineArcs(t *testing.T) {
	a, b := repeatedPairSplineFixture(t)
	sections := repeatedPairSplineSections(t, a, b)
	for _, tc := range []struct {
		name          string
		op            parametric.BooleanOp
		volume        float64
		shells, euler int
	}{
		{"union", parametric.Union, 319. / 24, 1, 0},
		{"common", parametric.Intersection, 1. / 24, 2, 4},
		{"cut", parametric.Difference, 23. / 24, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, h, e := CombineOpenSplineShells(context.Background(), a, b, tc.op, sections, SplineShellOpenOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if !testing.Short() {
				m, e := out.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .01, MaxSubdivisions: 64, MaxTriangles: 250000})
				if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-tc.volume) > .005 {
					t.Fatal("bounded sampled volume", m.SignedVolume(), tc.volume, e)
				}
			}

			if os.Getenv("PLAINSOLID_SPLINE_REPEAT_MASS") == "1" {
				p, e := out.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil || math.Abs(p.Volume-tc.volume) > 1e-8 || math.Abs(p.Centroid.X-.5) > 1e-8 || math.Abs(p.Centroid.Y-.5) > 1e-8 {
					t.Fatal(p, e)
				}
			}

			if len(out.Topology().Shells) != tc.shells || out.Topology().EulerCharacteristic != tc.euler || len(h.SectionEdges) == 0 {
				t.Fatalf("shells=%d Euler=%d sections=%d", len(out.Topology().Shells), out.Topology().EulerCharacteristic, len(h.SectionEdges))
			}
		})
	}
}

func repeatedPairSplineSections(t *testing.T, a, b Solid) []SplineShellOpenSection {
	t.Helper()
	ctx := context.Background()
	var result []SplineShellOpenSection
	for _, face := range []int{0, 1, 2, 3, 4} {
		pa, _ := intersectSplineFace(a.faces[face], geom.ParameterRectangle{UMax: 1, VMax: 1})
		pb, _ := intersectSplineFace(b.faces[1], geom.ParameterRectangle{UMax: 1, VMax: 1})
		var seeds intersect.SplineSeedReport
		cell := func(free int, lo, hi, bu0, bu1 float64) {
			da := geom.ParameterRectangle{UMax: 1, VMax: 1}
			db := geom.ParameterRectangle{UMin: bu0, UMax: bu1, VMin: .125, VMax: .875}
			if face < 2 {
				da.VMin, da.VMax = .125, .25
			}
			if face == 2 || face == 3 {
				const pad = 1. / 512
				bound := func(v float64, up bool) float64 {
					if up {
						return math.Ceil(v*1024)/1024 + pad
					}
					return math.Floor(v*1024)/1024 - pad
				}
				var xlo, xhi float64
				if free == 1 {
					xlo, xhi = lo, hi
					z0, z1 := (lo-.5)*(lo-.5)-1./16, (hi-.5)*(hi-.5)-1./16
					da.UMin, da.UMax = bound(min(z0, z1), false), bound(max(z0, z1), true)
				} else {
					if bu0 < .5 {
						xlo, xhi = .5-math.Sqrt(hi+1./16), .5-math.Sqrt(lo+1./16)
					} else {
						xlo, xhi = .5+math.Sqrt(lo+1./16), .5+math.Sqrt(hi+1./16)
					}
					da.VMin, da.VMax = bound(xlo, false), bound(xhi, true)
				}
				db.UMin, db.UMax = bound((xlo+.5)/2, false), bound((xhi+.5)/2, true)
				bv := .25 + float64(face-2)*.5
				db.VMin, db.VMax = bv-pad, bv+pad
			}

			if face == 4 {
				x := .25
				if bu0 > .5 {
					x = .75
				}
				da.UMin, da.UMax = x-1./16, x+1./16
			}
			if free == 0 {
				da.UMin, da.UMax = lo, hi
			} else {
				da.VMin, da.VMax = lo, hi
			}
			x, e := geom.NewSurfacePatch(a.faces[face].Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			y, e := geom.NewSurfacePatch(b.faces[1].Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			x, e = x.Trim(da)
			if e != nil {
				t.Fatal(e)
			}
			y, e = y.Trim(db)
			if e != nil {
				t.Fatal(e)
			}
			tube, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: free})
			if e != nil || !tube.ProvenBranch {
				t.Fatalf("face%d free%d span%v,%v: %v", face, free, lo, hi, e)
			}
			seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &tube})
		}
		add := func(free int, lo, hi, bu0, bu1 float64) {
			n := 1
			if face == 2 || face == 3 {
				n = 4
			}
			for i := 0; i < n; i++ {
				cell(free, lo+(hi-lo)*float64(i)/float64(n), lo+(hi-lo)*float64(i+1)/float64(n), bu0, bu1)
			}
		}

		var endpoints [][4]float64
		switch face {
		case 0, 1:
			bu := .25 + float64(face)*.5
			add(0, 0, 1, bu-1./64, bu+1./64)
			endpoints = [][4]float64{{0, 3. / 16, bu, .25}, {1, 3. / 16, bu, .75}}
		case 2, 3:
			add(1, 0, .125, 15./64, 21./64)
			add(0, 0, 5./64, 19./64, 25./64)
			add(0, 0, 5./64, 39./64, 45./64)
			add(1, .875, 1, 43./64, 49./64)
			bv := .25 + float64(face-2)*.5
			endpoints = [][4]float64{{3. / 16, 0, .25, bv}, {0, .25, .375, bv}, {0, .75, .625, bv}, {3. / 16, 1, .75, bv}}
		case 4:
			add(1, 0, 1, 23./64, 25./64)
			add(1, 0, 1, 39./64, 41./64)
			endpoints = [][4]float64{{.25, 0, .375, .25}, {.25, 1, .375, .75}, {.75, 0, .625, .25}, {.75, 1, .625, .75}}
		}
		graph, e := intersect.JoinSplineTubeBranches(ctx, pa, pb, seeds, intersect.TubeBranchOptions{})
		if e != nil {
			t.Fatal(face, e)
		}
		for branch, g := range graph.Branches {
			curves, e := intersect.SplineBranchCurves(ctx, pa, pb, seeds, g, intersect.TubeCurveOptions{})
			if e != nil {
				t.Fatal(e)
			}
			section := SplineShellOpenSection{FaceA: face, FaceB: 1, Branch: branch}
			if branch == 0 {
				section.TubeCells = seeds
			}
			for end := 0; end < 2; end++ {
				index := 0
				if end == 1 {
					index = len(curves) - 1
				}
				q, e := curves[index].EvaluatePair(float64(end))
				if e != nil {
					t.Fatal(e)
				}
				got := [4]float64{q.A.X, q.A.Y, q.B.X, q.B.Y}
				found := false
				for _, p := range endpoints {
					match := true
					for k := range p {
						if math.Abs(p[k]-got[k]) > 1e-8 {
							match = false
						}
					}
					if !match {
						continue
					}
					found = true
					side := geom.SurfaceEdgeUMin
					if p[0] == 1 {
						side = geom.SurfaceEdgeUMax
					} else if p[1] == 0 {
						side = geom.SurfaceEdgeVMin
					} else if p[1] == 1 {
						side = geom.SurfaceEdgeVMax
					}
					hint := [4]*big.Rat{}
					for k := range p {
						hint[k] = new(big.Rat).SetFloat64(p[k])
					}
					endpoint := SplineShellOpenEndpoint{Operand: 0, Edge: SplineTubeEndpoint{Side: side, Hint: hint}}
					if end == 0 {
						section.Start = endpoint
					} else {
						section.End = endpoint
					}
					break
				}
				if !found {
					t.Fatal("unmatched analytic endpoint", face, got)
				}
			}
			result = append(result, section)
		}
	}
	if len(result) != 8 {
		t.Fatal("expected eight branches", len(result))
	}
	return result
}
