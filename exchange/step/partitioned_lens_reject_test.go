package step

import (
	"bytes"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestPartitionedLensRejectsSourceChanges(t *testing.T) {
	raw, e := os.ReadFile("testdata/partitioned-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"model", "uv", "weight", "owner", "sense", "missing", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range f.Order {
				if kind == "sense" {
					r, e := simple(f, id, "ORIENTED_EDGE", 5)
					if e == nil {
						r.Args[4].Text = "F"
						ovalSet(f, id, r)
						changed = true
					}
				}
				if kind == "missing" || kind == "duplicate" {
					r, e := simple(f, id, "EDGE_LOOP", 2)
					if e == nil {
						if kind == "missing" {
							r.Args[1].Items = r.Args[1].Items[1:]
						} else {
							r.Args[1].Items[1] = r.Args[1].Items[0]
						}
						ovalSet(f, id, r)
						changed = true
					}
				}
				if kind == "owner" {
					r, e := simple(f, id, "SURFACE_CURVE", 4)
					if e == nil {
						r.Args[2].Items[1] = r.Args[2].Items[0]
						ovalSet(f, id, r)
						changed = true
					}
				}
				if kind == "model" || kind == "uv" || kind == "weight" {
					r, e := simple(f, id, "SURFACE_CURVE", 4)
					if e == nil {
						cid := r.Args[1].Ref
						if kind != "model" {
							pc, _ := simple(f, r.Args[2].Items[0].Ref, "PCURVE", 3)
							rep, _ := simple(f, pc.Args[2].Ref, "DEFINITIONAL_REPRESENTATION", 3)
							cid = rep.Args[1].Items[0].Ref
						}
						if kind == "weight" {
							en := f.Entities[cid]
							for i := range en.Records {
								if en.Records[i].Type == "RATIONAL_B_SPLINE_CURVE" {
									v, _ := numeric(en.Records[i].Args[0].Items[1])
									en.Records[i].Args[0].Items[1] = Value{Kind: Number, Text: real(math.Nextafter(v, math.Inf(1)))}
								}
							}
							f.Entities[cid] = en
						} else {
							base, _ := component(f.Entities[cid], "B_SPLINE_CURVE")
							pid := base.Args[1].Items[1].Ref
							p, _ := simple(f, pid, "CARTESIAN_POINT", 2)
							v, _ := numeric(p.Args[1].Items[0])
							p.Args[1].Items[0] = Value{Kind: Number, Text: real(math.Nextafter(v, math.Inf(1)))}
							ovalSet(f, pid, p)
						}
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
			if _, e = ReadPartitionedSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
				t.Fatal("accepted changed " + kind)
			}
		})
	}
	for _, l := range []Limits{{MaxEdgeUses: 11}, {MaxVertices: 5}, {MaxFaces: 1}} {
		if _, e = ReadPartitionedSplineLensBody(bytes.NewReader(raw), l, CurvedOvalOptions{}); e == nil {
			t.Fatal("ignored limits")
		}
	}
	var b bytes.Buffer
	if e = WritePartitionedSplineLensBody(&b, partitionedLensFixture(t), CurvedOvalOptions{MaxOperations: 64}); e == nil || b.Len() != 0 {
		t.Fatal("budget/no partial output", e)
	}
}

func FuzzPartitionedSplineLens(f *testing.F) {
	raw, e := os.ReadFile("testdata/partitioned-spline-lens.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadPartitionedSplineLensBody(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 512, MaxValues: 12000, MaxFaces: 2, MaxVertices: 256, MaxEdgeUses: 32, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}

func TestPartitionedLensRejectsRoundedSupportAndIrrationalCuts(t *testing.T) {
	raw, e := os.ReadFile("testdata/partitioned-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	changed := false
	for _, id := range f.Order {
		r, e := simple(f, id, "B_SPLINE_SURFACE_WITH_KNOTS", 13)
		if e != nil {
			continue
		}
		pid := r.Args[3].Items[0].Items[0].Ref
		p, _ := simple(f, pid, "CARTESIAN_POINT", 2)
		z, _ := numeric(p.Args[1].Items[2])
		p.Args[1].Items[2] = Value{Kind: Number, Text: real(math.Round(z*1e12) / 1e12)}
		ovalSet(f, pid, p)
		changed = true
		break
	}
	if !changed {
		t.Fatal("support mutation missed")
	}
	if _, e = ReadPartitionedSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("rounded support accepted")
	}
	m := splineLensFixture(t)
	p, e := geom.NewSurfacePatch(m.Lower, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.625, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Lower, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WritePartitionedSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
		t.Fatal("irrational cut rounded", e)
	}
}
