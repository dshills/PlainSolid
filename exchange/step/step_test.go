package step

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func fixture(t testing.TB) string {
	t.Helper()
	b, e := os.ReadFile("testdata/tetra.step")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestIndependentFixture(t *testing.T) {
	t.Parallel()
	m, e := Read(strings.NewReader(fixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.LengthUnit != .001 || len(m.Bodies) != 1 || m.Bodies[0].Name != "tetra's" {
		t.Fatal(m)
	}
	s := m.Bodies[0].Solid
	if s.Topology() != (brep.Topology{Vertices: 4, Edges: 6, Faces: 4, Shells: 1, EulerCharacteristic: 2}) {
		t.Fatal(s.Topology())
	}
	if math.Abs(s.Tessellate().SignedVolume()-1.0/6) > 1e-14 {
		t.Fatal("volume")
	}
}
func TestExportRoundTrip(t *testing.T) {
	t.Parallel()
	s, e := brep.Box(geom.Vec3{X: -1, Y: -2, Z: -3}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	m := Model{LengthUnit: 1, Bodies: []Body{{Name: "Box 'A' \\ Δ 🚀", Solid: s}, {Name: "Second", Solid: s}}}
	var b bytes.Buffer
	if e = Write(&b, m); e != nil {
		t.Fatal(e)
	}
	first := b.String()
	b.Reset()
	if e = Write(&b, m); e != nil || b.String() != first {
		t.Fatal("nondeterministic output", e)
	}
	parsed, e := Parse(strings.NewReader(first), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	products, reps := 0, 0
	for _, ent := range parsed.Entities {
		for _, r := range ent.Records {
			if r.Type == "PRODUCT" {
				products++
			}
			if r.Type == "SHAPE_DEFINITION_REPRESENTATION" {
				reps++
			}
		}
	}
	if products != 2 || reps != 2 {
		t.Fatal("product linkage")
	}
	got, e := Read(strings.NewReader(first), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 2 || got.LengthUnit != 1 || got.Bodies[0].Name != m.Bodies[0].Name {
		t.Fatal(got)
	}
	for _, body := range got.Bodies {
		if body.Solid.Topology() != s.Topology() || body.Solid.Tessellate().SignedVolume() != s.Tessellate().SignedVolume() {
			t.Fatal("shape changed")
		}
	}
}
func TestHoledPrism(t *testing.T) {
	t.Parallel()
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 2}}
	s, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = Write(&b, Model{LengthUnit: .001, Bodies: []Body{{"through hole", s}}}); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(b.String(), "FACE_BOUND(") {
		t.Fatal("missing hole entities")
	}
	m, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	got := m.Bodies[0].Solid
	if got.Topology() != s.Topology() || math.Abs(got.Tessellate().SignedVolume()-30) > 1e-12 {
		t.Fatal(got.Topology(), got.Tessellate().SignedVolume())
	}
	holes := 0
	for _, f := range got.Faces() {
		holes += len(f.Holes)
	}
	if holes != 2 {
		t.Fatal(holes)
	}
}
func TestSyntaxAndLimits(t *testing.T) {
	t.Parallel()
	valid := fixture(t)
	for _, l := range []Limits{{MaxBytes: 10}, {MaxEntities: 1}, {MaxValues: 1}, {MaxDepth: 1}, {MaxDepth: 257}, {MaxVertices: 2}, {MaxFaces: 2}, {MaxBytes: -1}} {
		if _, e := Read(strings.NewReader(valid), l); e == nil {
			t.Fatal(l)
		}
	}
	for _, s := range []string{"", valid + "garbage", strings.Replace(valid, "#100=CARTESIAN_POINT", "#101=CARTESIAN_POINT", 1), strings.Replace(valid, "#103)", "#999)", 1), strings.Replace(valid, "0.,0.,1.", "0.,0.,1.E999", 1), strings.Replace(valid, "/* Authored", "/* Unterminated", 1)[:100], strings.Replace(valid, "END-ISO-10303-21;", "", 1), strings.Replace(valid, "FILE_SCHEMA", "NOT_SCHEMA", 1), strings.Replace(valid, "#1=", "#0=", 1)} {
		if _, e := Parse(strings.NewReader(s), Limits{}); e == nil {
			t.Fatalf("accepted malformed syntax %q", s)
		}
	}
}
func TestUnsupportedAndMalformedTopology(t *testing.T) {
	t.Parallel()
	valid := fixture(t)
	for _, s := range []string{strings.Replace(valid, "FACETED_BREP_SHAPE_REPRESENTATION", "ADVANCED_BREP_SHAPE_REPRESENTATION", 1), strings.Replace(valid, "FACETED_BREP('tetra", "MANIFOLD_SOLID_BREP('tetra", 1), strings.Replace(valid, "FACE_OUTER_BOUND('',#11", "FACE_BOUND('',#11", 1), strings.Replace(valid, "FACE_OUTER_BOUND('',#12", "FACE_BOUND('',#12", 1), strings.Replace(valid, ".MILLI.,.METRE.", ".MILLI.,.SECOND.", 1), strings.Replace(valid, "CONTEXT(3)", "CONTEXT(2)", 1), strings.Replace(valid, "#100,#102,#101", "#100,#101,#102", 1), strings.Replace(valid, "#100,#102,#101", "#100,#100,#101", 1), strings.Replace(valid, ".T.", ".U.", 1)} {
		if s == valid {
			continue
		}
		if _, e := Read(strings.NewReader(s), Limits{}); e == nil {
			t.Fatalf("accepted unsupported/invalid model %s", s)
		}
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) / 2, nil }
func TestIOErrors(t *testing.T) {
	t.Parallel()
	if _, e := Parse(nil, Limits{}); e == nil {
		t.Fatal("nil")
	}
	if _, e := Parse(failReader{}, Limits{}); !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Fatal(e)
	}
	m, e := Read(strings.NewReader(fixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
		if e := Write(w, m); e == nil {
			t.Fatal("write error")
		}
	}
	if e := Write(io.Discard, Model{}); e == nil {
		t.Fatal("empty model")
	}
	m.LengthUnit = -1
	if e := Write(io.Discard, m); e == nil {
		t.Fatal("invalid units")
	}
}
func FuzzParse(f *testing.F) {
	f.Add(fixture(f))
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(strings.NewReader(s), Limits{MaxBytes: 1 << 16, MaxEntities: 200, MaxValues: 2000, MaxDepth: 12})
	})
}
func FuzzRead(f *testing.F) {
	f.Add(fixture(f))
	f.Fuzz(func(t *testing.T, s string) {
		m, e := Read(strings.NewReader(s), Limits{MaxBytes: 1 << 16, MaxEntities: 200, MaxValues: 2000, MaxDepth: 12, MaxVertices: 100, MaxFaces: 100, MaxSolids: 5, MaxEdgeUses: 300})
		if e == nil {
			for _, b := range m.Bodies {
				if e = b.Solid.Validate(); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestExpandedEdgeUseBudget(t *testing.T) {
	t.Parallel()
	if _, e := Read(strings.NewReader(fixture(t)), Limits{MaxEdgeUses: 11}); e == nil {
		t.Fatal("edge-use limit")
	}
}
func TestComplexAndEscapedValues(t *testing.T) {
	t.Parallel()
	s := strings.Replace(fixture(t), "#2=FACETED_BREP('tetra''s'", `#2=FACETED_BREP('\X2\0394\X0\'`, 1)
	m, e := Read(strings.NewReader(s), Limits{})
	if e != nil || m.Bodies[0].Name != "Δ" {
		t.Fatal(m, e)
	}
	for _, s := range []string{strings.Replace(fixture(t), "LENGTH_UNIT() NAMED_UNIT(*)", "LENGTH_UNIT() LENGTH_UNIT() NAMED_UNIT(*)", 1), strings.Replace(fixture(t), "'tetra''s'", `'\X4\00110000\X0\'`, 1), strings.Replace(fixture(t), "'tetra''s'", `'\X2\D800\X0\'`, 1)} {
		if _, e := Parse(strings.NewReader(s), Limits{}); e == nil {
			t.Fatal("invalid complex/string accepted")
		}
	}
}
func TestRejectAssemblyEntities(t *testing.T) {
	t.Parallel()
	s := strings.Replace(fixture(t), "#100=CARTESIAN_POINT", "#200=NEXT_ASSEMBLY_USAGE_OCCURRENCE('','','',#2,#2,$);\n#100=CARTESIAN_POINT", 1)
	if _, e := Read(strings.NewReader(s), Limits{}); e == nil {
		t.Fatal("silently ignored assembly")
	}
}

func TestPart21NumericLexemes(t *testing.T) {
	t.Parallel()
	valid := fixture(t)
	for _, bad := range []string{".5", "1E2", "1.e2", "1.2E3.", "+"} {
		s := strings.Replace(valid, "0.,0.,1.", "0.,0.,"+bad, 1)
		if _, e := Parse(strings.NewReader(s), Limits{}); e == nil {
			t.Fatalf("accepted numeric token %s", bad)
		}
	}
	for _, good := range []string{"+0.5E1", "2.", "0.E25", "012"} {
		s := strings.Replace(valid, "0.,0.,1.", "0.,0.,"+good, 1)
		if _, e := Parse(strings.NewReader(s), Limits{}); e != nil {
			t.Fatalf("rejected %s: %v", good, e)
		}
	}
}

func explicitPlaneFixture(t testing.TB) string {
	s := fixture(t)
	s = strings.Replace(s, "#10=FACE('',(#11));", "#10=FACE_SURFACE('',(#11),#200,.T.);", 1)
	extra := "#200=PLANE('',#201);\n#201=AXIS2_PLACEMENT_3D('',#100,#202,#203);\n#202=DIRECTION('',(0.,0.,-1.));\n#203=DIRECTION('',(1.,0.,0.));\n"
	return strings.Replace(s, "#90=(", extra+"#90=(", 1)
}
func TestExplicitPlanarFace(t *testing.T) {
	t.Parallel()
	s := explicitPlaneFixture(t)
	m, e := Read(strings.NewReader(s), Limits{})
	if e != nil || math.Abs(m.Bodies[0].Solid.Tessellate().SignedVolume()-1.0/6) > 1e-14 {
		t.Fatal(m, e)
	}
	reversed := strings.Replace(strings.Replace(s, "#200,.T.", "#200,.F.", 1), "(0.,0.,-1.)", "(0.,0.,1.)", 1)
	if _, e = Read(strings.NewReader(reversed), Limits{}); e != nil {
		t.Fatal("reversed surface sense", e)
	}
	defaults := strings.Replace(reversed, "#100,#202,#203", "#100,$,$", 1)
	if _, e = Read(strings.NewReader(defaults), Limits{}); e != nil {
		t.Fatal("default axes", e)
	}
	for _, bad := range []string{strings.Replace(s, "#200,.T.", "#200,.F.", 1), strings.Replace(s, "#203=DIRECTION('',(1.,0.,0.))", "#203=DIRECTION('',(0.,0.,1.))", 1), strings.Replace(s, "#201=AXIS2_PLACEMENT_3D('',#100", "#204=CARTESIAN_POINT('',(0.,0.,1.));\n#201=AXIS2_PLACEMENT_3D('',#204", 1), strings.Replace(s, "#200=PLANE", "#200=NOT_A_PLANE", 1)} {
		if _, e = Read(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("accepted inconsistent face plane")
		}
	}
}
func TestExporterDeclaresFaceSurfaces(t *testing.T) {
	t.Parallel()
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = Write(&b, Model{LengthUnit: .001, Bodies: []Body{{"box", s}}}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	planes, faces := 0, 0
	for _, entity := range f.Entities {
		for _, r := range entity.Records {
			switch r.Type {
			case "FACE":
				t.Fatal("implicit FACE is skipped by OCCT; emit FACE_SURFACE")
			case "FACE_SURFACE":
				faces++
			case "PLANE":
				planes++
			}
		}
	}
	if planes != 6 || faces != 6 {
		t.Fatal(planes, faces)
	}
}
