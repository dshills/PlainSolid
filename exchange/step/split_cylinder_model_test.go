package step

import (
	"bytes"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestSplitCylinderModel(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestSplitCylinderModelPose(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/split-cylinder-model-posed.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplitCylinderModel(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Frame.Origin() != (geom.Vec3{X: 10, Y: -4, Z: 7}) || m.Frame.XAxis() != (geom.Vec3{Z: -1}) || m.Frame.ZAxis() != (geom.Vec3{X: -1}) || m.Radius != 2 || m.Height != 3 {
		t.Fatal(m)
	}
	unrepresentable := strings.Replace(string(b), "(10.,-4.,7.)", "(8.67361737988403547205962240695953369140625E-19,-4.,7.)", 1)
	if unrepresentable == string(b) {
		t.Fatal("missing source origin")
	}
	if _, e = ReadSplitCylinderModel(strings.NewReader(unrepresentable), Limits{}); e == nil {
		t.Fatal("rounded local subtraction")
	}
	metre := strings.Replace(string(b), "SI_UNIT(.MILLI.,.METRE.)", "SI_UNIT($,.METRE.)", 1)
	n, e := ReadSplitCylinderModel(strings.NewReader(metre), Limits{})
	if e != nil || n.LengthUnit != 1 || n.Frame != m.Frame || n.Radius != m.Radius || n.Height != m.Height {
		t.Fatal(n, e)
	}
	bad := strings.Replace(string(b), "(-1.,0.,0.)", "(-1.,0.000000000000001,0.)", 1)
	if bad == string(b) {
		t.Fatal("missing axis")
	}
	if _, e = ReadSplitCylinderModel(strings.NewReader(bad), Limits{}); e == nil {
		t.Fatal("accepted rounded frame")
	}
}
