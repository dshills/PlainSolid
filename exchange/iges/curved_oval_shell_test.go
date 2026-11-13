package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCurvedOvalShell(t *testing.T) {
	b, e := os.ReadFile("testdata/curved-oval-shell.igs")
	if e != nil {
		t.Fatal(e)
	}
	q, e := ReadCurvedOvalShell(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Edges) != 8 || len(q.Vertices) != 8 || len(q.Loops) != 3 {
		t.Fatal(q)
	}
	var out bytes.Buffer
	if e = WriteCurvedOvalShell(&out, q); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(b, out.Bytes()) {
		t.Fatal("source changed")
	}
}

func TestCurvedOvalShellRejected(t *testing.T) {
	raw, e := os.ReadFile("testdata/curved-oval-shell.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"edge-vertex", "same-use", "missing-pcurve", "unowned-pcurve", "shell-orientation", "loop-chain", "model", "declared-bounds"} {
		t.Run(kind, func(t *testing.T) {
			d, e := readTopologySource(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			for i := range d.entities {
				r := &d.entities[i]
				switch {
				case kind == "edge-vertex" && r.kind == 504:
					r.values[4].text = "3"
				case kind == "same-use" && r.id == 51:
					r.values[5].text = "1"
				case kind == "missing-pcurve" && r.kind == 508:
					r.values[6].text = "0"
				case kind == "unowned-pcurve" && r.id == 51:
					r.values[8].text = "29"
				case kind == "shell-orientation" && r.kind == 514:
					r.values[3].text = "0"
				case kind == "loop-chain" && r.id == 7:
					r.values[4].text = "2"
				case kind == "model" && r.id == 11:
					r.values[23].text = "0.9"
				case kind == "declared-bounds" && r.id == 5:
					r.values[len(r.values)-3].text = "0.6"
				}
			}
			if _, e = certifyCurvedShell(d, Limits{}); e == nil {
				t.Fatal("invalid source topology accepted")
			}
		})
	}
	q, e := ReadCurvedOvalShell(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	q.Edges[0].StartVertex = 3
	var out bytes.Buffer
	if e = WriteCurvedOvalShell(&out, q); e == nil || out.Len() != 0 {
		t.Fatal("stale graph emitted")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxRecords: 2}, {MaxEntities: 5}, {MaxValues: 4}, {MaxControlPoints: 3}} {
		if _, e = ReadCurvedOvalShell(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit ignored")
		}
	}
}
func TestCurvedOvalShellOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	path := filepath.Join(t.TempDir(), "independent.igs")
	oracleCommand(t, path, "write-iges-curved-topology")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	q, e := ReadCurvedOvalShell(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteCurvedOvalShell(&b, q); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "target.igs")
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, file := range []string{path, target} {
		out := filepath.Join(t.TempDir(), "topology.json")
		oracleCommand(t, out, "iges-topology", file)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var report struct {
			Faces, Edges, Vertices, Shells int
			Area                           float64
		}
		if e = json.Unmarshal(data, &report); e != nil || report.Faces != 2 || report.Edges != 8 || report.Vertices != 8 || report.Shells != 1 || report.Area <= 0 {
			t.Fatal(report, e)
		}
		if math.Abs(report.Area-rectangleOracleArea(t, "testdata/curved-oval.igs")) > 1e-7 {
			t.Fatal("area mismatch")
		}
	}
}
func FuzzCurvedOvalShell(f *testing.F) {
	b, e := os.ReadFile("testdata/curved-oval-shell.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCurvedOvalShell(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxRecords: 2000, MaxEntities: 64, MaxValues: 20000, MaxControlPoints: 1024})
	})
}
func TestCurvedOvalShellRejectClosedDeclaration(t *testing.T) {
	raw, e := os.ReadFile("testdata/curved-oval-shell.igs")
	if e != nil {
		t.Fatal(e)
	}
	lines := bytes.Split(raw, []byte("\n"))
	for i, line := range lines {
		if len(line) == 80 && line[72] == 'D' && string(line[73:]) == "0000002" {
			line = append([]byte(nil), line...)
			copy(line[32:40], []byte("       1"))
			lines[i] = line
		}
	}
	if _, e = ReadCurvedOvalShell(bytes.NewReader(bytes.Join(lines, []byte("\n"))), Limits{}); e == nil {
		t.Fatal("closed declaration accepted for open graph")
	}
}
func TestNewCurvedOvalShell(t *testing.T) {
	q, e := NewCurvedOvalShell(curvedSheetFixture(t))
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Edges) != 8 || len(q.Vertices) != 8 || len(q.Loops) != 3 {
		t.Fatal("topology lost")
	}
	var b bytes.Buffer
	if e = WriteCurvedOvalShell(&b, q); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCurvedOvalShell(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "native.igs")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	oracleCommand(t, out, "iges-topology", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Faces, Edges, Vertices, Shells int
		Area                           float64
	}
	if e = json.Unmarshal(raw, &r); e != nil || r.Faces != 2 || r.Edges != 8 || r.Vertices != 8 || r.Shells != 1 {
		t.Fatal(r, e)
	}
	if math.Abs(r.Area-rectangleOracleArea(t, "testdata/curved-oval.igs")) > 1e-7 {
		t.Fatal("native area mismatch", r.Area)
	}
}
func TestNewCurvedOvalShellMetadataAndSource(t *testing.T) {
	raw, e := os.ReadFile("testdata/curved-oval-sheet.igs")
	if e != nil {
		t.Fatal(e)
	}
	s, e := ReadCurvedOvalSheet(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s.Model.LengthUnit = .0254
	s.Model.TrimmedSurfaces[0].Label = "ANNULUS"
	s.Model.TrimmedSurfaces[1].Label = "FILL"
	s.Model.Entities[s.Model.TrimmedSurfaces[0].Surface].Label = "CHART_A"
	q, e := NewCurvedOvalShell(s)
	if e != nil {
		t.Fatal(e)
	}
	if q.Sheet.Model.LengthUnit != .0254 {
		t.Fatal("units lost")
	}
	for i, face := range s.Model.TrimmedSurfaces {
		a := s.Model.Entities[face.Surface].Surface
		b := q.Sheet.Model.Entities[q.Sheet.Model.TrimmedSurfaces[i].Surface].Surface
		if !reflect.DeepEqual(a.Geometry.Data(), b.Geometry.Data()) || a.UStart != b.UStart || a.UEnd != b.UEnd || a.VStart != b.VStart || a.VEnd != b.VEnd {
			t.Fatal("source support changed")
		}
	}
	var b bytes.Buffer
	if e = WriteCurvedOvalShell(&b, q); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(b.Bytes(), []byte("ANNULUS")) || !bytes.Contains(b.Bytes(), []byte("CHART_A")) {
		t.Fatal("source label lost")
	}
	s.Model.CurvesOnSurfaces[2].ModelCurve = s.Model.CurvesOnSurfaces[0].ModelCurve
	if _, e = NewCurvedOvalShell(s); e == nil {
		t.Fatal("incorrect authoritative curve accepted")
	}
}
