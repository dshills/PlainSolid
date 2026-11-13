package document

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func partitionedLensDocument(t testing.TB) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/partitioned-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadPartitionedSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestPartitionedLensDocumentChain(t *testing.T) {
	d := partitionedLensDocument(t)
	s, _ := d.Snapshot()
	label := s.Labels["step:body-000001"]
	original := label.Attributes["step.lens.partitioned.provenance"].Text
	var p partitionedLensProvenance
	if e := json.Unmarshal([]byte(original), &p); e != nil {
		t.Fatal(e)
	}
	if len(p.Pieces) != 6 || len(s.Trimmed["body-000001"].Edges()) != 4 {
		t.Fatal("source/native topology conflated")
	}
	n := s.Nodes["body-000001"]
	n.Name = "Partitioned lens"
	s.Nodes["body-000001"] = n
	s.LengthUnit = .0254
	label.Attributes["step.lens.upper.name"] = Attribute{Kind: "string", Text: "Upper chart"}
	label.Attributes["step.lens.lower.name"] = Attribute{Kind: "string", Text: "Lower original knot"}
	s.Labels["step:body-000001"] = label
	d, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if s.Labels["step:body-000001"].Attributes["step.lens.partitioned.provenance"].Text != original {
		t.Fatal("source ancestry changed")
	}
	var out bytes.Buffer
	if e = s.WritePartitionedSplineLensSTEP(&out); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadPartitionedSplineLensBody(bytes.NewReader(out.Bytes()), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Body.Name != n.Name || m.Body.LengthUnit != .0254 || m.Body.FaceNames != ([2]string{"Upper chart", "Lower original knot"}) {
		t.Fatal("names/units")
	}
	for i, a := range p.Pieces {
		b := m.Pieces[i]
		if a.Quarter != b.Quarter || a.Start.Cmp(b.Start) != 0 || a.End.Cmp(b.End) != 0 || a.Cells != b.Cells {
			t.Fatal("lost rational lineage")
		}
	}
	again, e := FromPartitionedSplineLensSTEP(m.Body, 2)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := again.Snapshot()
	if !reflect.DeepEqual(a.Trimmed["body-000001"].Faces()[0].Surface, s.Trimmed["body-000001"].Faces()[0].Surface) {
		t.Fatal("original chart changed")
	}
	out.Reset()
	if e = s.WriteSplineLensSTEP(&out); e == nil || out.Len() != 0 {
		t.Fatal("old adapter accepted partitioned ancestry")
	}
}
func TestPartitionedLensDocumentRejectsStale(t *testing.T) {
	for _, kind := range []string{"source", "rebound-cell", "rebound-interval", "rebound-duplicate", "geometry", "placement", "instances"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := partitionedLensDocument(t).Snapshot()
			l := s.Labels["step:body-000001"]
			var p partitionedLensProvenance
			if e := json.Unmarshal([]byte(l.Attributes["step.lens.partitioned.provenance"].Text), &p); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "source":
				p.Pieces[0].SourceEdgeID++
			case "rebound-cell":
				p.Pieces[0].Cells[1].UMax = .75
			case "rebound-interval":
				p.Pieces[0].End = big.NewRat(2, 3)
			case "rebound-duplicate":
				p.Pieces[1].SourceEdgeID = p.Pieces[0].SourceEdgeID
			case "geometry":
				p.Geometry = "stale"
			case "placement":
				n := s.Nodes["body-000001"]
				n.Transform = geom.Translation(geom.Vec3{X: 1})
				s.Nodes["body-000001"] = n
			case "instances":
				n := s.Nodes["body-000001"]
				s.Nodes["duplicate"] = n
			}
			raw, e := json.Marshal(p)
			if e != nil {
				t.Fatal(e)
			}
			l.Attributes["step.lens.partitioned.provenance"] = Attribute{Kind: "string", Text: string(raw)}
			if kind != "source" {
				l.Attributes["step.lens.partitioned.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
			}
			s.Labels["step:body-000001"] = l
			var out bytes.Buffer
			if e = s.WritePartitionedSplineLensSTEP(&out); e == nil || out.Len() != 0 {
				t.Fatal("accepted stale "+kind, e)
			}
		})
	}
}
func TestOCCTPartitionedLensDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional STEP oracle")
	}
	run := func(args ...string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		data, e := exec.CommandContext(ctx, oracle, args...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %s", e, data)
		}
	}
	source := t.TempDir() + "/source.step"
	run(source, "write-step-partitioned-spline-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadPartitionedSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	var out bytes.Buffer
	if e = s.WritePartitionedSplineLensSTEP(&out); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/document.step"
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	massfile := t.TempDir() + "/mass.json"
	run(massfile, "step-mass-precise", path)
	raw, e = os.ReadFile(massfile)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &mass); e != nil {
		t.Fatal(e)
	}
	r := 15. / 64
	if math.Abs(mass.Volume-math.Pi*math.Pow(r, 4)/2) > 1e-10 || math.Abs(mass.Area-4*math.Pi/3*(math.Pow(1+r*r, 1.5)-1)) > 1e-9 {
		t.Fatal(mass)
	}
}

func TestPartitionedLensDocumentShortWrite(t *testing.T) {
	s, _ := partitionedLensDocument(t).Snapshot()
	if e := s.WritePartitionedSplineLensSTEP(shortWriter{}); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
