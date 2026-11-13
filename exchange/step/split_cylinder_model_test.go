package step

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSplitCylinderModel(t *testing.T) {
	b, e := os.ReadFile("testdata/split-cylinder-model.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplitCylinderModel(bytes.NewReader(b), Limits{})
	if e != nil || m.Radius != 2 || m.Height != 3 || m.Name != "split" || m.LengthUnit != .001 {
		t.Fatal(m, e)
	}
	for _, pair := range [][2]string{{"(2.,0.,0.)", "(2.,0.0000000000000001,0.)"}, {"CYLINDRICAL_SURFACE('',#4,2.)", "CYLINDRICAL_SURFACE('',#4,3.)"}, {".T.);", ".F.);"}} {
		s := strings.Replace(string(b), pair[0], pair[1], 1)
		if s == string(b) {
			t.Fatal("mutation missing", pair)
		}
		if got, e := ReadSplitCylinderModel(strings.NewReader(s), Limits{}); e == nil || got.Body != 0 {
			t.Fatal("accepted mutation", pair, e)
		}
	}
	original, e := os.ReadFile("testdata/split-cylinder-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadSplitCylinderModel(bytes.NewReader(original), Limits{}); e == nil {
		t.Fatal("accepted approximate source")
	}
}

func TestSplitCylinderModelOwnership(t *testing.T) {
	b, e := os.ReadFile("testdata/split-cylinder-model.step")
	if e != nil {
		t.Fatal(e)
	}
	raw := string(b)
	// Source-exact duplicated or disconnected topology must never be interpreted
	// as a second copy of the missing angular half.
	for _, pair := range [][2]string{
		{"(0.,0.,1.)", "(0.,0.,-1.)"},
		{"ADVANCED_FACE('',(#46),#8,.F.)", "ADVANCED_FACE('',(#46),#8,.T.)"},
		{"(#35,#42,#47,#52)", "(#35,#35,#47,#52)"},
		{"(#36,#37,#38,#39)", "(#29,#30,#31,#32)"},
		{"EDGE_CURVE('',#18,#19,#11,.T.)", "EDGE_CURVE('',#18,#19,#11,.F.)"},
		{"ENDSEC;END-ISO-10303-21;", "#200=CARTESIAN_POINT('',(0.,0.,0.));ENDSEC;END-ISO-10303-21;"},
	} {
		x := strings.Replace(raw, pair[0], pair[1], 1)
		if x == raw {
			t.Fatal("missing mutation", pair)
		}
		if m, e := ReadSplitCylinderModel(strings.NewReader(x), Limits{}); e == nil || m.Body != 0 {
			t.Fatal("accepted", pair)
		}
	}
	for _, l := range []Limits{{MaxFaces: 3}, {MaxVertices: 3}, {MaxEdgeUses: 11}, {MaxEntities: 10}, {MaxBytes: 32}} {
		if _, e := ReadSplitCylinderModel(bytes.NewReader(b), l); e == nil {
			t.Fatal("limit accepted", l)
		}
	}
}

func FuzzSplitCylinderModel(f *testing.F) {
	b, e := os.ReadFile("testdata/split-cylinder-model.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			return
		}
		m, e := ReadSplitCylinderModel(bytes.NewReader(b), Limits{MaxBytes: 1 << 16, MaxEntities: 256, MaxValues: 4096, MaxDepth: 16})
		if e == nil {
			if m.Radius <= 0 || m.Height <= 0 || m.Body == 0 {
				t.Fatal("invalid accepted model")
			}
			for _, id := range m.Faces {
				if id == 0 {
					t.Fatal("missing face")
				}
			}
		}
	})
}
