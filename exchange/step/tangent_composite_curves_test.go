package step

import (
	"bytes"
	"io"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func tangentCompositeFixture(t testing.TB, closed bool) []byte {
	t.Helper()
	name := "tangent-composite-curve"
	if closed {
		name += "-closed"
	}
	b, e := os.ReadFile("testdata/" + name + ".step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestTangentCompositeCurvesRoundtrip(t *testing.T) {
	t.Parallel()
	for _, closed := range []bool{false, true} {
		raw := tangentCompositeFixture(t, closed)
		if _, e := ReadCompositeCurves(bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("positional API widened")
		}
		m, e := ReadTangentCompositeCurves(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(m.TangentTransitions) != 1 || !m.TangentTransitions[0][0] || m.TangentTransitions[0][1] || m.Curves[0].Closed != closed {
			t.Fatal(m.TangentTransitions)
		}
		a, e := compositeEndpointDerivative(m.Curves[0].Segments[0], true)
		if e != nil || a[0].Cmp(big.NewRat(3, 2)) != 0 || a[1].Sign() != 0 {
			t.Fatal(a, e)
		}
		var b bytes.Buffer
		if e = WriteTangentCompositeCurves(&b, m); e != nil {
			t.Fatal(e)
		}
		r, e := ReadTangentCompositeCurves(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil || !r.TangentTransitions[0][0] {
			t.Fatal(e)
		}
		if e = WriteTangentCompositeCurves(sideDiskShortWriter{}, m); e != io.ErrShortWrite {
			t.Fatal(e)
		}
	}
}
func TestTangentCompositeCurvesRejects(t *testing.T) {
	t.Parallel()
	raw := tangentCompositeFixture(t, false)
	edits := []func(*File){
		func(f *File) {
			sideDiskEditRecord(f, "RATIONAL_B_SPLINE_CURVE", func(r *Record) { r.Args[0].Items[1].Text = "0.5" })
		},
		func(f *File) {
			for _, en := range f.Entities {
				if r, ok := component(en, "RATIONAL_B_SPLINE_CURVE"); ok {
					_ = r
					for _, rr := range en.Records {
						if rr.Type == "B_SPLINE_CURVE_WITH_KNOTS" {
							rr.Args[1].Items[1].Text = "2."
							return
						}
					}
				}
			}
		},
		func(f *File) {
			sideDiskEditRecord(f, "B_SPLINE_CURVE", func(r *Record) { r.Args[1].Items[1] = r.Args[1].Items[2] })
		},
		func(f *File) {
			sideDiskEditRecord(f, "COMPOSITE_CURVE_SEGMENT", func(r *Record) { r.Args[1].Text = "F" })
		},
		func(f *File) {
			sideDiskEditRecord(f, "COMPOSITE_CURVE_SEGMENT", func(r *Record) { r.Args[0].Text = "CONT_SAME_GRADIENT_SAME_CURVATURE" })
		},
	}
	for i, edit := range edits {
		b := sideDiskMutateSource(t, raw, edit)
		m, e := ReadTangentCompositeCurves(bytes.NewReader(b), Limits{})
		if e == nil || len(m.Curves) != 0 || len(m.TangentTransitions) != 0 {
			t.Fatal(i, e)
		}
	}
	m, e := ReadTangentCompositeCurves(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.TangentTransitions[0][1] = true
	var b bytes.Buffer
	if e = WriteTangentCompositeCurves(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("open terminal derivative join")
	}
}
func TestTangentCompositeEndpointProofs(t *testing.T) {
	t.Parallel()
	m, e := ReadTangentCompositeCurves(bytes.NewReader(tangentCompositeFixture(t, false)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	makeCurve := func(p []geom.Vec3, k, w []float64) *geom.BSpline {
		g, e := geom.NewBSpline(2, p, k, w)
		if e != nil {
			t.Fatal(e)
		}
		return g
	}
	// Interior knot and rational endpoint weight require the actual final span,
	// not the entire domain or an unweighted control-edge difference.
	m.Curves[0].Segments[1].Curve.Geometry = makeCurve([]geom.Vec3{{X: 2.5, Y: 1}, {X: 2.5, Y: .5}, {X: 2}, {X: 1.5}}, []float64{0, 0, 0, .5, 1, 1, 1}, []float64{1, 1, .75, 1})
	var b bytes.Buffer
	if e = WriteTangentCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	for _, x := range []float64{1.5, 1} { // zero endpoint tangent, then opposite tangent
		p := []geom.Vec3{{X: 2.5, Y: 1}, {X: x}, {X: 1.5}}
		m.Curves[0].Segments[1].Curve.Geometry = makeCurve(p, []float64{0, 0, 0, 1, 1, 1}, []float64{1, .75, 1})
		b.Reset()
		if e = WriteTangentCompositeCurves(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("zero/opposite tangent", x, e)
		}
	}
}
func TestTangentCompositeClosedLastJoin(t *testing.T) {
	t.Parallel()
	p := [4][3]geom.Vec3{{{X: 1}, {X: 1, Y: 1}, {Y: 1}}, {{Y: 1}, {X: -1, Y: 1}, {X: -1}}, {{X: -1}, {X: -1, Y: -1}, {Y: -1}}, {{Y: -1}, {X: 1, Y: -1}, {X: 1}}}
	c := CompositeSplineCurve{Name: "four exact rational quarters", Closed: true}
	for _, v := range p {
		g, e := geom.NewBSpline(2, v[:], []float64{0, 0, 0, 1, 1, 1}, []float64{1, .75, 1})
		if e != nil {
			t.Fatal(e)
		}
		c.Segments = append(c.Segments, CompositeSplineSegment{Curve: SplineCurve{Geometry: g}, SameSense: true})
	}
	m := TangentCompositeCurveModel{CompositeCurveModel: CompositeCurveModel{Curves: []CompositeSplineCurve{c}, LengthUnit: .001}, TangentTransitions: [][]bool{{true, true, true, true}}}
	var b bytes.Buffer
	if e := WriteTangentCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	r, e := ReadTangentCompositeCurves(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || !r.TangentTransitions[0][3] {
		t.Fatal(e)
	}
}
func TestTangentCompositeMatrixAndLimits(t *testing.T) {
	t.Parallel()
	raw := tangentCompositeFixture(t, false)
	m, e := ReadTangentCompositeCurves(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, flags := range [][][]bool{nil, {}, {{true}}, {{true, false}, {}}} {
		bad := m
		bad.TangentTransitions = flags
		var b bytes.Buffer
		if e = WriteTangentCompositeCurves(&b, bad); e == nil || b.Len() != 0 {
			t.Fatal("matrix/partial", flags, e)
		}
	}
	if _, e = ReadTangentCompositeCurves(bytes.NewReader(raw), Limits{MaxVertices: 3}); e == nil {
		t.Fatal("limits")
	}
}
func FuzzTangentCompositeCurves(f *testing.F) {
	f.Add(tangentCompositeFixture(f, false))
	f.Add(tangentCompositeFixture(f, true))
	f.Add(compositeFixture(f, false))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 65536 {
			t.Skip()
		}
		m, e := ReadTangentCompositeCurves(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 256, MaxValues: 4096, MaxDepth: 12, MaxVertices: 128})
		if e != nil {
			if len(m.Curves) != 0 || len(m.TangentTransitions) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var b bytes.Buffer
		if e = WriteTangentCompositeCurves(&b, m); e != nil {
			t.Fatal(e)
		}
		r, e := ReadTangentCompositeCurves(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil || len(r.Curves) != len(m.Curves) {
			t.Fatal(e)
		}
		for i, flags := range m.TangentTransitions {
			for j, v := range flags {
				if r.TangentTransitions[i][j] != v {
					t.Fatal("declaration drift")
				}
			}
		}
	})
}
func TestTangentCompositeThreeDimensionalParameters(t *testing.T) {
	t.Parallel()
	m, e := ReadTangentCompositeCurves(bytes.NewReader(tangentCompositeFixture(t, false)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, s := range m.Curves[0].Segments {
		g := s.Curve.Geometry
		p, k := g.Poles(), g.Knots()
		for j, v := range p {
			p[j] = geom.Vec3{X: v.X + v.Y, Y: 2*v.X - v.Y, Z: v.X + 3*v.Y}
		}
		for j := range k {
			k[j] = 3 + 2*k[j]
		}
		c, e := geom.NewBSpline(g.Degree(), p, k, g.Weights())
		if e != nil {
			t.Fatal(e)
		}
		m.Curves[0].Segments[i].Curve.Geometry = c
	}
	d, e := compositeEndpointDerivative(m.Curves[0].Segments[0], true)
	if e != nil || d[0].Cmp(big.NewRat(3, 4)) != 0 || d[1].Cmp(big.NewRat(3, 2)) != 0 || d[2].Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal(d, e)
	}
	var b bytes.Buffer
	if e = WriteTangentCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadTangentCompositeCurves(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
}
