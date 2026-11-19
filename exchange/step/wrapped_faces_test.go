package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestWrappedFundamentalFace(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-surface.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	face, e := DecodeWrappedSplineFace(context.Background(), f, 17, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	if face.SideEdges != [4]int64{64, 21, 85, 21} || face.CornerVertices != [4]int64{22, 22, 24, 24} {
		t.Fatalf("unexpected explicit topology: %+v", face)
	}
	for _, change := range [][2]string{
		{"VECTOR('',#54,1.)", "VECTOR('',#54,1.00000000000001)"},
		{"#83 = ORIENTED_EDGE('',*,*,#21,.T.)", "#83 = ORIENTED_EDGE('',*,*,#21,.F.)"},
		{"(#20,#63,#83,#84)", "(#20,#83,#63,#84)"},
		{"(#30,#56)", "(#30,#30)"},
		{"#64 = EDGE_CURVE('',#22,#22,#65,.T.)", "#64 = EDGE_CURVE('',#22,#24,#65,.T.)"},
	} {
		mutated := strings.Replace(string(raw), change[0], change[1], 1)
		if mutated == string(raw) {
			t.Fatalf("stale mutation %s", change[0])
		}
		parsed, e := Parse(strings.NewReader(mutated), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = DecodeWrappedSplineFace(context.Background(), parsed, 17, WrappedSurfaceOptions{}); e == nil {
			t.Fatalf("accepted %s", change[1])
		}
	}
}

func TestWrappedFundamentalFaceOCCT(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/wrapped.step"
	stepOracle(t, path, "write-step-wrapped-surface")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for id, ent := range f.Entities {
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_FACE" {
			face, e := DecodeWrappedSplineFace(context.Background(), f, id, WrappedSurfaceOptions{ContinuityOrder: 1})
			if e != nil {
				t.Fatal(e)
			}
			if face.SideEdges[1] != face.SideEdges[3] {
				t.Fatal("missing seam")
			}
			count++
		}
	}
	if count != 1 {
		t.Fatalf("faces %d", count)
	}
}

func TestWriteWrappedFundamentalFace(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteWrappedSplineFaces(&b, WrappedSurfaceModel{Surfaces: []WrappedSplineSurface{{Name: "cylinder", Geometry: wrappedCylinder(t)}}, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	faces := 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_FACE" {
			v, e := DecodeWrappedSplineFace(context.Background(), f, id, WrappedSurfaceOptions{ContinuityOrder: 1})
			if e != nil {
				t.Fatal(e)
			}
			if v.Name != "cylinder" || v.SideEdges[1] != v.SideEdges[3] || v.CornerVertices[0] != v.CornerVertices[1] {
				t.Fatal("roundtrip topology")
			}
			faces++
		}
	}
	if faces != 1 {
		t.Fatal(faces)
	}
	path := t.TempDir() + "/wrapped.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-mass-precise", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct{ Area float64 }
	if e = json.Unmarshal(raw, &mass); e != nil || math.Abs(mass.Area-20*math.Pi) > 1e-6 {
		t.Fatal(mass, e)
	}
	stepHoledArea(t, path)
}

func wrappedToroidForSTEP(t *testing.T) *geom.WrappedSurfacePatch {
	t.Helper()
	circle := []geom.Vec2{{X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: 0}, {X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 0}}
	var poles []geom.Vec3
	var weights []float64
	for i, u := range circle {
		wu := 1.
		if i%2 == 1 {
			wu = math.Round(math.Sqrt(.5)*(1<<24)) / (1 << 24)
		}
		for j, v := range circle {
			wv := 1.
			if j%2 == 1 {
				wv = math.Round(math.Sqrt(.5)*(1<<24)) / (1 << 24)
			}
			poles = append(poles, geom.Vec3{X: (3 + v.X) * u.X, Y: (3 + v.X) * u.Y, Z: v.Y})
			weights = append(weights, wu*wv)
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	s, e := geom.NewBSplineSurface(2, 2, 9, 9, poles, knots, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, WrapV: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func TestWriteDoublyWrappedFundamentalFace(t *testing.T) {
	t.Parallel()
	w := wrappedToroidForSTEP(t)
	var e error
	var b bytes.Buffer
	if e = WriteWrappedSplineFaces(&b, WrappedSurfaceModel{Surfaces: []WrappedSplineSurface{{Geometry: w}}, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_FACE" {
			face, e := DecodeWrappedSplineFace(context.Background(), f, id, WrappedSurfaceOptions{ContinuityOrder: 1})
			if e != nil {
				t.Fatal(e)
			}
			for _, v := range face.CornerVertices {
				if v != face.CornerVertices[0] {
					t.Fatal("not one shared vertex")
				}
			}
			if face.SideEdges[0] != face.SideEdges[2] || face.SideEdges[1] != face.SideEdges[3] {
				t.Fatal("missing double seams")
			}
		}
	}
	path := t.TempDir() + "/torus.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	stepHoledArea(t, path)
}
