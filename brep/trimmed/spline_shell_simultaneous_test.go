package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// Re-express a fixture's affine placement in its original spline charts.
func splineSimultaneousTransform(t *testing.T, input Solid, m geom.Affine) Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for _, f := range input.faces {
		d, e := faceUVBounds(f)
		if e != nil {
			t.Fatal(e)
		}
		x := f.Surface.(*geom.BSplineSurface).Data()
		for i, p := range x.Poles {
			x.Poles[i] = m.Apply(p)
		}
		s, e := geom.NewBSplineSurface(x.DegreeU, x.DegreeV, x.CountU, x.CountV, x.Poles, x.KnotsU, x.KnotsV, x.Weights)
		if e != nil {
			t.Fatal(e)
		}
		surfaces, domains, reversed = append(surfaces, s), append(domains, d), append(reversed, f.Reversed)
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func splineSimultaneousWarp(t *testing.T, input Solid, h float64) Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for _, f := range input.faces {
		s := f.Surface.(*geom.BSplineSurface)
		x, _ := s.Eval(0, 0)
		u, _ := s.Eval(1, 0)
		v, _ := s.Eval(0, 1)
		u, v = u.Sub(x), v.Sub(x)
		poles := make([]geom.Vec3, 0, 9)
		for i := range 3 {
			for j := range 3 {
				a, b := float64(i)/2, float64(j)/2
				p := x.Add(u.Scale(a)).Add(v.Scale(b))
				// The fixed triangular warp is z <- z+h*(x-y)^2.
				d, e, f := x.X-x.Y, u.X-u.Y, v.X-v.Y
				uv := d*d + 2*d*e*a + 2*d*f*b + 2*e*f*a*b
				if i == 2 {
					uv += e * e
				}
				if j == 2 {
					uv += f * f
				}
				p.Z += h * uv
				poles = append(poles, p)
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		p, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces, domains, reversed = append(surfaces, p), append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1}), append(reversed, f.Reversed)
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestCombineCurvedSplineShellsSimultaneousEdge(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	b := splineSimultaneousTransform(t, a, geom.Affine{{0, .25, .25, .875}, {.25, 0, .25, .875}, {.25, .25, 0, .5}})
	var ap, bp []splineEmbeddingFace
	for _, x := range []struct {
		s Solid
		p *[]splineEmbeddingFace
	}{{a, &ap}, {b, &bp}} {
		if _, e := x.s.validateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}, x.p); e != nil {
			t.Fatal(e)
		}
	}
	pairs := [][2]int{{1, 0}, {1, 4}, {3, 2}, {3, 4}}
	sections, e := discoverAffineSplineOpenSections(ctx, a, b, ap, bp, pairs, SplineShellOpenOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, b = splineSimultaneousWarp(t, a, 1.0/64), splineSimultaneousWarp(t, b, 1.0/64)
	// The same exact chart coordinates describe the warped intersections.
	// Only box/axis hints are carried across; the assembly rechecks each
	// proof against these new nonlinear world supports.
	o := SplineShellOpenOptions{Trace: intersect.SplinePairTraceOptions{Coverage: intersect.SplineTubeCoverageOptions{
		Cell: geom.SurfaceTubeUnionCoverageOptions{MaxDepth: 16, MaxOperations: 12000000},
	}}}
	// The warp has determinant one. The unwarped overlap is 1/768
	// and the second shell volume is 1/32.
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineSectionedSplineShells(ctx, a, b, op, sections, o)
		if e != nil || len(h.SectionEdges) != 8 {
			t.Fatal(h.FacePairs, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [3]float64{1 + 1.0/32 - 1.0/768, 1.0 / 768, 1 - 1.0/768}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-10 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(p, want, e)
		}
	}
	bad := append([]SplineShellSection(nil), sections...)
	for i := range bad {
		if bad[i].Start.OtherSide != nil {
			bad[i].Start.OtherSide = nil
			break
		}
		if bad[i].End.OtherSide != nil {
			bad[i].End.OtherSide = nil
			break
		}
	}
	if s, _, e := CombineSectionedSplineShells(ctx, a, b, parametric.Intersection, bad, o); !errors.Is(e, geom.ErrConvergence) || len(s.faces) != 0 {
		t.Fatal("incomplete simultaneous wire was accepted", e)
	}
	auto := SplineShellCombineOptions{Open: o, DiscoverVerticalQuadraticWarp: true, DisablePlaneDiscovery: true}
	result, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, auto)
	if e != nil || h.Method != "vertical-quadratic-warp-arrangement" || len(h.SectionEdges) != 8 {
		t.Fatal(h.Method, len(h.SectionEdges), e)
	}
	m, e := result.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-1.0/768) > 1e-10 {
		t.Fatal(m, e)
	}
}
