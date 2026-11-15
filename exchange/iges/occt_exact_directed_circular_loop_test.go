package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
)

func TestOCCTExactDirectedCircularLoopOriginalIGES(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/occt-directed-two-holes-local-charts.igs")
	if err != nil {
		t.Fatal(err)
	}
	src, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if src.SourceSolid != 1 || src.SourceShell != 3 || src.SourceEdges != 17 || src.SourceVertices != 21 ||
		src.SourceCaps != [2]int{321, 337} || len(src.SourceWalls) != 12 ||
		len(src.Descriptor.Holes) != 2 || len(src.SourceCapLoops[0]) != 3 ||
		len(src.SourceRimEdges[1]) != 12 || len(src.SourceVerticalEdges) != 12 ||
		len(src.SourceParameterCurves) != 8 {
		t.Fatalf("lost original IGES identities or owner paths: %+v", src.ExactDirectedCircularLoopIGES)
	}
	for id, uses := range src.SourceParameterCurves {
		if len(uses) != 4 {
			t.Fatalf("original cylinder face %d has %d owner pcurves", id, len(uses))
		}
	}
	m, err := src.Native.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(m.Volume-(48-math.Pi)) > 1e-8 || math.Abs(m.Area-(88+3*math.Pi)) > 1e-8 {
		t.Fatalf("native material differs: %+v %v", m, err)
	}
	var out, canonical bytes.Buffer
	if err = WriteOCCTExactDirectedCircularLoopSource(ctx, &out, src.Source, Limits{}); err != nil || !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("original IGES bytes changed: %v", err)
	}
	if err = WriteExactDirectedCircularLoopExtrusion(ctx, &canonical, src.Descriptor); err != nil {
		t.Fatal(err)
	}
	dst, err := ReadExactDirectedCircularLoopExtrusion(ctx, &canonical, Limits{})
	if err != nil || len(dst.Native.Faces()) != 14 {
		t.Fatalf("canonical IGES lost native topology: %v", err)
	}
}

func TestOCCTExactDirectedCircularLoopRejectsOriginalIGESDrift(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/occt-directed-two-holes-shared-chart.igs")
	if err != nil {
		t.Fatal(err)
	}
	if q, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{}); err == nil || q.Source != nil ||
		!strings.Contains(err.Error(), "circular endpoints differ from original vertices") {
		t.Fatalf("rounded authored arc did not fail its exact model endpoint proof: %v", err)
	}
}

func TestOCCTExactDirectedCircularLoopRejectsEditedOwnerIGES(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/occt-directed-two-holes-local-charts.igs")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id               int
		from, to, reason string
	}{
		{173, "0.,-2.,0.,0.,0.,0.", "1.,-2.,0.,1.,0.,0.", "not cardinal"},
		{177, "-90.,0.,0.,-90.,-2.,0.", "270.,0.,0.,270.,-2.,0.", "original unwrapped chart"},
		{49, "1.,0.,0.,2.,", "1.,0.,0.,3.,", "circular endpoints differ"},
		{171, "1,0,173", "1,0,179", "not an exact cylinder generator"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			lines := bytes.Split(raw, []byte("\n"))
			changed := false
			for i, line := range lines {
				if len(line) != 80 || line[72] != 'P' {
					continue
				}
				id, _ := field(string(line[64:72]))
				if id != tc.id || !bytes.Contains(line[:64], []byte(tc.from)) {
					continue
				}
				if len(tc.from) != len(tc.to) {
					t.Fatal("fixture edit changes length")
				}
				lines[i] = bytes.Replace(line, []byte(tc.from), []byte(tc.to), 1)
				changed = true
				break
			}
			if !changed {
				t.Fatal("missing original parameter record", tc.id)
			}
			bad := bytes.Join(lines, []byte("\n"))
			if q, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(bad), Limits{}); err == nil || q.Source != nil ||
				!strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("edited owner was not rejected at its exact boundary: %v", err)
			}
			var out bytes.Buffer
			if err := WriteOCCTExactDirectedCircularLoopSource(ctx, &out, bad, Limits{}); err == nil || out.Len() != 0 {
				t.Fatal("re-export wrote an unproved source")
			}
		})
	}
}

func TestOCCTExactDirectedCircularLoopOriginalIGESLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_DIRECTED_IGES_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES")
	if author == "" || reader == "" {
		t.Skip("optional pinned independent IGES author and reader")
	}
	ctx := context.Background()
	tmp := t.TempDir()
	file := filepath.Join(tmp, "original.igs")
	if b, err := exec.Command(author, "iges-local-chart", file).CombinedOutput(); err != nil {
		t.Fatalf("independent author: %v %s", err, b)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("testdata/occt-directed-two-holes-local-charts.igs")
	if err != nil {
		t.Fatal(err)
	}
	entities := func(b []byte) []byte {
		var out bytes.Buffer
		for _, l := range bytes.Split(b, []byte("\n")) {
			if len(l) == 80 && (l[72] == 'D' || l[72] == 'P') {
				out.Write(l)
			}
		}
		return out.Bytes()
	}
	if !bytes.Equal(entities(raw), entities(committed)) {
		t.Fatal("pinned independent author changed original directory/parameter records")
	}
	p, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var original, canonical bytes.Buffer
	if err = WriteOCCTExactDirectedCircularLoopSource(ctx, &original, p.Source, Limits{}); err != nil {
		t.Fatal(err)
	}
	if err = WriteExactDirectedCircularLoopExtrusion(ctx, &canonical, p.Descriptor); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string][]byte{"original.igs": raw, "replay.igs": original.Bytes(), "canonical.igs": canonical.Bytes()} {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, v, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := exec.Command(reader, "read", path).CombinedOutput()
		i := bytes.LastIndexByte(got, '{')
		var m struct {
			Solids, Shells, Faces int
			Volume, Area          float64
		}
		if err != nil || i < 0 || json.Unmarshal(got[i:], &m) != nil || m.Solids != 1 || m.Shells != 1 || m.Faces != 14 ||
			math.Abs(m.Volume-(48-math.Pi)) > 1e-8 || math.Abs(m.Area-(88+3*math.Pi)) > 1e-8 {
			t.Fatalf("independent IGES read %s: %+v %v %s", name, m, err, got)
		}
	}
}
