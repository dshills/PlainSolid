package step

import (
	"bytes"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestCurvedOvalHoles(t *testing.T) {
	t.Parallel()
	for _, multi := range []bool{false, true} {
		m := curvedOvalFixture(t)
		h := m.Outer
		h.Radii = geom.Vec2{X: 3. / 32, Y: 3. / 32}
		m.Holes = []QuadraticOval{h}
		if multi {
			h.Radii = geom.Vec2{X: 3. / 128, Y: 3. / 128}
			h.Center.X = .5 - 3./64
			m.Holes = []QuadraticOval{h}
			h.Center.X = .5 + 3./64
			m.Holes = append(m.Holes, h)
		}
		var b bytes.Buffer
		if e := WriteCurvedOvalFace(&b, m, CurvedOvalOptions{}); e != nil {
			t.Fatal(multi, e)
		}
		got, e := ReadCurvedOvalFace(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(got.Holes) != len(m.Holes) || len(got.SourceHoleEdges) != len(m.Holes) {
			t.Fatal(got)
		}
		for i, h := range m.Holes {
			if got.Holes[i] != h {
				t.Fatal(got.Holes, h)
			}
		}
		if _, e = ReadCurvedOvalFace(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{MaxOperations: 10}); e == nil {
			t.Fatal("budget reset")
		}
		if _, e = ReadCurvedOvalFace(bytes.NewReader(b.Bytes()), Limits{MaxEdgeUses: 4}, CurvedOvalOptions{}); e == nil {
			t.Fatal("ignored edge budget")
		}
	}
}
func TestCurvedOvalInvalidHoles(t *testing.T) {
	t.Parallel()
	base := curvedOvalFixture(t)
	h := base.Outer
	h.Radii = geom.Vec2{X: 3. / 32, Y: 3. / 32}
	small := h
	small.Radii = geom.Vec2{X: 3. / 64, Y: 3. / 64}
	for _, holes := range [][]QuadraticOval{{base.Outer}, {h, h}, {h, small}, {{Center: geom.Vec2{X: .9, Y: .5}, Radii: h.Radii, QuarterWeight: .5}}} {
		m := base
		m.Holes = holes
		var b bytes.Buffer
		if e := WriteCurvedOvalFace(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
			t.Fatal("invalid holes emitted", e)
		}
	}
}
func TestCurvedOvalHoleOCCT(t *testing.T) {
	t.Parallel()
	checkCurvedOvalOCCT(t, "write-step-curved-oval-hole")
}

func TestCurvedOvalHoleSourceRejections(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/curved-oval-hole.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"outer-role", "bound-sense", "duplicate-bound", "missing-bound", "hole-model", "hole-use"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var fid int64
			for _, id := range f.Order {
				if _, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
					fid = id
				}
			}
			face, _ := simple(f, fid, "ADVANCED_FACE", 4)
			bounds, _ := listRefs(face.Args[1], 2)
			bid := bounds[1]
			bound, e := simple(f, bid, "FACE_BOUND", 3)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "outer-role":
				bound.Type = "FACE_OUTER_BOUND"
				ovalSet(f, bid, bound)
			case "bound-sense":
				bound.Args[2].Text = "F"
				ovalSet(f, bid, bound)
			case "duplicate-bound":
				face.Args[1].Items[1] = face.Args[1].Items[0]
				ovalSet(f, fid, face)
			case "missing-bound":
				face.Args[1].Items = face.Args[1].Items[:1]
				ovalSet(f, fid, face)
			default:
				lid, _ := ref(bound.Args[1])
				loop, _ := simple(f, lid, "EDGE_LOOP", 2)
				uses, _ := listRefs(loop.Args[1], 4)
				use, _ := simple(f, uses[0], "ORIENTED_EDGE", 5)
				if kind == "hole-use" {
					use.Args[4].Text = "F"
					ovalSet(f, uses[0], use)
				} else {
					eid, _ := ref(use.Args[3])
					edge, _ := simple(f, eid, "EDGE_CURVE", 5)
					sid, _ := ref(edge.Args[3])
					sc, _ := simple(f, sid, "SURFACE_CURVE", 4)
					mid, _ := ref(sc.Args[1])
					base, _ := component(f.Entities[mid], "B_SPLINE_CURVE")
					ps, _ := listRefs(base.Args[1], 2)
					p, _ := simple(f, ps[1], "CARTESIAN_POINT", 2)
					v, _ := numeric(p.Args[1].Items[2])
					p.Args[1].Items[2] = Value{Kind: Number, Text: real(math.Nextafter(v, math.Inf(1)))}
					ovalSet(f, ps[1], p)
				}
			}
			if _, e = ReadCurvedOvalFace(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
				t.Fatal("accepted malformed hole")
			}
		})
	}
}
