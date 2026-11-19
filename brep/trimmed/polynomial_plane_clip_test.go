package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func polynomialClipStock(t *testing.T) (Solid, []*geom.BSplineSurface) {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		if axis == 1 {
			u, v = 0, 2
		}
		for side := 0; side < 2; side++ {
			var poles []geom.Vec3
			for i := 0; i < 5; i++ {
				for j := 0; j < 5; j++ {
					var p [3]float64
					p[axis] = float64(side)
					p[u] = float64(i) / 4
					p[v] = float64(j) / 4
					x2 := p[0] * p[0] * p[0]
					if u == 0 {
						x2 = 0
						if i == 3 {
							x2 = .25
						}
						if i == 4 {
							x2 = 1
						}
					}
					if v == 0 {
						x2 = 0
						if j == 3 {
							x2 = .25
						}
						if j == 4 {
							x2 = 1
						}
					}
					p[2] += x2/8 + p[1]/4
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
				}
			}
			s, e := geom.NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces = append(surfaces, s)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			rev := side == 0
			if axis == 1 {
				rev = !rev
			}
			reversed = append(reversed, rev)
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, surfaces
}

func TestPolynomialPlaneClipRejects(t *testing.T) {
	ctx := context.Background()
	stock, surfaces := polynomialClipStock(t)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyPolynomialPlaneFaceSection(canceled, stock, 4, 3./16, SplineRectangleEmbeddingOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal("cancel", e)
	}
	proof, e := CertifyPolynomialPlaneFaceSection(ctx, stock, 4, 3./16, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := CertifyPolynomialPlaneFaceSection(ctx, stock, 4, 3./16, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil {
		t.Fatal("work budget")
	}
	ep, _, e := polynomialPlaneEndpoints(proof.Trim())
	if e != nil {
		t.Fatal(e)
	}
	params := proof.Parameters()
	for i, id := range proof.Edges() {
		jet, e := proof.patch.EvaluateRational(ctx, ep[i][0], ep[i][1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		parameter, exact := params[i].Float64()
		if !exact {
			t.Fatal("fixture parameter")
		}
		point, e := stock.edges[id].Curve.Eval(parameter)
		if e != nil {
			t.Fatal(e)
		}
		for a, v := range []float64{point.X, point.Y, point.Z} {
			if ratInset(v).Cmp(jet.Point[a]) != 0 {
				t.Fatal("original endpoint ownership")
			}
		}
	}
	q := proof.Trim()
	poles := q.Curve.Poles()
	poles[2].Y += 1. / 64
	other, e := geom.NewBSpline(q.Curve.Degree(), poles, q.Curve.Knots(), q.Curve.Weights())
	if e != nil {
		t.Fatal(e)
	}
	_, ends, e := polynomialPlaneEndpoints(q)
	if e != nil {
		t.Fatal(e)
	}
	edge := parametric.Edge{Curve: surfaceTrimCurve{surfaces[4], ends[0], ends[1], q}, Range: parametric.Interval{Max: 1}}
	same, e := comparePolynomialTrimUse(ctx, edge, surfaces[4], Use{Curve: SplineUV{Curve: other, Range: UVInterval{Max: 1}}}, edgecheck.Options{})
	if e != nil || same {
		t.Fatal("same endpoints unequal interior", same, e)
	}
	// Mixed U*V residual is outside the proved separable family.
	poles = surfaces[4].Data().Poles
	poles[6].Z += 1. / 128
	bad, e := geom.NewBSplineSurface(4, 4, 5, 5, poles, []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}, []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(bad, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = polynomialPlaneTrimAxis(patch, big.NewRat(3, 16), 0); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("mixed", e)
	}
}
func TestPolynomialPlaneDegreeAndExportLimits(t *testing.T) {
	var poles []geom.Vec3
	for i := 0; i < 6; i++ {
		for j := 0; j < 2; j++ {
			poles = append(poles, geom.Vec3{X: float64(i) / 5, Y: float64(j), Z: float64(j) / 4})
		}
	}
	s, e := geom.NewBSplineSurface(5, 1, 6, 2, poles, []float64{0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = polynomialPlaneTrimAxis(p, big.NewRat(1, 8), 0); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("higher degree", e)
	}
	h := [][3]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(1, 1)}}
	if _, e = polynomialNativeTrim(h); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("nonbinary control", e)
	}
}
func TestPolynomialPlaneGenuineQuartic(t *testing.T) {
	_, surfaces := polynomialClipStock(t)
	d := surfaces[4].Data()
	for j := 0; j < 5; j++ {
		d.Poles[20+j].Z += 1. / 32
	}
	s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, sign, e := polynomialPlaneTrimAxis(p, big.NewRat(3, 16), 0)
	if e != nil || sign != 1 {
		t.Fatal(e, sign)
	}
	// z=x³/8+x⁴/32+y/4, so y=3/4−x³/2−x⁴/8.
	want := []float64{.75, .75, .75, .625, .125}
	for i, v := range q.Curve.Poles() {
		if v.X != float64(i)/4 || v.Y != want[i] {
			t.Fatal("exact quartic control", i, v)
		}
	}
	ctx := context.Background()
	_, ends, e := polynomialPlaneEndpoints(q)
	if e != nil {
		t.Fatal(e)
	}
	edge := parametric.Edge{Curve: surfaceTrimCurve{s, ends[0], ends[1], q}, Range: parametric.Interval{Max: 1}}
	ok, e := comparePolynomialTrimUse(ctx, edge, s, Use{Curve: q, Reversed: true}, edgecheck.Options{})
	if e != nil || ok {
		t.Fatal("wrong direction", ok, e)
	}
	qr := q
	qr.Reversed = true
	ok, e = comparePolynomialTrimUse(ctx, edge, s, Use{Curve: qr, Reversed: true}, edgecheck.Options{})
	if e != nil || !ok {
		t.Fatal("paired reverse", ok, e)
	}
}
