package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEllipseAnnulusFace(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ellipse-annulus.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipseAnnulusFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	face, outer, inner, e := m.NativeFace()
	if e != nil || len(face.Holes) != 1 || !face.Holes[0][0].Reversed {
		t.Fatal(e)
	}
	for _, u := range []float64{0, .17, .5, .9, 1} {
		uv, e := face.Holes[0][0].Curve.EvalUV(u)
		if e != nil {
			t.Fatal(e)
		}
		p, e := face.Surface.Eval(uv.X, uv.Y)
		q, er := inner.Eval((1 - u) * 2 * math.Pi)
		if e != nil || er != nil || p.Distance(q) > 1e-13 {
			t.Fatal(p, q, e, er)
		}
	}
	_ = outer
	var native bytes.Buffer
	if e = WriteEllipseAnnulusFaceJSON(&native, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadEllipseAnnulusFaceJSON(bytes.NewReader(native.Bytes()), Limits{})
	if e != nil || n.Inner.SourceBoundaryID != m.Inner.SourceBoundaryID {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteEllipseAnnulusFace(&out, n); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseAnnulusFace(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseDiskFace(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("disk widened")
	}
	disk, e := os.ReadFile("testdata/occt-ellipse-disk.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseAnnulusFace(bytes.NewReader(disk), Limits{}); e == nil {
		t.Fatal("annulus accepted disk")
	}
	for _, coeff := range [][6]float64{m.Outer.Model.Curve.Coefficients, {1, 0, 4, 0, 0, -16}, {1, 0, 4, 0, 0, -36}} {
		x := m
		x.Inner.Model.Curve.Coefficients = coeff
		var b bytes.Buffer
		if WriteEllipseAnnulusFace(&b, x) == nil || b.Len() != 0 {
			t.Fatal("touch/cross accepted")
		}
	}
	for _, pair := range [][2]string{{"144,3,1,1,11,17;", "144,3,1,1,11,11;"}, {"142,0,3,19,21,3;", "142,0,3,13,21,3;"}, {"142,0,3,19,21,3;", "142,0,5,19,21,3;"}, {"104,1.,0.,4.,0.,0.,-4.,3.,2.,0.,2.,0.;", "104,1.,0.,4.,0.,0.,-4.,4.,2.,0.,2.,0.;"}} {
		bad := append([]byte(nil), raw...)
		off := bytes.Index(bad, []byte(pair[0]))
		if off < 0 {
			t.Fatal(pair)
		}
		copy(bad[off:off+64], []byte(pair[1]+strings.Repeat(" ", 64-len(pair[1]))))
		if _, e = ReadEllipseAnnulusFace(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("owner mutation accepted")
		}
	}
	var env ellipseAnnulusEnvelope
	if e = json.Unmarshal(native.Bytes(), &env); e != nil {
		t.Fatal(e)
	}
	env.Face.Inner.SourceBoundaryID = env.Face.Outer.SourceBoundaryID
	b, e := json.Marshal(env)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseAnnulusFaceJSON(bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("stale ancestry")
	}
	for _, l := range []Limits{{MaxEntities: 10}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 4}} {
		if _, e = ReadEllipseAnnulusFace(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPSE_ANNULUS"); oracle != "" {
		p := filepath.Join(t.TempDir(), "annulus.igs")
		if e = os.WriteFile(p, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		for _, p := range []string{"testdata/occt-ellipse-annulus.igs", p} {
			b, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", b, e)
			}
			var v struct {
				Faces         int
				Area, CenterZ float64
			}
			at := bytes.LastIndex(b, []byte("{"))
			if at < 0 || json.Unmarshal(bytes.TrimSpace(b[at:]), &v) != nil || v.Faces != 1 || math.Abs(v.Area-8*math.Pi) > 1e-11 || v.CenterZ != 3 {
				t.Fatalf("%s", b)
			}
		}
	}
}
func FuzzEllipseAnnulusFace(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-ellipse-annulus.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		m, e := ReadEllipseAnnulusFace(bytes.NewReader(b), Limits{})
		if e == nil {
			var out bytes.Buffer
			if e = WriteEllipseAnnulusFace(&out, m); e != nil {
				t.Fatal(e)
			}
		}
	})
}
