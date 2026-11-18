package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strconv"
	"testing"
)

func TestRationalHierarchyPlanarLine(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy.step"
		}
		t.Run(name, func(t *testing.T) {
			raw, _ := os.ReadFile("testdata/" + name)
			m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
			if e != nil {
				t.Fatal(e)
			}
			index := -1
			for k, u := range m.Instances {
				if m.Products[u.ProductID].Planar != nil {
					index = k
					break
				}
			}
			u := m.Instances[index]
			cases := []struct {
				name                                                   string
				o, d                                                   [3]int64
				lo, hi                                                 int64
				hits                                                   []int64
				spans, boundary, firstFaces, firstEdges, firstVertices int
			}{
				{"transverse", [3]int64{-12, -2, 0}, [3]int64{1, 0, 0}, -5, 8, []int64{2, 4}, 1, 0, 1, 0, 0},
				{"corner", [3]int64{-12, -6, -4}, [3]int64{1, 1, 1}, -5, 8, []int64{2, 4}, 1, 0, 3, 3, 1},
				{"edge", [3]int64{-10, -4, 0}, [3]int64{0, 0, 1}, -5, 8, []int64{-2, 2}, 0, 1, 3, 3, 1},
				{"face", [3]int64{0, -2, -2}, [3]int64{1, 0, 0}, -12, -6, []int64{-10, -8}, 0, 1, 2, 1, 0},
				{"corner-touch", [3]int64{-10, -4, -2}, [3]int64{1, -1, 0}, -5, 5, []int64{0}, 0, 0, 3, 3, 1},
				{"outside", [3]int64{-11, -4, 0}, [3]int64{0, 0, 1}, -5, 8, nil, 0, 0, 0, 0, 0},
				{"clipped-face", [3]int64{-12, -2, -2}, [3]int64{1, 0, 0}, 3, 4, []int64{4}, 0, 1, 2, 1, 0},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					var origin, direction [3]*big.Rat
					matrix := u.Matrix()
					for i := 0; i < 3; i++ {
						origin[i] = new(big.Rat).Set(matrix[i][3])
						direction[i] = new(big.Rat)
						for j := 0; j < 3; j++ {
							origin[i].Add(origin[i], new(big.Rat).Mul(matrix[i][j], big.NewRat(c.o[j], 1)))
							direction[i].Add(direction[i], new(big.Rat).Mul(matrix[i][j], big.NewRat(c.d[j], 1)))
						}
					}
					query := func(o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
						if vendor {
							return ClipOCCTRationalHierarchyAnalyticLine(ctx, OCCTRationalSphericalHierarchy(m), origin, direction, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), o)
						}
						return ClipRationalHierarchyAnalyticLine(ctx, m, origin, direction, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), o)
					}
					r, e := query(RationalHierarchyLineOptions{})
					if e != nil {
						t.Fatal(e)
					}
					leaf := r.Leaves[index]
					if !leaf.Supported || len(leaf.Spans) != c.spans || len(leaf.BoundarySpans) != c.boundary {
						t.Fatal("span", leaf)
					}
					hits := 0
					for _, h := range r.Hits {
						for _, owner := range h.Owners {
							if !sameLinePath(owner.Path, u.Path) {
								continue
							}
							if hits >= len(c.hits) {
								t.Fatal("extra hit")
							}
							expected, _, _ := geom.NewExactRationalRoot(ctx, big.NewRat(c.hits[hits], 1), geom.ExactCurvePlaneEventOptions{})
							order, e := h.Parameter.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !order.Proven || order.Order != 0 {
								t.Fatal("parameter")
							}
							if hits == 0 && (len(owner.ContactFaceIDs) != c.firstFaces || len(owner.ContactEdgeIDs) != c.firstEdges || len(owner.ContactVertexIDs) != c.firstVertices) {
								t.Fatal("contact ancestry", owner)
							}
							for _, id := range owner.ContactEdgeIDs {
								if _, e = simple(m.Source, id, "EDGE_CURVE", 5); e != nil {
									t.Fatal(e)
								}
							}
							for _, id := range owner.ContactVertexIDs {
								if _, e = simple(m.Source, id, "VERTEX_POINT", 2); e != nil {
									t.Fatal(e)
								}
							}
							hits++
						}
					}
					if hits != len(c.hits) {
						t.Fatal("hit count", hits)
					}
					if c.name == "edge" {
						s := leaf.BoundarySpans[0]
						if len(s.SourceFaceIDs) != 2 || len(s.SourceEdgeIDs) != 1 {
							t.Fatal("edge span ancestry", s)
						}
					}
					if c.name == "face" || c.name == "clipped-face" {
						s := leaf.BoundarySpans[0]
						if len(s.SourceFaceIDs) != 1 || len(s.SourceEdgeIDs) != 0 {
							t.Fatal("face span ancestry", s)
						}
					}
					if out, e := query(RationalHierarchyLineOptions{MaxPlanarTests: r.PlanarTests - 1}); e == nil || len(out.Hits) != 0 {
						t.Fatal("plane budget")
					}
					if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE"); bin != "" && (c.name == "transverse" || c.name == "corner") {
						args := []string{"testdata/" + name}
						for _, v := range [][3]*big.Rat{origin, direction} {
							for _, q := range v {
								v, _ := q.Float64()
								args = append(args, strconv.FormatFloat(v, 'g', 17, 64))
							}
						}
						args = append(args, strconv.FormatInt(c.lo, 10), strconv.FormatInt(c.hi, 10))
						raw, e := exec.Command(bin, args...).Output()
						if e != nil {
							t.Fatal(e)
						}
						var external []float64
						if e = json.Unmarshal(raw, &external); e != nil {
							t.Fatal(e)
						}
						var native []float64
						for _, h := range r.Hits {
							enc, e := h.Parameter.Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
							if e != nil || !enc.Converged {
								t.Fatal(e)
							}
							q := new(big.Rat).Quo(new(big.Rat).Add(enc.Interval.Lower, enc.Interval.Upper), big.NewRat(2, 1))
							v, _ := q.Float64()
							for range h.Owners {
								native = append(native, v)
							}
						}
						if len(native) != len(external) {
							t.Fatal(native, external)
						}
						for i, v := range native {
							if math.Abs(v-external[i]) > 1e-7 {
								t.Fatal(v, external[i])
							}
						}
					}
				})
			}
		})
	}
}

func TestHierarchyPlanarLineRejectsUnsupportedConvexity(t *testing.T) {
	ctx := context.Background()
	outer := []geom.Vec3{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 4}, {X: 0, Y: 4}}
	solid, e := brep.Prism(outer, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteAdvanced(&raw, Model{Bodies: []Body{{Name: "concave", Solid: solid}}, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var id int64
	for _, n := range f.Order {
		for _, r := range f.Entities[n].Records {
			if r.Type == "MANIFOLD_SOLID_BREP" {
				id = n
			}
		}
	}
	body, e := readBody(f, id, 100, 100, 1000, true)
	if e != nil {
		t.Fatal(e)
	}
	br, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	shell, _ := simple(f, br.Args[1].Ref, "CLOSED_SHELL", 2)
	faces, _ := listRefs(shell.Args[1], 1)
	leaf := HierarchyLineLeaf{Owner: HierarchyLineOwner{SourceFaceIDs: faces}}
	p := IndependentExactMaterial{Source: f, Planar: &body}
	a := hierarchyPointArithmetic{bits: 4096}
	out := RationalHierarchyLine{}
	got, e := clipHierarchyPlanar(ctx, leaf, p, RationalHierarchyInstance{}, [3]*big.Rat{}, [3]*big.Rat{}, big.NewRat(0, 1), big.NewRat(1, 1), &a, &out, 65536, nil, nil)
	if e != nil || got.Supported || got.UnsupportedReason == "" {
		t.Fatal(got, e)
	}
}

func TestHierarchyPlanarLineForgedSource(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, false)
	if e != nil {
		t.Fatal(e)
	}
	var bodyID int64
	for _, p := range m.Products {
		if p.Planar != nil {
			bodyID = p.SourceBodyID
		}
	}
	body, _ := simple(m.Source, bodyID, "MANIFOLD_SOLID_BREP", 2)
	shell, _ := simple(m.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
	ids, _ := listRefs(shell.Args[1], 1)
	en := m.Source.Entities[ids[0]]
	if en.Records[0].Args[3].Text == "T" {
		en.Records[0].Args[3].Text = "F"
	} else {
		en.Records[0].Args[3].Text = "T"
	}
	m.Source.Entities[ids[0]] = en
	origin := [3]*big.Rat{big.NewRat(-12, 1), big.NewRat(-2, 1), new(big.Rat)}
	direction := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}
	r, e := ClipRationalHierarchyAnalyticLine(ctx, m, origin, direction, new(big.Rat), big.NewRat(8, 1), RationalHierarchyLineOptions{})
	if e == nil || len(r.Leaves) != 0 || len(r.Hits) != 0 {
		t.Fatal("forged face sense was accepted")
	}
}
