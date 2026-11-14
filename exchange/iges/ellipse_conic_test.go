package iges

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestEllipseConic(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ellipse-conic.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipseConic(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.LengthUnit != .001 || m.SourceCurveID != 1 || m.SourceTransformID != 3 {
		t.Fatal(m)
	}
	g, e := m.Curve.Geometry()
	if e != nil {
		t.Fatal(e)
	}
	p, e := g.Eval(.375)
	if e != nil || p.Distance(geom.Vec3{X: 1, Y: 2 + 5*math.Cos(.375), Z: 3 + 2*math.Sin(.375)}) > 1e-14 {
		t.Fatal(p, e)
	}
	m.Curve.Metadata = CylinderDatumMetadata{Label: "ELLIPSE", Subscript: 7}
	m.Curve.PlacementMetadata = CylinderDatumMetadata{Label: "POSE", Subscript: 2}
	var b bytes.Buffer
	if e = WriteEllipseConic(&b, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadEllipseConic(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || n.Curve.Coefficients != m.Curve.Coefficients || n.Curve.Metadata != m.Curve.Metadata || *n.Curve.Placement != *m.Curve.Placement {
		t.Fatal(n, e)
	}
	if path := os.Getenv("PLAINSOLID_IGES_ELLIPSE_OUTPUT"); path != "" {
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	b.Reset()
	m.Curve.Placement = nil
	m.Curve.PlacementMetadata = CylinderDatumMetadata{}
	m.Curve.Z = 2.5
	if e = WriteEllipseConic(&b, m); e != nil {
		t.Fatal(e)
	}
	n, e = ReadEllipseConic(&b, Limits{})
	if e != nil || n.SourceTransformID != 0 || n.Curve.Z != 2.5 {
		t.Fatal(e)
	}
	if e = WriteEllipseConic(cylinderShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	for _, mut := range []func(*EllipseConicModel){func(m *EllipseConicModel) { m.Curve.Coefficients[0] = 3 }, func(m *EllipseConicModel) { m.Curve.Coefficients[1] = 1 }, func(m *EllipseConicModel) { m.Curve.Coefficients[5] = 0 }, func(m *EllipseConicModel) { m.LengthUnit = 0 }, func(m *EllipseConicModel) { x := geom.Identity(); x[0][0] = -1; m.Curve.Placement = &x }, func(m *EllipseConicModel) { x := geom.Identity(); x[0][1] = .1; m.Curve.Placement = &x }} {
		x := m
		mut(&x)
		b.Reset()
		if e = WriteEllipseConic(&b, x); e == nil || b.Len() != 0 {
			t.Fatal("invalid output")
		}
	}
}
func TestEllipseConicSourceRejection(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ellipse-conic.igs")
	if e != nil {
		t.Fatal(e)
	}
	// Equal-length edits preserve IGES record ownership/columns and target semantics.
	for _, p := range [][2]string{{"104,4.,0.,25.", "104,3.,0.,25."}, {"0.,5.,0.,5.,0.;", "0.,5.,0.,4.,0.;"}, {"124,0.,0.,1.", "128,0.,0.,1."}, {"104,4.,0.,25.", "104,4.,1.,25."}} {
		bad := bytes.Replace(raw, []byte(p[0]), []byte(p[1]), 1)
		if bytes.Equal(bad, raw) {
			t.Fatal("missing mutation")
		}
		m, e := ReadEllipseConic(bytes.NewReader(bad), Limits{})
		if e == nil || m.SourceCurveID != 0 {
			t.Fatal("invalid source", p)
		}
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if len(line) == 80 && line[72] == 'P' && strings.HasPrefix(line, "104,") {
			data := strings.TrimRight(line[:64], " ")
			data = strings.Replace(data, "-100.,0.,5.", "-100.,0.1,5.", 1)
			lines[i] = fmt.Sprintf("%-64s", data) + line[64:]
		}
	}
	if _, e = ReadEllipseConic(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
		t.Fatal("rounded decimal source accepted")
	}
	if _, e = ReadEllipseConic(bytes.NewReader(raw), Limits{MaxEntities: 1}); e == nil {
		t.Fatal("limit")
	}
	if _, e = Read(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("generic codec widened")
	}
}
func FuzzEllipseConic(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-ellipse-conic.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 16384 {
			t.Skip()
		}
		m, e := ReadEllipseConic(bytes.NewReader(b), Limits{MaxBytes: 16384, MaxRecords: 128, MaxEntities: 2, MaxValues: 256})
		if e != nil && m.SourceCurveID != 0 {
			t.Fatal("partial result")
		}
	})
}
