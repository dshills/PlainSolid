package step

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestQuarterCylinderModel(t *testing.T) {
	b, e := os.ReadFile("testdata/quarter-cylinder-model.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadQuarterCylinderModel(bytes.NewReader(b), Limits{})
	if e != nil || m.Radius != 2 || m.Height != 3 {
		t.Fatal(m, e)
	}
	for _, id := range m.Faces {
		if id == 0 {
			t.Fatal("missing face")
		}
	}
	for _, id := range m.Edges {
		if id == 0 {
			t.Fatal("missing edge")
		}
	}
	if _, e = ReadSplitCylinderModel(bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("half reader broadened")
	}
	half, e := os.ReadFile("testdata/split-cylinder-model.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadQuarterCylinderModel(bytes.NewReader(half), Limits{}); e == nil {
		t.Fatal("quarter reader accepted half")
	}
	d := splitWriterFixture(t)
	var out bytes.Buffer
	if e = WriteQuarterCylinderModel(&out, d); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadQuarterCylinderModel(&out, Limits{})
	if e != nil || n.Frame != d.Frame || n.Name != d.Name {
		t.Fatal(n, e)
	}
	if e = WriteQuarterCylinderModel(&out, d); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("unstable", e)
	}
	again, e := ReadQuarterCylinderModel(&out, Limits{})
	if e != nil || !reflect.DeepEqual(n.Edges, again.Edges) {
		t.Fatal("roles", e)
	}
	for _, p := range [][2]string{{"(0.,2.,0.)", "(0.00001,2.,0.)"}, {".T.);", ".F.);"}} {
		s := strings.Replace(string(b), p[0], p[1], 1)
		if s == string(b) {
			t.Fatal("missing mutation")
		}
		if m, e := ReadQuarterCylinderModel(strings.NewReader(s), Limits{}); e == nil || m.Body != 0 {
			t.Fatal("accepted mutation")
		}
	}
	for _, l := range []Limits{{MaxFaces: 5}, {MaxVertices: 7}, {MaxEdgeUses: 23}} {
		if _, e = ReadQuarterCylinderModel(bytes.NewReader(b), l); e == nil {
			t.Fatal("limit")
		}
	}
}

func TestQuarterCylinderModelLongArcReject(t *testing.T) {
	b, e := os.ReadFile("testdata/quarter-cylinder-model.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadQuarterCylinderModel(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	// Replace the first increasing quarter's end (+Y) with -Y. This is a
	// three-quarter arc, not an interchangeable shortest-arc interpretation.
	prior := f.Entities[m.Edges[3]].Records[0].Args[1]
	ent := f.Entities[m.Edges[0]]
	ent.Records[0].Args[2] = prior
	f.Entities[m.Edges[0]] = ent
	if got, e := ReadQuarterCylinderModel(bytes.NewReader(splitProductFile(f, nil, nil)), Limits{}); e == nil || got.Body != 0 {
		t.Fatal("accepted complementary long arc")
	}
	f, e = Parse(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	sh := f.Entities[m.Shell]
	sh.Records[0].Args[1].Items[1] = sh.Records[0].Args[1].Items[0]
	f.Entities[m.Shell] = sh
	if _, e = ReadQuarterCylinderModel(bytes.NewReader(splitProductFile(f, nil, nil)), Limits{}); e == nil {
		t.Fatal("accepted repeated angular cell")
	}
}
