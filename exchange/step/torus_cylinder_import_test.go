package step

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestTorusCylinderPairedImport(t *testing.T) {
	for _, c := range []float64{4, 6} {
		for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
			for _, caps := range [][2]float64{{-3, 4}, {-4, 3}} {
				body := torusCylinderSTEPBody(op)
				body.CylinderRadius, body.Lower, body.Upper = c, caps[0], caps[1]
				var raw bytes.Buffer
				if e := WriteTorusCylinderSets(&raw, TorusCylinderModel{[]TorusCylinderBody{body}, .001, 1e-8}); e != nil {
					t.Fatal(e)
				}
				s, e := ReadTorusCylinderSet(bytes.NewReader(raw.Bytes()), Limits{}, 1e-8)
				if e != nil {
					t.Fatal(c, op, e)
				}
				recovered, retained := s.Source()
				if retained != (op == geom.TorusCylinderUnion) || recovered.Operation != op || recovered.CylinderRadius != c || s.LengthUnit() != .001 {
					t.Fatal("source", recovered, retained)
				}
				if retained {
					if recovered.Lower != caps[0] || recovered.Upper != caps[1] {
						t.Fatal("visible caps")
					}
				} else {
					if recovered.Upper != math.Nextafter(2, math.Inf(1)) || recovered.Lower != -recovered.Upper {
						t.Fatal("hidden history fabricated")
					}
				}
				radius, h2, roots, e := s.RimProof()
				if e != nil || radius.Cmp(new(big.Rat).SetFloat64(c)) != 0 || h2.Cmp(big.NewRat(3, 1)) != 0 {
					t.Fatal("symbol", e)
				}
				for _, root := range roots {
					sign, e := root.SignPolynomial(context.Background(), []*big.Rat{big.NewRat(-3, 1), new(big.Rat), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
					if e != nil || sign.Sign != 0 {
						t.Fatal("algebraic token", e)
					}
				}
				radius.SetInt64(99)
				again, _, _, _ := s.RimProof()
				if again.Cmp(new(big.Rat).SetFloat64(c)) != 0 {
					t.Fatal("proof alias")
				}
				var output bytes.Buffer
				if e = s.WriteSTEP(&output); e != nil || !bytes.Equal(output.Bytes(), raw.Bytes()) {
					t.Fatal("deterministic reexport", e)
				}
				// Relabel IDs bijectively without depending on vendor labels or payloads.
				re := regexp.MustCompile(`#([0-9]+)`)
				relabeled := re.ReplaceAllStringFunc(raw.String(), func(v string) string { id, _ := strconv.Atoi(v[1:]); return fmt.Sprintf("#%d", id*3+7) })
				if _, e = ReadTorusCylinderSet(strings.NewReader(relabeled), Limits{}, 1e-8); e != nil {
					t.Fatal("entity identity relabel", e)
				}
			}
		}
	}
}

func TestTorusCylinderImportRejectsMutations(t *testing.T) {
	if _, e := ReadTorusCylinderSet(nil, Limits{}, 1e-8); e == nil {
		t.Fatal("nil input")
	}
	b := torusCylinderSTEPBody(geom.TorusCylinderUnion)
	var raw bytes.Buffer
	if e := WriteTorusCylinderSets(&raw, TorusCylinderModel{[]TorusCylinderBody{b}, .001, 1e-8}); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"wrong-seam", "pcurve-owner", "shifted-station", "swapped-cap", "face-orientation", "edge-orientation", "detached", "rim-subfloat", "rim-outside", "source-subfloat"} {
		t.Run(kind, func(t *testing.T) {
			mutated := sideDiskMutateSource(t, raw.Bytes(), func(f *File) {
				find := func(typ string) []int64 {
					var ids []int64
					for _, id := range f.Order {
						for _, r := range f.Entities[id].Records {
							if r.Type == typ {
								ids = append(ids, id)
							}
						}
					}
					return ids
				}
				change := func(id int64, fn func(*Record)) { en := f.Entities[id]; fn(&en.Records[0]); f.Entities[id] = en }
				switch kind {
				case "source-subfloat":
					change(find("TOROIDAL_SURFACE")[0], func(r *Record) { r.Args[2] = Value{Kind: Number, Text: "5.0000000000000000000000000000000000000001"} })
				case "wrong-seam":
					change(find("SEAM_CURVE")[0], func(r *Record) { r.Args[1] = Value{Kind: Reference, Ref: find("CIRCLE")[0]} })
				case "pcurve-owner":
					change(find("PCURVE")[0], func(r *Record) { r.Args[1] = Value{Kind: Reference, Ref: find("CYLINDRICAL_SURFACE")[0]} })
				case "shifted-station":
					plane := f.Entities[find("PLANE")[0]].Records[0]
					pl := f.Entities[plane.Args[1].Ref].Records[0]
					change(pl.Args[1].Ref, func(r *Record) { r.Args[1].Items[2] = Value{Kind: Number, Text: "-3.25"} })
				case "swapped-cap":
					ids := find("ADVANCED_FACE")
					a := f.Entities[ids[3]].Records[0].Args[2]
					z := f.Entities[ids[4]].Records[0].Args[2]
					change(ids[3], func(r *Record) { r.Args[2] = z })
					change(ids[4], func(r *Record) { r.Args[2] = a })
				case "face-orientation":
					change(find("ADVANCED_FACE")[0], func(r *Record) { r.Args[3] = Value{Kind: Enumeration, Text: "F"} })
				case "edge-orientation":
					change(find("ORIENTED_EDGE")[0], func(r *Record) { r.Args[4] = Value{Kind: Enumeration, Text: "F"} })
				case "detached":
					id := int64(999999)
					f.Order = append(f.Order, id)
					f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
				case "rim-subfloat", "rim-outside":
					vertex := f.Entities[find("VERTEX_POINT")[0]].Records[0]
					change(vertex.Args[1].Ref, func(r *Record) {
						q, _ := torusCylinderImportNumber(r.Args[1].Items[2])
						delta := big.NewRat(1, 100)
						digits := 40
						if kind == "rim-subfloat" {
							delta, _ = new(big.Rat).SetString("0.0000000000000000000000000000000000000001")
						}
						q.Add(q, delta)
						r.Args[1].Items[2] = Value{Kind: Number, Text: q.FloatString(digits)}
					})
				}
			})
			result, e := ReadTorusCylinderSet(bytes.NewReader(mutated), Limits{}, 1e-8)
			if e == nil {
				t.Fatal("accepted", kind)
			}
			if source, _ := result.Source(); source.MajorRadius != 0 {
				t.Fatal("partial failure")
			}
		})
	}
	for _, l := range []Limits{{MaxEntities: 5}, {MaxValues: 5}, {MaxBytes: 1}, {MaxDepth: 1}, {MaxVertices: 1}, {MaxFaces: 1}, {MaxEdgeUses: 1}} {
		if _, e := ReadTorusCylinderSet(bytes.NewReader(raw.Bytes()), l, 1e-8); e == nil {
			t.Fatal("budget", l)
		}
	}
	if _, e := ReadTorusCylinderSet(bytes.NewReader(raw.Bytes()), Limits{}, 1e-30); e == nil {
		t.Fatal("unproved tolerance")
	}
	var output bytes.Buffer
	if e := (ImportedTorusCylinder{}).WriteSTEP(&output); e == nil || output.Len() != 0 {
		t.Fatal("zero result write")
	}
}

func TestTorusCylinderImportUnobservableCapsScale(t *testing.T) {
	for _, exp := range []int{-100, 100} {
		scale := math.Ldexp(1, exp)
		b := torusCylinderSTEPBody(geom.TorusCylinderDifference)
		b.MajorRadius *= scale
		b.MinorRadius *= scale
		b.CylinderRadius *= scale
		b.Lower *= scale
		b.Upper *= scale
		var raw bytes.Buffer
		tol := 1e-8 * scale
		if e := WriteTorusCylinderSets(&raw, TorusCylinderModel{[]TorusCylinderBody{b}, .001, tol}); e != nil {
			t.Fatal(exp, e)
		}
		imported, e := ReadTorusCylinderSet(&raw, Limits{}, tol)
		if e != nil {
			t.Fatal(exp, e)
		}
		source, retained := imported.Source()
		if retained || source.Upper <= source.MinorRadius || source.Lower >= -source.MinorRadius || !finiteNumber(source.Upper) {
			t.Fatal("synthetic clear caps", exp)
		}
	}
}
