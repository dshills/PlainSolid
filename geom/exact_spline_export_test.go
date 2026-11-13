package geom

import (
	"errors"
	"testing"
)

func TestExactSplineLosslessExportAndPoints(t *testing.T) {
	source, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	c, _ := NewExactSplineCurve(source, SurfacePatchOptions{})
	c, _ = c.Restrict(.25, .75, SurfacePatchOptions{})
	exported, e := c.ToBSpline()
	if e != nil {
		t.Fatal(e)
	}
	restored, _ := NewExactSplineCurve(exported, SurfacePatchOptions{})
	r, e := CompareExactCurves(c, restored, ExactCurveComparisonOptions{})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	point, e := c.ExactPoint(.5)
	if e != nil || point != (Vec3{1, 1, 0}) {
		t.Fatal(point, e)
	}
	rational := &BSpline{degree: 1, poles: []Vec3{{}, {1, 0, 0}}, weights: []float64{1, 2}, knots: []float64{0, 0, 1, 1}}
	c, _ = NewExactSplineCurve(rational, SurfacePatchOptions{})
	if _, e := c.ExactPoint(.5); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
	c, _ = c.Restrict(.5, 1, SurfacePatchOptions{})
	if _, e := c.ToBSpline(); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
}
func TestExactSurfaceLosslessExport(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	p, e := p.Trim(ParameterRectangle{UMin: .25, UMax: .75, VMin: .125, VMax: .5})
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	q, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, uv := range []Vec2{{.25, .125}, {.75, .5}, {.5, .25}} {
		a, _ := p.Eval(uv.X, uv.Y)
		b, _ := q.Eval(uv.X, uv.Y)
		if a != b {
			t.Fatal(a, b)
		}
	}
}
func TestExactReparameterization(t *testing.T) {
	source, _ := NewBSpline(1, []Vec3{{}, {1, 0, 0}, {3, 0, 0}}, []float64{2, 2, 3, 5, 5}, nil)
	c, _ := NewExactSplineCurve(source, SurfacePatchOptions{})
	if _, e := c.Reparameterize(0, 1, false, SurfacePatchOptions{}); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
	q, e := c.Reparameterize(-4, 2, true, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareExactCurves(c, q, ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: true})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	mapped, e := p.Reparameterize(ParameterRectangle{UMin: 2, UMax: 6, VMin: -3, VMax: 5}, true, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point, e := mapped.Eval(3, 1)
	if e != nil {
		t.Fatal(e)
	}
	expected, _ := p.Eval(.75, .5)
	if point != expected {
		t.Fatal(point, expected)
	}
	du, dv, e := mapped.Partials(3, 1)
	if e != nil {
		t.Fatal(e)
	}
	a, b, _ := p.Partials(.75, .5)
	near(t, du, a.Scale(-.25), 0)
	near(t, dv, b.Scale(.125), 0)
}
