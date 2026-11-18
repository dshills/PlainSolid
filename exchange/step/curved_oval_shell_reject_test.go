package step

import (
	"bytes"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestCurvedOvalShellSourceRejections(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/curved-oval-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"missing-owner", "duplicate-owner", "cycle", "same-use", "extra-edge", "wrong-vertex"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var sid int64
			var sc Record
			for _, id := range f.Order {
				r, e := simple(f, id, "SURFACE_CURVE", 4)
				if e == nil {
					ids, e := listRefs(r.Args[2], 1)
					if e == nil && len(ids) == 2 {
						sid, sc = id, r
						break
					}
				}
			}
			if sid == 0 {
				t.Fatal("fixture no seam")
			}
			pcs, _ := listRefs(sc.Args[2], 2)
			switch kind {
			case "missing-owner":
				sc.Args[2].Items = sc.Args[2].Items[:1]
				ovalSet(f, sid, sc)
			case "duplicate-owner":
				first, _ := simple(f, pcs[0], "PCURVE", 3)
				second, _ := simple(f, pcs[1], "PCURVE", 3)
				second.Args[1] = first.Args[1]
				ovalSet(f, pcs[1], second)
			case "cycle":
				pc, _ := simple(f, pcs[0], "PCURVE", 3)
				pc.Args[2] = Value{Kind: Reference, Ref: pcs[0]}
				ovalSet(f, pcs[0], pc)
			default:
				var eid int64
				var edge Record
				for _, id := range f.Order {
					r, e := simple(f, id, "EDGE_CURVE", 5)
					if e == nil && r.Args[3].Ref == sid {
						eid, edge = id, r
						break
					}
				}
				if eid == 0 {
					t.Fatal("fixture edge")
				}
				switch kind {
				case "extra-edge":
					id := f.Order[len(f.Order)-1] + 1
					ovalSet(f, id, edge)
					f.Order = append(f.Order, id)
				case "wrong-vertex":
					edge.Args[2] = edge.Args[1]
					ovalSet(f, eid, edge)
				case "same-use":
					for _, id := range f.Order {
						u, e := simple(f, id, "ORIENTED_EDGE", 5)
						if e == nil && u.Args[3].Ref == eid {
							u.Args[4].Text = "T"
							ovalSet(f, id, u)
						}
					}
				}
			}
			if _, e = ReadCurvedOvalOpenShell(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
				t.Fatal("accepted invalid shell")
			}
		})
	}
}
func TestCurvedOvalShellFixtureAndLimits(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/curved-oval-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCurvedOvalOpenShell(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Annulus.Holes) != 1 || m.Disk.Geometry.Data().Poles[0] == m.Annulus.Geometry.Data().Poles[0] {
		t.Fatal("independent charts")
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 10}, {MaxEdgeUses: 11}, {MaxDepth: 3}} {
		if _, e = ReadCurvedOvalOpenShell(bytes.NewReader(raw), l, CurvedOvalOptions{}); e == nil {
			t.Fatal("ignored limits", l)
		}
	}
	if _, e = ReadCurvedOvalOpenShell(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{MaxOperations: 10}); e == nil {
		t.Fatal("reset arithmetic budget")
	}
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	for _, id := range f.Order {
		r, e := simple(f, id, "OPEN_SHELL", 2)
		if e == nil {
			r.Args[1].Items[0], r.Args[1].Items[1] = r.Args[1].Items[1], r.Args[1].Items[0]
			ovalSet(f, id, r)
		}
	}
	if _, e = ReadCurvedOvalOpenShell(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e != nil {
		t.Fatal("source face ordering", e)
	}
}
func TestCurvedOvalShellWriterRejectsSeamMismatch(t *testing.T) {
	t.Parallel()
	m := curvedOvalShellFixture(t)
	m.Disk.Outer.Radii = geom.Vec2{X: .125, Y: .125}
	var b bytes.Buffer
	if e := WriteCurvedOvalOpenShell(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
		t.Fatal("mismatched seam emitted", e)
	}
}
func FuzzCurvedOvalOpenShell(f *testing.F) {
	raw, e := os.ReadFile("testdata/curved-oval-shell.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCurvedOvalOpenShell(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 512, MaxValues: 10000, MaxFaces: 2, MaxVertices: 256, MaxEdgeUses: 12, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}
