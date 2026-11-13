package step

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func orthogonalFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/orthogonal-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestOrthogonalSplineFace(t *testing.T) {
	raw := orthogonalFixture(t)
	m, e := ReadOrthogonalSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil || len(m.Faces) != 1 || len(m.Faces[0].Boundary) != 6 {
		t.Fatal(m, e)
	}
	var b bytes.Buffer
	if e = WriteOrthogonalSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	round, e := ReadOrthogonalSplineFaces(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(round.Faces[0].Boundary, m.Faces[0].Boundary) || !reflect.DeepEqual(round.Faces[0].Geometry.Data(), m.Faces[0].Geometry.Data()) {
		t.Fatal(round, e)
	}
	if _, e = ReadBoundedSplineFaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("rectangle reader widened")
	}
	for _, l := range []Limits{{MaxEdgeUses: 5}, {MaxVertices: 3}, {MaxBytes: 100}, {MaxEntities: 10}} {
		if _, e = ReadOrthogonalSplineFaces(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	for _, pair := range [][2]string{{"#23 = CARTESIAN_POINT('',(20.,0.,0.))", "#23 = CARTESIAN_POINT('',(20.000000000000004,0.,0.))"}, {"#28 = CARTESIAN_POINT('',(20.,0.,0.))", "#28 = CARTESIAN_POINT('',(20.000000000000004,0.,0.))"}, {"#83 = CARTESIAN_POINT('',(0.75,0.))", "#83 = CARTESIAN_POINT('',(0.7500000000000001,0.))"}} {
		bad := strings.Replace(string(raw), pair[0], pair[1], 1)
		if bad == string(raw) {
			t.Fatal("missed source mutation")
		}
		if _, e := ReadOrthogonalSplineFaces(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
	bad := strings.Replace(string(raw), "#21 = EDGE_CURVE('',#22,#24,#26,.T.)", "#21 = EDGE_CURVE('',#22,#24,#26,.F.)", 1)
	if bad == string(raw) {
		t.Fatal("missed edge sense")
	}
	if _, e = ReadOrthogonalSplineFaces(strings.NewReader(bad), Limits{}); e == nil {
		t.Fatal("wrong model sense")
	}
}
func TestOrthogonalSplinePolygonReject(t *testing.T) {
	raw := orthogonalFixture(t)
	m, e := ReadOrthogonalSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"clockwise", "diagonal", "touch", "duplicate", "limit", "unrepresentable", "infinite"} {
		t.Run(kind, func(t *testing.T) {
			bad := m
			bad.Faces = append([]OrthogonalSplineFace(nil), m.Faces...)
			p := append([]geom.Vec2(nil), m.Faces[0].Boundary...)
			switch kind {
			case "clockwise":
				for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
					p[i], p[j] = p[j], p[i]
				}
			case "infinite":
				p[1].X = math.Inf(1)
				p[2].X = math.Inf(1)
			case "diagonal":
				p[1].Y = .125
			case "duplicate":
				p[1] = p[0]
			case "touch":
				p = []geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {X: .5, Y: 1}, {X: .5}, {Y: 0}, {Y: 1}, {X: 0}}
			case "limit":
				p = make([]geom.Vec2, MaxOrthogonalSplineVertices+2)
			case "unrepresentable":
				for i := range p {
					if p[i].X == .3125 {
						p[i].X = .25
					}
				}
			}
			bad.Faces[0].Boundary = p
			var b bytes.Buffer
			if e := WriteOrthogonalSplineFaces(&b, bad); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
}
func TestOCCTOrthogonalSplineFace(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-orthogonal-spline-face")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteOrthogonalSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	reference, got := stepHoledArea(t, source), stepHoledArea(t, path)
	if math.Abs(reference-got) > 1e-8 {
		t.Fatal(reference, got)
	}
}
func FuzzOrthogonalSplineFace(f *testing.F) {
	f.Add(orthogonalFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadOrthogonalSplineFaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 256, MaxFaces: 8, MaxSolids: 8, MaxEdgeUses: 128})
	})
}

func TestExactSplineWeightNormalization(t *testing.T) {
	for _, tc := range []struct {
		values   string
		accepted bool
	}{{"1.,2.", true}, {"1.3125,2.", true}, {"1.,3.", false}, {"0.,1.", false}} {
		f, e := Parse(strings.NewReader(preflightFile("#1=RATIONAL_B_SPLINE_CURVE(("+tc.values+"));")), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		e = exactSplineWeightNormalization(f, 1, "RATIONAL_B_SPLINE_CURVE")
		if (e == nil) != tc.accepted {
			t.Fatal(tc, e)
		}
	}
}
