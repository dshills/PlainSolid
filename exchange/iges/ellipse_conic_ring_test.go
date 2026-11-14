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

func TestEllipseConicRing(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ellipse-ring.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipseConicRing(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, q := range m.Members {
		if q.StartQuarter != i || q.SourceCurveID == 0 || q.SourceTransformID == 0 {
			t.Fatal(q)
		}
	}
	var b bytes.Buffer
	if e = WriteEllipseConicRing(&b, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadEllipseConicRing(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || n.Members[3].StartQuarter != 3 {
		t.Fatal(n, e)
	}
	if _, e = ReadQuarterEllipseConic(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("quarter accepted ring")
	}
	if _, e = ReadEllipseConic(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("full accepted ring")
	}
	for _, mut := range []func(*EllipseConicRing){func(x *EllipseConicRing) { x.Members[1] = x.Members[0] }, func(x *EllipseConicRing) { x.Members[2].Curve.Z = 1 }, func(x *EllipseConicRing) { x.Members[3].LengthUnit = 1 }} {
		x := m
		mut(&x)
		var out bytes.Buffer
		if WriteEllipseConicRing(&out, x) == nil || out.Len() != 0 {
			t.Fatal("non-atomic rejection")
		}
	}
	for _, lim := range []Limits{{MaxEntities: 4}, {MaxRecords: 2}, {MaxBytes: 80}, {MaxValues: 4}} {
		if _, e = ReadEllipseConicRing(bytes.NewReader(raw), lim); e == nil {
			t.Fatal("budget accepted")
		}
	}
	// Corrupt the composite membership without changing fixed record columns.
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 80 && line[72] == 'P' && strings.HasPrefix(string(line), "102,4,") {
			bad := append([]byte(nil), raw...)
			off := bytes.Index(bad, line)
			copy(bad[off:off+6], []byte("102,3,"))
			if _, e = ReadEllipseConicRing(bytes.NewReader(bad), Limits{}); e == nil {
				t.Fatal("bad count")
			}
		}
	}

	for _, replacement := range []string{"102,4,3,3,9,11;", "102,4,3,9,7,11;", "102,4,3,7,9,1;"} {
		bad := append([]byte(nil), raw...)
		off := bytes.Index(bad, []byte("102,4,3,7,9,11;"))
		if off < 0 {
			t.Fatal("fixture root")
		}
		copy(bad[off:off+64], []byte(replacement+strings.Repeat(" ", 64-len(replacement))))
		if _, e = ReadEllipseConicRing(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("membership accepted")
		}
	}
	for start := 0; start < 4; start++ {
		x := m
		for i := range x.Members {
			x.Members[i].StartQuarter = (start + i) % 4
			x.Members[i].Curve.Placement = nil
			x.Members[i].Curve.PlacementMetadata = CylinderDatumMetadata{}
		}
		var out bytes.Buffer
		if e = WriteEllipseConicRing(&out, x); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadEllipseConicRing(&out, Limits{}); e != nil {
			t.Fatal(e)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPSE_RING"); oracle != "" {
		p := filepath.Join(t.TempDir(), "ring.igs")
		if e = os.WriteFile(p, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		for _, p := range []string{"testdata/occt-ellipse-ring.igs", p} {
			out, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil || !bytes.Contains(out, []byte(`"Edges":4`)) {
				t.Fatalf("%s %v", out, e)
			}
			var measure struct {
				Edges  int
				Length float64
			}
			at := bytes.LastIndex(out, []byte("{"))
			if at < 0 || json.Unmarshal(bytes.TrimSpace(out[at:]), &measure) != nil || math.Abs(measure.Length-23.01311259606501) > 1e-10 {
				t.Fatalf("measure %s", out)
			}

		}
	}
}
func FuzzEllipseConicRing(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-ellipse-ring.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("102,4,1,1,1,1;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		m, e := ReadEllipseConicRing(bytes.NewReader(b), Limits{MaxBytes: 32768, MaxEntities: 6, MaxRecords: 256, MaxValues: 1024})
		if e == nil {
			var out bytes.Buffer
			if e = WriteEllipseConicRing(&out, m); e != nil {
				t.Fatal(e)
			}
		}
	})
}
