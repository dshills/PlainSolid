package step

import (
	"bytes"
	"io"
	"os"
	"plainsolid/geom"
	"testing"
)

func compositeFixture(t testing.TB, closed bool) []byte {
	t.Helper()
	name := "composite-curve"
	if closed {
		name += "-closed"
	}
	b, e := os.ReadFile("testdata/" + name + ".step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestCompositeCurvesIndependentRoundtrip(t *testing.T) {
	t.Parallel()
	for _, closed := range []bool{false, true} {
		raw := compositeFixture(t, closed)
		m, e := ReadCompositeCurves(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		n := 2
		if closed {
			n = 3
		}
		if m.Name != "composite product" || m.LengthUnit != .001 || len(m.Curves) != 1 || len(m.Curves[0].Segments) != n || m.Curves[0].Closed != closed || m.Curves[0].Segments[1].SameSense {
			t.Fatal(m)
		}
		var b bytes.Buffer
		if e = WriteCompositeCurves(&b, m); e != nil {
			t.Fatal(e)
		}
		r, e := ReadCompositeCurves(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil || r.Name != m.Name || r.Curves[0].Closed != closed {
			t.Fatal(e)
		}
		for i, c := range m.Curves[0].Segments {
			other := r.Curves[0].Segments[i]
			if c.SameSense != other.SameSense || c.Curve.Name != other.Curve.Name {
				t.Fatal("basis metadata")
			}
			a, _ := geom.NewExactSplineCurve(c.Curve.Geometry, geom.SurfacePatchOptions{})
			z, _ := geom.NewExactSplineCurve(other.Curve.Geometry, geom.SurfacePatchOptions{})
			eq, e := geom.CompareExactCurves(a, z, geom.ExactCurveComparisonOptions{})
			if e != nil || !eq.Complete || !eq.Equal {
				t.Fatal("source basis changed", e)
			}
		}
		if _, e = ReadCurves(bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("legacy widened")
		}
	}
}
func TestCompositeCurvesRejectGraph(t *testing.T) {
	t.Parallel()
	raw := compositeFixture(t, false)
	edits := []func(*File){
		func(f *File) {
			sideDiskEditRecord(f, "COMPOSITE_CURVE_SEGMENT", func(r *Record) { r.Args[0].Text = "CONTSAMEGRADIENT" })
		},
		func(f *File) {
			sideDiskEditRecord(f, "COMPOSITE_CURVE_SEGMENT", func(r *Record) { r.Args[1].Text = "F" })
		},
		func(f *File) {
			sideDiskEditRecord(f, "COMPOSITE_CURVE", func(r *Record) { r.Args[1].Items[1] = r.Args[1].Items[0] })
		},
		func(f *File) { sideDiskEditRecord(f, "COMPOSITE_CURVE", func(r *Record) { r.Args[2].Text = "F" }) },
		func(f *File) {
			var root int64
			for id, en := range f.Entities {
				if en.Records[0].Type == "COMPOSITE_CURVE" {
					root = id
				}
			}
			sideDiskEditRecord(f, "COMPOSITE_CURVE_SEGMENT", func(r *Record) { r.Args[2].Ref = root })
		},
		func(f *File) {
			sideDiskEditRecord(f, "CARTESIAN_POINT", func(r *Record) { r.Args[1].Items[0].Text = "0.00000000000000001" })
		},
		func(f *File) { sideDiskEditRecord(f, "PRODUCT", func(r *Record) { r.Args[0].Text = "other" }) },
		func(f *File) {
			sideDiskEditRecord(f, "RATIONAL_B_SPLINE_CURVE", func(r *Record) { r.Args[0].Items[0].Text = "3." })
		},
	}
	for i, edit := range edits {
		b := sideDiskMutateSource(t, raw, edit)
		m, e := ReadCompositeCurves(bytes.NewReader(b), Limits{})
		if e == nil || len(m.Curves) != 0 {
			t.Fatal("invalid/partial", i, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 32}, {MaxEntities: 8}, {MaxVertices: 3}} {
		if _, e := ReadCompositeCurves(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
func TestCompositeCurvesWriterAndMultiple(t *testing.T) {
	t.Parallel()
	m, e := ReadCompositeCurves(bytes.NewReader(compositeFixture(t, false)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Curves = append(m.Curves, m.Curves[0])
	m.Curves[1].Name = "second independent definition"
	var b bytes.Buffer
	if e = WriteCompositeCurves(&b, m); e != nil {
		t.Fatal(e)
	}
	r, e := ReadCompositeCurves(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || len(r.Curves) != 2 {
		t.Fatal(e)
	}
	b.Reset()
	m.Curves[0].Closed = true
	if e = WriteCompositeCurves(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("false closure/partial output", e)
	}
	m.Curves[0].Closed = false
	if e = WriteCompositeCurves(sideDiskShortWriter{}, m); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
func FuzzCompositeCurves(f *testing.F) {
	f.Add(compositeFixture(f, false))
	f.Add(compositeFixture(f, true))
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 65536 {
			t.Skip()
		}
		m, e := ReadCompositeCurves(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 256, MaxValues: 4096, MaxDepth: 12, MaxVertices: 128})
		if e != nil {
			if len(m.Curves) != 0 {
				t.Fatal("partial result")
			}
			return
		}
		var b bytes.Buffer
		if e = WriteCompositeCurves(&b, m); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadCompositeCurves(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
			t.Fatal(e)
		}
	})
}
func TestCompositeCurvesPositionalAndClosureClaims(t *testing.T) {
	t.Parallel()
	raw := compositeFixture(t, false)
	for _, mode := range []string{"gap", "false-close", "inner-break", "shared-point", "orphan"} {
		b := sideDiskMutateSource(t, raw, func(f *File) {
			switch mode {
			case "gap":
				for _, id := range f.Order {
					if en := f.Entities[id]; en.Records[0].Type == "B_SPLINE_CURVE_WITH_KNOTS" {
						pid := en.Records[0].Args[2].Items[1].Ref
						f.Entities[pid].Records[0].Args[1].Items[0].Text = "1.5"
						return
					}
				}
			case "false-close":
				for _, id := range f.Order {
					r := f.Entities[id].Records[0]
					if r.Type == "COMPOSITE_CURVE_SEGMENT" && r.Args[0].Text == "DISCONTINUOUS" {
						r.Args[0].Text = "CONTINUOUS"
						return
					}
				}
			case "inner-break":
				sideDiskEditRecord(f, "COMPOSITE_CURVE_SEGMENT", func(r *Record) { r.Args[0].Text = "DISCONTINUOUS" })
			case "shared-point":
				var point int64
				for _, id := range f.Order {
					en := f.Entities[id]
					if en.Records[0].Type == "B_SPLINE_CURVE_WITH_KNOTS" {
						point = en.Records[0].Args[2].Items[1].Ref
					}
				}
				sideDiskEditRecord(f, "B_SPLINE_CURVE", func(r *Record) { r.Args[1].Items[2].Ref = point })
			case "orphan":
				id := int64(9999)
				f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
				f.Order = append(f.Order, id)
			}
		})
		if _, e := ReadCompositeCurves(bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal(mode)
		}
	}
}
