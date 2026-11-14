package step

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
	"regexp"
	"strconv"
	"testing"
)

func TestTorusCylinderPlacedPairedImport(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []int{1, 2} {
		for _, c := range []float64{4, 6} {
			for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
				b := torusCylinderSTEPBody(op)
				b.CylinderRadius = c
				b.Lower, b.Upper = -4, 3
				pose := torusCylinderExportPose(t, mode)
				s, e := NewTorusCylinderSTEPOccurrence(ctx, b, pose, .001, 1e-8)
				if e != nil {
					t.Fatal(e)
				}
				var raw bytes.Buffer
				if e = s.WriteSTEP(&raw); e != nil {
					t.Fatal(e)
				}
				recovered, e := ReadTorusCylinderOccurrence(ctx, bytes.NewReader(raw.Bytes()), Limits{}, 1e-8)
				if e != nil {
					t.Fatal(mode, c, op, e)
				}
				want, _ := pose.Matrix()
				got, e := recovered.Pose()
				if e != nil {
					t.Fatal(e)
				}
				for i := range want {
					for j := range want[i] {
						if got[i][j].Cmp(want[i][j]) != 0 {
							t.Fatal("exact recovered pose", mode)
						}
					}
				}
				got[0][3].SetInt64(99)
				again, _ := recovered.Pose()
				if again[0][3].Cmp(want[0][3]) != 0 {
					t.Fatal("pose alias")
				}
				source, retained := recovered.Original().Source()
				if retained != (op == geom.TorusCylinderUnion) || source.CylinderRadius != c || source.Operation != op {
					t.Fatal("source")
				}
				if retained && (source.Lower != -4 || source.Upper != 3) {
					t.Fatal("retained caps")
				}
				_, h2, roots, e := recovered.Original().RimProof()
				if e != nil || h2.Cmp(big.NewRat(3, 1)) != 0 {
					t.Fatal("rim", e)
				}
				for _, root := range roots {
					p, e := root.SignPolynomial(ctx, []*big.Rat{big.NewRat(-3, 1), new(big.Rat), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
					if e != nil || p.Sign != 0 {
						t.Fatal("exact symbol", e)
					}
				}
				local := [3]*big.Rat{big.NewRat(7, 1), new(big.Rat), new(big.Rat)}
				if op == geom.TorusCylinderCommon {
					local[0] = big.NewRat(3, 1)
				}
				world, e := pose.TransformPoint(ctx, local, false, 4096)
				if e != nil {
					t.Fatal(e)
				}
				location, owners, e := recovered.Query().ClassifyRational(ctx, world, geom.SurfacePatchOptions{})
				if e != nil || location != geom.BoreBoundary || owners != geom.TorusCylinderTorus {
					t.Fatal("world owner", location, owners, e)
				}
				var output bytes.Buffer
				if e = recovered.WriteSTEP(&output); e != nil || !bytes.Equal(raw.Bytes(), output.Bytes()) {
					t.Fatal("deterministic world reexport", e)
				}
			}
		}
	}
}
func TestTorusCylinderPlacedImportRejects(t *testing.T) {
	ctx := context.Background()
	s, e := NewTorusCylinderSTEPOccurrence(ctx, torusCylinderSTEPBody(geom.TorusCylinderUnion), torusCylinderExportPose(t, 2), .001, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = s.WriteSTEP(&raw); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"scaled-axis", "skew-axis", "wrong-reference", "reflected-world", "shifted-world-vertex", "circle-radius", "missing-reference", "pcurve", "orientation"} {
		t.Run(kind, func(t *testing.T) {
			changed := sideDiskMutateSource(t, raw.Bytes(), func(f *File) {
				find := func(typ string) int64 {
					for _, id := range f.Order {
						for _, rec := range f.Entities[id].Records {
							if rec.Type == typ {
								return id
							}
						}
					}
					t.Fatal("missing", typ)
					return 0
				}
				edit := func(id int64, fn func(*Record)) { en := f.Entities[id]; fn(&en.Records[0]); f.Entities[id] = en }
				torus := f.Entities[find("TOROIDAL_SURFACE")].Records[0]
				frameID := torus.Args[1].Ref
				frame := f.Entities[frameID].Records[0]
				switch kind {
				case "scaled-axis", "skew-axis", "wrong-reference":
					id := frame.Args[2].Ref
					if kind == "wrong-reference" {
						id = frame.Args[3].Ref
					}
					edit(id, func(r *Record) {
						for j, v := range r.Args[1].Items {
							q, _ := torusCylinderImportNumber(v)
							if kind == "skew-axis" {
								if j == 0 {
									q.Add(q, big.NewRat(1, 100))
								}
							} else {
								n := int64(2)
								if kind == "wrong-reference" {
									n = -1
								}
								q.Mul(q, big.NewRat(n, 1))
							}
							txt, _ := torusCylinderDecimal(q)
							r.Args[1].Items[j] = Value{Kind: Number, Text: txt}
						}
					})
				case "reflected-world":
					for _, id := range f.Order {
						en := f.Entities[id]
						if len(en.Records) != 1 {
							continue
						}
						r := en.Records[0]
						if (r.Type == "CARTESIAN_POINT" || r.Type == "DIRECTION") && len(r.Args[1].Items) == 3 {
							edit(id, func(r *Record) {
								q, _ := torusCylinderImportNumber(r.Args[1].Items[0])
								q.Neg(q)
								txt, _ := torusCylinderDecimal(q)
								r.Args[1].Items[0] = Value{Kind: Number, Text: txt}
							})
						}
					}
				case "shifted-world-vertex":
					vertex := f.Entities[find("VERTEX_POINT")].Records[0]
					edit(vertex.Args[1].Ref, func(r *Record) {
						q, _ := torusCylinderImportNumber(r.Args[1].Items[0])
						delta, _ := new(big.Rat).SetString("0.0000000000000000000000000000000000000001")
						q.Add(q, delta)
						txt, _ := torusCylinderDecimal(q)
						r.Args[1].Items[0] = Value{Kind: Number, Text: txt}
					})
				case "circle-radius":
					edit(find("CIRCLE"), func(r *Record) { r.Args[2] = Value{Kind: Number, Text: "7."} })
				case "missing-reference":
					edit(frameID, func(r *Record) { r.Args[3] = Value{Kind: Omitted} })
				case "pcurve":
					for _, id := range f.Order {
						en := f.Entities[id]
						if len(en.Records) == 1 && en.Records[0].Type == "CARTESIAN_POINT" && len(en.Records[0].Args[1].Items) == 2 {
							edit(id, func(r *Record) { r.Args[1].Items[0] = Value{Kind: Number, Text: "0.01"} })
							break
						}
					}
				case "orientation":
					edit(find("ADVANCED_FACE"), func(r *Record) { r.Args[3] = Value{Kind: Enumeration, Text: "F"} })
				}
			})
			recovered, e := ReadTorusCylinderOccurrence(ctx, bytes.NewReader(changed), Limits{}, 1e-8)
			if e == nil {
				t.Fatal("accepted", kind)
			}
			var output bytes.Buffer
			if e = recovered.WriteSTEP(&output); e == nil || output.Len() != 0 {
				t.Fatal("partial error")
			}
		})
	}
	for _, limits := range []Limits{{MaxEntities: 5}, {MaxBytes: 1}, {MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}} {
		if _, e := ReadTorusCylinderOccurrence(ctx, bytes.NewReader(raw.Bytes()), limits, 1e-8); e == nil {
			t.Fatal("budget")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadTorusCylinderOccurrence(canceled, bytes.NewReader(raw.Bytes()), Limits{}, 1e-8); e == nil {
		t.Fatal("cancel")
	}
}

func TestTorusCylinderPlacedImportExactDecimalLexemes(t *testing.T) {
	ctx := context.Background()
	source, e := NewTorusCylinderSTEPOccurrence(ctx, torusCylinderSTEPBody(geom.TorusCylinderUnion), torusCylinderExportPose(t, 2), .001, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = source.WriteSTEP(&raw); e != nil {
		t.Fatal(e)
	}
	changed := sideDiskMutateSource(t, raw.Bytes(), func(f *File) {
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != "DIRECTION" || len(en.Records[0].Args[1].Items) != 3 {
				continue
			}
			for i, v := range en.Records[0].Args[1].Items {
				q, e := torusCylinderImportNumber(v)
				if e == nil && q.Cmp(big.NewRat(3, 5)) == 0 {
					en.Records[0].Args[1].Items[i] = Value{Kind: Number, Text: "6.E-1"}
				}
			}
			f.Entities[id] = en
		}
	})
	re := regexp.MustCompile(`#([0-9]+)`)
	renamed := re.ReplaceAllStringFunc(string(changed), func(x string) string { id, _ := strconv.Atoi(x[1:]); return fmt.Sprintf("#%d", id*5+3) })
	recovered, e := ReadTorusCylinderOccurrence(ctx, bytes.NewBufferString(renamed), Limits{}, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var result bytes.Buffer
	if e = recovered.WriteSTEP(&result); e != nil || !bytes.Equal(raw.Bytes(), result.Bytes()) {
		t.Fatal("canonical exact reexport", e)
	}
}
