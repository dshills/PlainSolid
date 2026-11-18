package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func wrappedCylinder(t testing.TB) *geom.WrappedSurfacePatch {
	t.Helper()
	circle, e := geom.NewCircle(geom.Vec3{}, geom.Vec3{Z: 1}, 2)
	if e != nil {
		t.Fatal(e)
	}
	c, e := circle.RationalBSpline()
	if e != nil {
		t.Fatal(e)
	}
	poles := []geom.Vec3{}
	weights := []float64{}
	for i, p := range c.Poles() {
		for _, z := range []float64{-2, 3} {
			poles = append(poles, geom.Vec3{X: p.X, Y: p.Y, Z: z})
			weights = append(weights, c.Weights()[i])
		}
	}
	s, e := geom.NewBSplineSurface(c.Degree(), 1, len(c.Poles()), 2, poles, c.Knots(), []float64{-2, -2, 3, 3}, weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), patch, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}
func TestWrappedSurfaceRoundtrip(t *testing.T) {
	t.Parallel()
	w := wrappedCylinder(t)
	m := WrappedSurfaceModel{LengthUnit: .001, Surfaces: []WrappedSplineSurface{{Name: "closed cylinder", Geometry: w, SelfIntersect: "U"}}}
	var b bytes.Buffer
	if e := WriteWrappedSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	round, e := ReadWrappedSurfaces(&b, Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	got := round.Surfaces[0]
	if got.Name != "closed cylinder" || round.LengthUnit != m.LengthUnit || !got.Geometry.SeamCertificate().FirstPartialsMatched[0] {
		t.Fatal(round)
	}
	a, _ := w.ExportRecipe()
	c, _ := got.Geometry.ExportRecipe()
	if !reflect.DeepEqual(a.Surface.Data(), c.Surface.Data()) {
		t.Fatal("fundamental data changed")
	}
	x, e := got.Geometry.EvaluateRational(context.Background(), big.NewRat(1, 4), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	y, e := got.Geometry.EvaluateRational(context.Background(), big.NewRat(369, 4), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if x.Point[i].Cmp(y.Point[i]) != 0 {
			t.Fatal("wrapped phase changed")
		}
	}
	generic, e := ReadSurfaces(bytes.NewReader(raw), Limits{})
	if e != nil || generic.Surfaces[0].ClosedU != "T" || generic.Surfaces[0].ClosedV != "F" {
		t.Fatal(generic, e)
	}
	for _, o := range []WrappedSurfaceOptions{{MaxPoles: 4}, {MaxOperations: 1}, {ContinuityOrder: 2}} {
		if _, e := ReadWrappedSurfaces(bytes.NewReader(raw), Limits{}, o); e == nil {
			t.Fatal(o)
		}
	}
}
func TestWrappedSurfaceSeamReject(t *testing.T) {
	t.Parallel()
	w := wrappedCylinder(t)
	recipe, _ := w.ExportRecipe()
	d := recipe.Surface.Data()
	d.Poles[len(d.Poles)-1].X = math.Nextafter(d.Poles[len(d.Poles)-1].X, math.Inf(1))
	g, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSurfaces(&b, SurfaceModel{LengthUnit: 1, Surfaces: []SplineSurface{{Geometry: g, ClosedU: "T", ClosedV: "F", SelfIntersect: "U"}}}); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	if _, e = ReadSurfaces(bytes.NewReader(raw), Limits{}); e != nil {
		t.Fatal("generic behavior changed", e)
	}
	if _, e = ReadWrappedSurfaces(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("oneULP seam accepted")
	}
	tiny, _, e := w.Transformed(context.Background(), geom.Translation(geom.Vec3{X: math.Ldexp(1, -80)}), geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e = WriteWrappedSurfaces(&b, WrappedSurfaceModel{LengthUnit: 1, Surfaces: []WrappedSplineSurface{{Geometry: tiny}}}); !errors.Is(e, geom.ErrUnrepresentable) || b.Len() != 0 {
		t.Fatal("rounded export", e)
	}
}
func TestWrappedSurfaceC0NotC1(t *testing.T) {
	t.Parallel()
	xy := []geom.Vec2{{X: 1}, {X: 1, Y: 1}, {Y: 1}, {}, {X: 1}}
	p := []geom.Vec3{}
	for _, v := range xy {
		p = append(p, geom.Vec3{X: v.X, Y: v.Y}, geom.Vec3{X: v.X, Y: v.Y, Z: 1})
	}
	s, e := geom.NewBSplineSurface(1, 1, 5, 2, p, []float64{0, 0, 1, 2, 3, 4, 4}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSurfaces(&b, SurfaceModel{LengthUnit: 1, Surfaces: []SplineSurface{{Geometry: s, ClosedU: "T", ClosedV: "F", SelfIntersect: "U"}}}); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	if _, e = ReadWrappedSurfaces(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedSurfaces(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1}); e == nil {
		t.Fatal("closure flag implied C1")
	}
}
func wrappedFromOCCT(t testing.TB, raw []byte) WrappedSplineSurface {
	t.Helper()
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
			w, e := DecodeWrappedSurface(context.Background(), f, id, WrappedSurfaceOptions{ContinuityOrder: 1})
			if e != nil {
				t.Fatal(e)
			}
			return w
		}
	}
	t.Fatal("missing surface")
	return WrappedSplineSurface{}
}
func TestOCCTWrappedSurface(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-wrapped-surface")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	w := wrappedFromOCCT(t, raw)
	var b bytes.Buffer
	if e = WriteWrappedSurfaces(&b, WrappedSurfaceModel{LengthUnit: .001, Surfaces: []WrappedSplineSurface{w}}); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	a, c := stepHoledArea(t, source), stepHoledArea(t, path)
	if math.Abs(a-c) > 1e-8 {
		t.Fatal(a, c)
	}
	for _, p := range []string{source, path} {
		out := filepath.Join(t.TempDir(), "area.json")
		stepOracle(t, out, "step-mass-precise", p)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var record struct{ Area float64 }
		if e = json.Unmarshal(raw, &record); e != nil || math.Abs(record.Area-20*math.Pi) > 1e-6 {
			t.Fatal(record, e)
		}
	}
}

func TestWrappedSurfaceSourceWeights(t *testing.T) {
	t.Parallel()
	w := wrappedCylinder(t)
	var b bytes.Buffer
	if e := WriteWrappedSurfaces(&b, WrappedSurfaceModel{LengthUnit: 1, Surfaces: []WrappedSplineSurface{{Geometry: w}}}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var sid int64
	for id, entity := range f.Entities {
		for i, r := range entity.Records {
			if r.Type != "RATIONAL_B_SPLINE_SURFACE" {
				continue
			}
			sid = id
			for row := range r.Args[0].Items {
				for j := range r.Args[0].Items[row].Items {
					x, e := numeric(r.Args[0].Items[row].Items[j])
					if e != nil {
						t.Fatal(e)
					}
					value := "1."
					if x == 1 {
						value = "3."
					}
					r.Args[0].Items[row].Items[j].Text = value
				}
			}
			entity.Records[i] = r
		}
		f.Entities[id] = entity
	}
	if sid == 0 {
		t.Fatal("no rational surface")
	}
	if _, e := DecodeBSplineSurface(f, sid, 100); e != nil {
		t.Fatal("generic decode changed", e)
	}
	if _, e := DecodeWrappedSurface(context.Background(), f, sid, WrappedSurfaceOptions{}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal(e)
	}
}
func TestIndependentWrappedSurface(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-surface.step")
	if e != nil {
		t.Fatal(e)
	}
	w := wrappedFromOCCT(t, raw)
	if !w.Geometry.SeamCertificate().FirstPartialsMatched[0] {
		t.Fatal("missing exact C1")
	}
	if _, e := ReadWrappedSurfaces(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("standalone reader accepted face graph")
	}
}
func FuzzWrappedSurface(f *testing.F) {
	w := wrappedCylinder(f)
	var b bytes.Buffer
	if e := WriteWrappedSurfaces(&b, WrappedSurfaceModel{LengthUnit: 1, Surfaces: []WrappedSplineSurface{{Geometry: w}}}); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadWrappedSurfaces(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxVertices: 256}, WrappedSurfaceOptions{MaxPoles: 256, MaxOperations: 100000})
	})
}

func TestWrappedSurfaceVAxisAndLogicalFlags(t *testing.T) {
	t.Parallel()
	recipe, _ := wrappedCylinder(t).ExportRecipe()
	d := recipe.Surface.Data()
	poles := []geom.Vec3{}
	weights := []float64{}
	for v := 0; v < d.CountV; v++ {
		for u := 0; u < d.CountU; u++ {
			poles = append(poles, d.Poles[u*d.CountV+v])
			weights = append(weights, d.Weights[u*d.CountV+v])
		}
	}
	s, e := geom.NewBSplineSurface(d.DegreeV, d.DegreeU, d.CountV, d.CountU, poles, d.KnotsV, d.KnotsU, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapV: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedSurfaces(&b, WrappedSurfaceModel{LengthUnit: 1, Surfaces: []WrappedSplineSurface{{Geometry: w}}}); e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedSurfaces(&b, Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil || !m.Surfaces[0].Geometry.SeamCertificate().FirstPartialsMatched[1] {
		t.Fatal(m, e)
	}
	for _, flag := range []string{"F", "U"} {
		b.Reset()
		if e = WriteSurfaces(&b, SurfaceModel{LengthUnit: 1, Surfaces: []SplineSurface{{Geometry: s, ClosedU: "F", ClosedV: flag, SelfIntersect: "U"}}}); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadWrappedSurfaces(&b, Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("unproven logical flag", flag)
		}
	}
}
