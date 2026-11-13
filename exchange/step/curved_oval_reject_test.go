package step

import (
	"bytes"
	"errors"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestCurvedOvalSourceRejections(t *testing.T) {
	raw, e := os.ReadFile("testdata/rational-curved-oval-face.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"model-pole", "model-weight", "owner", "use-sense", "vertex", "extra-curve", "extra-shell"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range f.Order {
				en := f.Entities[id]
				if kind == "model-pole" || kind == "model-weight" || kind == "extra-curve" {
					sc, e := simple(f, id, "SURFACE_CURVE", 4)
					if e != nil {
						continue
					}
					mid, _ := ref(sc.Args[1])
					model := f.Entities[mid]
					if kind == "extra-curve" {
						nid := f.Order[len(f.Order)-1] + 1
						model.ID = nid
						f.Entities[nid] = model
						f.Order = append(f.Order, nid)
						changed = true
						break
					}
					if kind == "model-pole" {
						base, _ := component(model, "B_SPLINE_CURVE")
						ids, _ := listRefs(base.Args[1], 2)
						p, _ := simple(f, ids[1], "CARTESIAN_POINT", 2)
						v, _ := numeric(p.Args[1].Items[2])
						p.Args[1].Items[2] = Value{Kind: Number, Text: real(math.Nextafter(v, math.Inf(1)))}
						ovalSet(f, ids[1], p)
					} else {
						for j, r := range model.Records {
							if r.Type == "RATIONAL_B_SPLINE_CURVE" {
								v, _ := numeric(r.Args[0].Items[1])
								r.Args[0].Items[1] = Value{Kind: Number, Text: real(v + .125)}
								model.Records[j] = r
							}
						}
						f.Entities[mid] = model
					}
					changed = true
					break
				}
				if len(en.Records) != 1 {
					continue
				}
				r := en.Records[0]
				switch kind {
				case "owner":
					if r.Type == "PCURVE" {
						r.Args[1] = Value{Kind: Reference, Ref: id}
						ovalSet(f, id, r)
						changed = true
					}
				case "use-sense":
					if r.Type == "ORIENTED_EDGE" {
						r.Args[4].Text = "F"
						ovalSet(f, id, r)
						changed = true
					}
				case "vertex":
					if r.Type == "EDGE_CURVE" {
						r.Args[2] = r.Args[1]
						ovalSet(f, id, r)
						changed = true
					}
				case "extra-shell":
					if r.Type == "OPEN_SHELL" {
						nid := f.Order[len(f.Order)-1] + 1
						en.ID = nid
						f.Entities[nid] = en
						f.Order = append(f.Order, nid)
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
			if _, e = ReadCurvedOvalFace(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
}
func TestCurvedOvalSourceScaleAndRotation(t *testing.T) {
	raw, e := os.ReadFile("testdata/rational-curved-oval-face.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range f.Order {
		if sc, e := simple(f, id, "SURFACE_CURVE", 4); e == nil {
			mid, _ := ref(sc.Args[1])
			en := f.Entities[mid]
			for i, r := range en.Records {
				if r.Type == "RATIONAL_B_SPLINE_CURVE" {
					for j, v := range r.Args[0].Items {
						x, _ := numeric(v)
						r.Args[0].Items[j] = Value{Kind: Number, Text: real(3 * x)}
					}
				}
				en.Records[i] = r
			}
			f.Entities[mid] = en
		}
		if r, e := simple(f, id, "EDGE_LOOP", 2); e == nil {
			items := r.Args[1].Items
			r.Args[1].Items = append(append([]Value(nil), items[1:]...), items[0])
			ovalSet(f, id, r)
		}
	}
	if _, e = ReadCurvedOvalFace(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e != nil {
		t.Fatal("equivalent source changed", e)
	}
}
func TestCurvedOvalWriterRejections(t *testing.T) {
	m := curvedOvalFixture(t)
	m.Outer.QuarterWeight = .75
	var b bytes.Buffer
	if e := WriteCurvedOvalFace(&b, m, CurvedOvalOptions{}); !errors.Is(e, geom.ErrUnrepresentable) || b.Len() != 0 {
		t.Fatal("rounded model export", e)
	}
	m = curvedOvalFixture(t)
	d := m.Geometry.Data()
	for j := 0; j < 3; j++ {
		d.Poles[3+j].X = 2
	}
	m.Geometry, _ = geom.NewBSplineSurface(2, 2, 3, 3, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e := WriteCurvedOvalFace(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
		t.Fatal("folded support")
	}
	if e := WriteCurvedOvalFace(&b, curvedOvalFixture(t), CurvedOvalOptions{MaxOperations: 10}); e == nil || b.Len() != 0 {
		t.Fatal("budget")
	}
	raw, e := os.ReadFile("testdata/curved-oval-face.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCurvedOvalFace(bytes.NewReader(raw), Limits{MaxVertices: 10}, CurvedOvalOptions{}); e == nil {
		t.Fatal("point budget")
	}
}
func TestIndependentCurvedOvalFaces(t *testing.T) {
	for _, name := range []string{"curved-oval-face", "rational-curved-oval-face", "curved-oval-hole"} {
		raw, e := os.ReadFile("testdata/" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadCurvedOvalFace(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
		if e != nil {
			t.Fatal(name, e)
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		all := append([]int64(nil), m.SourceEdges[:]...)
		for _, loop := range m.SourceHoleEdges {
			all = append(all, loop[:]...)
		}
		for _, id := range all {
			if _, e = simple(f, id, "EDGE_CURVE", 5); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func FuzzCurvedOvalFace(f *testing.F) {
	raw, e := os.ReadFile("testdata/rational-curved-oval-face.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	hole, e := os.ReadFile("testdata/curved-oval-hole.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(hole)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadCurvedOvalFace(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 512, MaxFaces: 1, MaxVertices: 256, MaxEdgeUses: 68}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}
