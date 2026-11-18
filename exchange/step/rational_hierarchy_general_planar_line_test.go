package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"strconv"
	"testing"
)

func TestGeneralPlanarHierarchyLine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy-reentrant.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy-reentrant.step"
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
				}
			}
			u := m.Instances[index]
			cases := []struct {
				name        string
				o, d        [3]int64
				hits        []string
				spans       [][2]string
				unsupported bool
			}{
				{"multiple", [3]int64{-11, 0, 0}, [3]int64{1, -1, 0}, []string{"1", "2", "3", "4"}, [][2]string{{"1", "2"}, {"3", "4"}}, false},
				{"nonunit", [3]int64{-11, 0, 0}, [3]int64{2, -2, 0}, []string{"1/2", "1", "3/2", "2"}, [][2]string{{"1/2", "1"}, {"3/2", "2"}}, false},
				{"reentrant-contact", [3]int64{-11, -1, 0}, [3]int64{1, -1, 0}, []string{"1", "2", "3"}, [][2]string{{"1", "2"}, {"2", "3"}}, false},
				{"vertex", [3]int64{-11, -5, -3}, [3]int64{1, 1, 1}, []string{"1", "2"}, [][2]string{{"1", "2"}}, false},
				{"coplanar-cap", [3]int64{-11, 0, -2}, [3]int64{1, -1, 0}, nil, nil, true},
				{"coplanar-wall", [3]int64{-10, -5, 0}, [3]int64{0, 1, 0}, nil, nil, true},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
					origin, direction := hierarchyTestWorldLine(u, frame, c.o, c.d)
					query := func(o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
						if vendor {
							return ClipGeneralOCCTRationalHierarchyLine(ctx, OCCTRationalSphericalHierarchy(m), origin, direction, new(big.Rat), big.NewRat(6, 1), o)
						}
						return ClipGeneralRationalHierarchyLine(ctx, m, origin, direction, new(big.Rat), big.NewRat(6, 1), o)
					}
					r, e := query(RationalHierarchyLineOptions{})
					if e != nil {
						t.Fatal(e)
					}
					leaf := r.Leaves[index]
					if leaf.Supported == c.unsupported || len(leaf.Spans) != len(c.spans) || len(leaf.BoundarySpans) != 0 {
						t.Fatal("scope/spans", leaf)
					}
					if c.unsupported && leaf.UnsupportedReason == "" {
						t.Fatal("reason")
					}
					equal := func(root *geom.ExactAlgebraicRoot, s string) {
						q, _ := new(big.Rat).SetString(s)
						expected, _, e := geom.NewExactRationalRoot(ctx, q, geom.ExactCurvePlaneEventOptions{})
						if e != nil {
							t.Fatal(e)
						}
						order, e := root.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
						if e != nil || !order.Proven || order.Order != 0 {
							t.Fatal("parameter", s, e)
						}
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
							equal(h.Parameter, c.hits[hits])
							if c.name == "vertex" && hits == 0 {
								if len(owner.ContactFaceIDs) != 3 || len(owner.ContactEdgeIDs) != 3 || len(owner.ContactVertexIDs) != 1 {
									t.Fatal("corner identities", owner)
								}
							}
							if c.name == "reentrant-contact" && hits == 1 {
								if len(owner.ContactFaceIDs) != 2 || len(owner.ContactEdgeIDs) != 1 {
									t.Fatal("reentrant identities")
								}
							}
							hits++
						}
					}
					if hits != len(c.hits) {
						t.Fatal("hit count", hits)
					}
					for i, span := range leaf.Spans {
						equal(span.Lower, c.spans[i][0])
						equal(span.Upper, c.spans[i][1])
					}
					if !c.unsupported {
						if out, e := query(RationalHierarchyLineOptions{MaxRayDirections: 1}); e == nil || len(out.Hits) != 0 {
							t.Fatal("ray quota")
						}
						if out, e := query(RationalHierarchyLineOptions{MaxPlanarTests: r.PlanarTests - 1}); e == nil || len(out.Hits) != 0 {
							t.Fatal("predicate quota")
						}
					}
					if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE"); bin != "" && !c.unsupported {
						args := []string{"testdata/" + name}
						for _, v := range [][3]*big.Rat{origin, direction} {
							for _, q := range v {
								f, _ := q.Float64()
								args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
							}
						}
						args = append(args, "0", "6")
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
							t.Fatal("oracle count", native, external)
						}
						for i, v := range native {
							if math.Abs(v-external[i]) > 1e-7 {
								t.Fatal("oracle", v, external[i])
							}
						}
					}
				})
			}
		})
	}
}
