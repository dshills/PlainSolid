package step

import (
	"bytes"
	"math"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestSplineLensSourceFailures(t *testing.T) {
	raw, e := os.ReadFile("testdata/spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"lower-bound", "inward", "model-pole", "owner", "extra-shell", "vertex"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range f.Order {
				if kind == "inward" {
					r, e := simple(f, id, "ADVANCED_FACE", 4)
					if e != nil {
						continue
					}
					v, _ := boolean(r.Args[3])
					if v {
						r.Args[3].Text = "F"
					} else {
						r.Args[3].Text = "T"
					}
					ovalSet(f, id, r)
					bounds, _ := listRefs(r.Args[1], 1)
					br, _ := simple(f, bounds[0], "FACE_BOUND", 3)
					br.Args[2] = r.Args[3]
					ovalSet(f, bounds[0], br)
					changed = true
					continue
				}
				switch kind {
				case "lower-bound":
					r, e := simple(f, id, "FACE_BOUND", 3)
					if e == nil && r.Args[2].Text == "F" {
						r.Args[2].Text = "T"
						ovalSet(f, id, r)
						changed = true
					}
				case "model-pole":
					r, e := simple(f, id, "SURFACE_CURVE", 4)
					if e == nil {
						mid, _ := ref(r.Args[1])
						base, _ := component(f.Entities[mid], "B_SPLINE_CURVE")
						ps, _ := listRefs(base.Args[1], 2)
						p, _ := simple(f, ps[1], "CARTESIAN_POINT", 2)
						p.Args[1].Items[2] = Value{Kind: Number, Text: real(math.Nextafter(2, 3))}
						ovalSet(f, ps[1], p)
						changed = true
					}
				case "owner":
					r, e := simple(f, id, "SURFACE_CURVE", 4)
					if e == nil {
						pcs, _ := listRefs(r.Args[2], 2)
						a, _ := simple(f, pcs[0], "PCURVE", 3)
						b, _ := simple(f, pcs[1], "PCURVE", 3)
						b.Args[1] = a.Args[1]
						ovalSet(f, pcs[1], b)
						changed = true
					}
				case "extra-shell":
					r, e := simple(f, id, "CLOSED_SHELL", 2)
					if e == nil {
						next := f.Order[len(f.Order)-1] + 1
						ovalSet(f, next, r)
						f.Order = append(f.Order, next)
						changed = true
					}
				case "vertex":
					r, e := simple(f, id, "EDGE_CURVE", 5)
					if e == nil {
						r.Args[2] = r.Args[1]
						ovalSet(f, id, r)
						changed = true
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Fatal("mutation missed")
			}
			if _, e = ReadSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
				t.Fatal("invalid lens accepted")
			}
		})
	}
}
func TestSplineLensIndependentFixture(t *testing.T) {
	raw, e := os.ReadFile("testdata/spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var written bytes.Buffer
	if e = WriteSplineLensBody(&written, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	copy, e := ReadSplineLensBody(&written, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(copy.Upper.Data(), m.Upper.Data()) || !reflect.DeepEqual(copy.Lower.Data(), m.Lower.Data()) {
		t.Fatal("source supports changed")
	}
	if m.SourceBodyID == 0 || m.SourceFaceIDs[0] == m.SourceFaceIDs[1] {
		t.Fatal(m)
	}
	seen := map[int64]bool{}
	for _, id := range m.SourceEdges {
		if id == 0 || seen[id] {
			t.Fatal(m.SourceEdges)
		}
		seen[id] = true
	}
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	for _, id := range f.Order {
		r, e := simple(f, id, "CLOSED_SHELL", 2)
		if e == nil {
			r.Args[1].Items[0], r.Args[1].Items[1] = r.Args[1].Items[1], r.Args[1].Items[0]
			ovalSet(f, id, r)
		}
	}
	if _, e = ReadSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 5}, {MaxEdgeUses: 7}, {MaxDepth: 3}} {
		if _, e = ReadSplineLensBody(bytes.NewReader(raw), l, CurvedOvalOptions{}); e == nil {
			t.Fatal(l)
		}
	}
}
func TestSplineLensInvalidMaterial(t *testing.T) {
	for _, kind := range []string{"reverse", "wrong-seam", "outside"} {
		m := splineLensFixture(t)
		switch kind {
		case "reverse":
			m.Upper, m.Lower = m.Lower, m.Upper
		case "wrong-seam":
			m.Radii = geom.Vec2{X: .125, Y: .125}
		case "outside":
			m.Center.X = .95
		}
		var b bytes.Buffer
		if e := WriteSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
			t.Fatal(kind, e)
		}
	}
}
func FuzzSplineLens(f *testing.F) {
	raw, e := os.ReadFile("testdata/spline-lens.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	sheared, e := os.ReadFile("testdata/sheared-spline-lens.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(sheared)
	refined, e := os.ReadFile("testdata/refined-spline-lens.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(refined)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadSplineLensBody(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 512, MaxValues: 10000, MaxFaces: 2, MaxVertices: 256, MaxEdgeUses: 8, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}

func TestSplineLensNativeRecoveryExact(t *testing.T) {
	raw, e := os.ReadFile("testdata/spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = SplineLensFromSolid(m.Solid, m.Center, m.Radii, m.Name, m.LengthUnit, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	// Sampled native tolerance alone permits this single-ulp mismatch. Exact
	// canonical recovery must independently reject it before STEP reconstruction.
	points := m.Solid.Vertices()
	points[0].X = math.Nextafter(points[0].X, math.Inf(1))
	changed, e := trimmed.NewSolid(points, m.Solid.Edges(), m.Solid.Faces(), 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = SplineLensFromSolid(changed, m.Center, m.Radii, m.Name, m.LengthUnit, CurvedOvalOptions{}); e == nil {
		t.Fatal("rounded native vertex accepted")
	}
}
