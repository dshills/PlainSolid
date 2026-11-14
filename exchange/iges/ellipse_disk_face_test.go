package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"strings"
	"testing"
)

func TestEllipseDiskFace(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ellipse-disk.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipseDiskFace(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	face, c, e := m.NativeFace()
	if e != nil {
		t.Fatal(e)
	}
	for _, angle := range []float64{0, .2, 1.7, 4.8, 2 * math.Pi} {
		uv, e := face.Outer[0].Curve.EvalUV(angle / (2 * math.Pi))
		if e != nil {
			t.Fatal(e)
		}
		p, e := face.Surface.Eval(uv.X, uv.Y)
		q, er := c.Eval(angle)
		if e != nil || er != nil || p.Distance(q) > 1e-14 {
			t.Fatal(p, q, e, er)
		}
	}
	// Native analytic surface and model curve use existing dependency-free persistence.
	s, e := parametric.EncodeSurface(face.Surface)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = parametric.DecodeSurface(s, 64); e != nil {
		t.Fatal(e)
	}
	curve, e := parametric.EncodeCurve(c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = parametric.DecodeCurve(curve, 64); e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteEllipseDiskFaceJSON(&native, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadEllipseDiskFaceJSON(bytes.NewReader(native.Bytes()), Limits{})
	if e != nil || n.SourceFaceID != m.SourceFaceID {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteEllipseDiskFace(&out, n); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseDiskFace(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}

	var envelope ellipseDiskEnvelope
	if e = json.Unmarshal(native.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	envelope.Face.Model.Curve.Z = 4
	forged, e := json.Marshal(envelope)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseDiskFaceJSON(bytes.NewReader(forged), Limits{}); e == nil {
		t.Fatal("forged native geometry")
	}
	x := m
	x.Model.Curve.Z = 4
	var empty bytes.Buffer
	if WriteEllipseDiskFaceJSON(&empty, x) == nil || empty.Len() != 0 {
		t.Fatal("stale native accepted")
	}
	if _, e = ReadEllipseDiskFaceJSON(bytes.NewReader(bytes.Replace(native.Bytes(), []byte(`"Version":1`), []byte(`"Version":2`), 1)), Limits{}); e == nil {
		t.Fatal("version")
	}
	for _, pair := range [][2]string{{"144,3,1,0,11;", "144,3,0,0,11;"}, {"142,0,3,13,15,3;", "142,0,3,13,13,3;"}, {"142,0,3,13,15,3;", "142,0,5,13,15,3;"}, {"123,0.,0.,1.;", "123,0.,0.,-1.;"}, {"104,4.,0.,25.,0.,0.,-100.,3.,5.,0.,5.,0.;", "104,4.,0.,25.,0.,0.,-100.,4.,5.,0.,5.,0.;"}} {
		bad := append([]byte(nil), raw...)
		off := bytes.Index(bad, []byte(pair[0]))
		if off < 0 {
			t.Fatal(pair)
		}
		copy(bad[off:off+64], []byte(pair[1]+strings.Repeat(" ", 64-len(pair[1]))))
		if _, e = ReadEllipseDiskFace(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("source mutation accepted", pair)
		}
	}
	for _, l := range []Limits{{MaxEntities: 7}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 4}} {
		if _, e = ReadEllipseDiskFace(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	if _, e = ReadEllipseConic(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("atom admitted face")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPSE_DISK"); oracle != "" {
		p := filepath.Join(t.TempDir(), "disk.igs")
		if e = os.WriteFile(p, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		for _, p := range []string{"testdata/occt-ellipse-disk.igs", p} {
			b, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", b, e)
			}
			var v struct {
				Faces         int
				Area, CenterZ float64
			}
			at := bytes.LastIndex(b, []byte("{"))
			if at < 0 || json.Unmarshal(bytes.TrimSpace(b[at:]), &v) != nil || v.Faces != 1 || math.Abs(v.Area-10*math.Pi) > 1e-11 || v.CenterZ != 3 {
				t.Fatalf("%s", b)
			}
		}
	}
}
func FuzzEllipseDiskFace(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-ellipse-disk.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		m, e := ReadEllipseDiskFace(bytes.NewReader(b), Limits{})
		if e == nil {
			var out bytes.Buffer
			if e = WriteEllipseDiskFace(&out, m); e != nil {
				t.Fatal(e)
			}
		}
	})
}
