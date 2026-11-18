package step

import (
	"bytes"
	"context"
	"math"
	"os"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func refinedLensFixture(t testing.TB) SplineLensBody {
	m := splineLensFixture(t)
	p, e := geom.NewSurfacePatch(m.Lower, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.5, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(.5, 2, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Lower, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	m.Name = "Independent lower knot partition"
	return m
}
func TestRefinedSplineLens(t *testing.T) {
	t.Parallel()
	m := refinedLensFixture(t)
	var b bytes.Buffer
	if e := WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Upper.Data(), got.Upper.Data()) || !reflect.DeepEqual(m.Lower.Data(), got.Lower.Data()) {
		t.Fatal("basis was changed")
	}
	if _, e = SplineLensFromSolid(got.Solid, got.Center, got.Radii, got.Name, got.LengthUnit, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	// Each original seam's whole model curve is compared exactly; no sampled
	// agreement or reconstruction of the lower support is used.
	o, _ := (CurvedOvalOptions{}).defaults()
	q := QuadraticOval{Center: m.Center, Radii: m.Radii, QuarterWeight: .5}
	_, a, e := certifyCurvedOvalMode(CurvedOvalFace{Geometry: m.Upper, Outer: q}, o, true)
	if e != nil {
		t.Fatal(e)
	}
	_, lower, e := certifyCurvedOvalMode(CurvedOvalFace{Geometry: m.Lower, Outer: q}, o, true)
	if e != nil {
		t.Fatal(e)
	}
	for i := range a {
		same, e := geom.CompareExactCurvesContext(context.Background(), a[i], lower[i], geom.ExactCurveComparisonOptions{})
		if e != nil || !same.Complete || !same.Equal {
			t.Fatal(same, e)
		}
	}
}
func TestRefinedSplineLensRejections(t *testing.T) {
	t.Parallel()
	m := refinedLensFixture(t)
	var b bytes.Buffer
	if e := WriteSplineLensBody(&b, m, CurvedOvalOptions{MaxPoles: 18}); e == nil || b.Len() != 0 {
		t.Fatal("pole budget ignored", e)
	}
	// An interior quarter crossing cannot be rounded into a parameter cut or
	// replaced by a single sampled model curve in this deliberately bounded API.
	m = splineLensFixture(t)
	p, e := geom.NewSurfacePatch(m.Lower, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.625, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Lower, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
		t.Fatal("unsupported interior knot crossing accepted", e)
	}
}
func TestRefinedBothSplineLensSupports(t *testing.T) {
	t.Parallel()
	m := refinedLensFixture(t)
	p, e := geom.NewSurfacePatch(m.Upper, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Upper, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil || !reflect.DeepEqual(got.Upper.Data(), m.Upper.Data()) || !reflect.DeepEqual(got.Lower.Data(), m.Lower.Data()) {
		t.Fatal("independent bases not retained", e)
	}
}
func TestIndependentRefinedLens(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/refined-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Lower.Data().CountU != 4 || m.Lower.Data().CountV != 5 {
		t.Fatal("source partition")
	}
	var b bytes.Buffer
	if e = WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	copy, e := ReadSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil || !reflect.DeepEqual(copy.Lower.Data(), m.Lower.Data()) {
		t.Fatal("source basis changed", e)
	}
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	face, _ := simple(f, m.SourceFaceIDs[1], "ADVANCED_FACE", 4)
	sid, _ := ref(face.Args[2])
	surface, e := simple(f, sid, "B_SPLINE_SURFACE_WITH_KNOTS", 13)
	if e != nil {
		t.Fatal(e)
	}
	pid, _ := ref(surface.Args[3].Items[1].Items[1])
	p, _ := simple(f, pid, "CARTESIAN_POINT", 2)
	z, _ := numeric(p.Args[1].Items[2])
	p.Args[1].Items[2] = Value{Kind: Number, Text: real(math.Nextafter(z, math.Inf(1)))}
	ovalSet(f, pid, p)
	if _, e = ReadSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("changed refined support accepted")
	}
}
func TestRefinedLensRejectsFullMultiplicityBreak(t *testing.T) {
	t.Parallel()
	m := splineLensFixture(t)
	var e error
	m.Lower, e = m.Lower.InsertUKnot(.5, 3)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
		t.Fatal("full-multiplicity break accepted", e)
	}
}
