package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func transposedQuadraticClipStock(t *testing.T, reverseDependent bool) Solid {
	t.Helper()
	original, surfaces := quadraticClipStock(t)
	domain := geom.ParameterRectangle{UMin: -2, UMax: 2, VMin: 3, VMax: 7}
	var target []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for id, s := range surfaces {
		d := s.Data()
		poles := make([]geom.Vec3, len(d.Poles))
		weights := make([]float64, len(d.Weights))
		for i := 0; i < d.CountU; i++ {
			for j := 0; j < d.CountV; j++ {
				poles[j*d.CountU+i] = d.Poles[i*d.CountV+j]
				if len(weights) > 0 {
					weights[j*d.CountU+i] = d.Weights[i*d.CountV+j]
				}
			}
		}
		transposed, e := geom.NewBSplineSurface(d.DegreeV, d.DegreeU, d.CountV, d.CountU, poles, d.KnotsV, d.KnotsU, weights)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := geom.NewSurfacePatch(transposed, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		patch, e = patch.Reparameterize(domain, reverseDependent, false, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		transposed, e = patch.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
		target = append(target, transposed)
		domains = append(domains, domain)
		rev := !original.faces[id].Reversed
		if reverseDependent {
			rev = !rev
		}
		reversed = append(reversed, rev)
	}
	s, _, e := SewSplineRectangles(context.Background(), target, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	for i, old := range edges {
		exact, reverse, e := exactEdgeSpline(old)
		if e != nil || exact == nil {
			t.Fatal(e)
		}
		exact, e = exact.Reparameterize(2, 6, !reverse, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		curve, e := exact.ToBSpline()
		if e != nil {
			t.Fatal(e)
		}
		edges[i] = parametric.Edge{Start: old.End, End: old.Start, Curve: curve, Range: parametric.Interval{Min: 2, Max: 6}}
	}
	for i := range faces {
		for j := range faces[i].Outer {
			faces[i].Outer[j].Reversed = !faces[i].Outer[j].Reversed
		}
	}
	s, e = NewSolidContext(context.Background(), vertices, edges, faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestQuadraticPlaneFreeVOriginalChart(t *testing.T) {
	t.Run("increasing-dependent", func(t *testing.T) { checkQuadraticPlaneFreeVOriginalChart(t, false) })
	t.Run("reversed-dependent", func(t *testing.T) { checkQuadraticPlaneFreeVOriginalChart(t, true) })
}
func checkQuadraticPlaneFreeVOriginalChart(t *testing.T, reverseDependent bool) {
	ctx := context.Background()
	stock := transposedQuadraticClipStock(t, reverseDependent)
	for _, face := range []int{2, 4} {
		if _, e := CertifyQuadraticPlaneFaceSection(ctx, stock, face, 3./16, SplineRectangleEmbeddingOptions{}); e == nil {
			t.Fatal("legacy wrapper changed")
		}
		p, e := CertifyQuadraticPlaneFaceSectionAxis(ctx, stock, face, 3./16, 1, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if p.FreeAxis() != 1 || p.DependentAxis() != 0 || p.AboveSign() != func() int {
			if reverseDependent {
				return -1
			}
			return 1
		}() {
			t.Fatal("axis/sign")
		}
		exact, display := p.Trim().Endpoints()
		if display[0].Y != 3 || display[1].Y != 7 {
			t.Fatal("original V edges", display)
		}
		parameters := p.Parameters()
		for i, id := range p.Edges() {
			if parameters[i].Cmp(ratInset(2)) <= 0 || parameters[i].Cmp(ratInset(6)) >= 0 {
				t.Fatal("edge interior")
			}
			parameter, e := geom.RepresentedExactScalar(parameters[i])
			if e != nil {
				t.Fatal(e)
			}
			point, e := stock.edges[id].Curve.Eval(parameter)
			if e != nil {
				t.Fatal(e)
			}
			j, e := p.patch.EvaluateRational(ctx, exact[i][0], exact[i][1], geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis, x := range []float64{point.X, point.Y, point.Z} {
				if j.Point[axis].Cmp(ratInset(x)) != 0 {
					t.Fatal("reversed nonunit original edge parameter")
				}
			}
		}
	}
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, e := CombineQuadraticSplinePlaneClip(ctx, stock, tool, 5, op, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		volume, center := 29./640, geom.Vec3{X: 65. / 174, Y: 527. / 2436, Z: 325. / 2436}
		if op == parametric.Difference {
			volume = 611. / 640
			center = geom.Vec3{X: 1855. / 3666, Y: 26353. / 51324, Z: 35515. / 51324}
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-volume) > 1e-9 || m.Centroid.Sub(center).Length() > 1e-9 {
			t.Fatal("parameter-independent integral", m, e)
		}
		if r.Solid.Topology().EulerCharacteristic != 2 || len(r.SectionEdges) != 4 {
			t.Fatal("topology")
		}
		for _, face := range []int{2, 4} {
			if len(r.StockFaces[face]) != 1 || r.Solid.faces[r.StockFaces[face][0]].Surface != stock.faces[face].Surface {
				t.Fatal("original support changed")
			}
		}
		owned := map[int]bool{}
		for _, ids := range r.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("source ancestry")
				}
				owned[id] = true
			}
		}
		for _, id := range r.SectionEdges {
			if owned[id] {
				t.Fatal("section ancestry")
			}
			owned[id] = true
		}
		if len(owned) != len(r.Solid.edges) {
			t.Fatal("missing ancestry")
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		lm, e := reload.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(lm.Volume-volume) > 1e-9 {
			t.Fatal("reload", e)
		}
		mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		c, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !c.Complete || len(c.Contacts) != 0 {
			t.Fatal("contacts", e)
		}
	}
	for _, axis := range []int{-1, 2} {
		if _, e := CertifyQuadraticPlaneFaceSectionAxis(ctx, stock, 4, 3./16, axis, SplineRectangleEmbeddingOptions{}); e == nil {
			t.Fatal("invalid axis")
		}
	}
	if _, e := CertifyQuadraticPlaneFaceSectionAxis(ctx, stock, 4, 3./16, 1, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyQuadraticPlaneFaceSectionAxis(canceled, stock, 4, 3./16, 1, SplineRectangleEmbeddingOptions{}); e != context.Canceled {
		t.Fatal("cancel", e)
	}
}

func TestQuadraticPlaneAxisUnsupportedTerms(t *testing.T) {
	_, surfaces := quadraticClipStock(t)
	d := surfaces[4].Data()
	// Both variables are genuinely quadratic, giving neither constant nonzero
	// dependent derivative. It must not be accepted by switching the free axis.
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z += .125
			}
			if j == 2 {
				z += .25
			}
			d.Poles[i*3+j].Z = z
		}
	}
	s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for axis := 0; axis < 2; axis++ {
		if _, _, e = quadraticPlaneTrimAxis(p, ratInset(3./16), axis); e == nil {
			t.Fatal("two quadratic axes", axis)
		}
	}
	source := transposedQuadraticClipStock(t, false).faces[4].Surface.(*geom.BSplineSurface)
	source, e = source.InsertVKnot(5, 2)
	if e != nil {
		t.Fatal(e)
	}
	p, e = geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = quadraticPlaneTrimAxis(p, ratInset(3./16), 1); e == nil {
		t.Fatal("internal free-axis knot accepted without piece ownership")
	}
}
