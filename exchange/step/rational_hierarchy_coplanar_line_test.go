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

func TestGeneralPlanarCoplanarHierarchyLine(t *testing.T) {
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		for _, family := range []string{"reentrant", "holed"} {
			name := "rational-spherical-hierarchy-" + family + ".step"
			if vendor {
				name = "occt-" + name
			}
			t.Run(name, func(t *testing.T) {
				raw, _ := os.ReadFile("testdata/" + name)
				m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
				if e != nil {
					t.Fatal(e)
				}

				var native bytes.Buffer
				if vendor {
					if e = WriteOCCTRationalSphericalHierarchyJSON(ctx, &native, OCCTRationalSphericalHierarchy(m)); e != nil {
						t.Fatal(e)
					}
					again, e := ReadOCCTRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
					if e != nil {
						t.Fatal(e)
					}
					m = RationalSphericalHierarchy(again)
				} else {
					if e = WriteRationalSphericalHierarchyJSON(ctx, &native, m); e != nil {
						t.Fatal(e)
					}
					m, e = ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
					if e != nil {
						t.Fatal(e)
					}
				}
				index := -1
				for k, u := range m.Instances {
					if m.Products[u.ProductID].Planar != nil {
						index = k
					}
				}
				u := m.Instances[index]
				type sample struct {
					name               string
					o, d               [3]int64
					lo, hi             int64
					hits               []int64
					interior, boundary [][2]int64
					faces, edges       int
					oracle             bool
				}
				cases := []sample{{"cap-disjoint", [3]int64{-11, 0, -2}, [3]int64{1, -1, 0}, 0, 6, []int64{1, 2, 3, 4}, nil, [][2]int64{{1, 2}, {3, 4}}, 1, 0, true}, {"reentrant-edge", [3]int64{-9, -3, -3}, [3]int64{0, 0, 1}, 0, 6, []int64{1, 5}, nil, [][2]int64{{1, 5}}, 2, 1, true}, {"clipped-edge", [3]int64{-9, -3, 0}, [3]int64{0, 0, 1}, 0, 1, nil, nil, [][2]int64{{0, 1}}, 2, 1, false}, {"cap-tangent", [3]int64{-10, -4, -2}, [3]int64{1, -1, 0}, -1, 1, []int64{0}, nil, nil, 0, 0, true}}
				if family == "holed" {
					cases = []sample{{"cap-hole-reversed", [3]int64{-5, -2, -2}, [3]int64{-1, 0, 0}, 0, 6, []int64{1, 2, 4, 5}, nil, [][2]int64{{1, 2}, {4, 5}}, 1, 0, true}, {"hole-interior-to-edge", [3]int64{-8, -2, -2}, [3]int64{0, 1, 0}, 0, 1, []int64{1}, nil, nil, 0, 0, true}, {"cap-hole", [3]int64{-11, -2, -2}, [3]int64{1, 0, 0}, 0, 6, []int64{1, 2, 4, 5}, nil, [][2]int64{{1, 2}, {4, 5}}, 1, 0, true}, {"hole-wall", [3]int64{-11, -3, 0}, [3]int64{1, 0, 0}, 0, 6, []int64{1, 2, 4, 5}, [][2]int64{{1, 2}, {4, 5}}, [][2]int64{{2, 4}}, 1, 0, true}, {"hole-edge", [3]int64{-9, -3, -3}, [3]int64{0, 0, 1}, 0, 6, []int64{1, 5}, nil, [][2]int64{{1, 5}}, 2, 1, true}}
				}
				for _, c := range cases {
					t.Run(c.name, func(t *testing.T) {
						frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
						origin, direction := hierarchyTestWorldLine(u, frame, c.o, c.d)
						query := func(o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
							if vendor {
								return ClipCoplanarGeneralOCCTRationalHierarchyLine(ctx, OCCTRationalSphericalHierarchy(m), origin, direction, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), o)
							}
							return ClipCoplanarGeneralRationalHierarchyLine(ctx, m, origin, direction, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), o)
						}
						r, e := query(RationalHierarchyLineOptions{})
						if e != nil {
							t.Fatal(e)
						}
						leaf := r.Leaves[index]
						if !leaf.Supported || len(leaf.Spans) != len(c.interior) || len(leaf.BoundarySpans) != len(c.boundary) {
							t.Fatal("spans", leaf)
						}
						equal := func(root *geom.ExactAlgebraicRoot, v int64) {
							expected, _, _ := geom.NewExactRationalRoot(ctx, big.NewRat(v, 1), geom.ExactCurvePlaneEventOptions{})
							order, e := root.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !order.Proven || order.Order != 0 {
								t.Fatal("parameter", v, e)
							}
						}
						for i, s := range leaf.Spans {
							equal(s.Lower, c.interior[i][0])
							equal(s.Upper, c.interior[i][1])
						}
						for i, s := range leaf.BoundarySpans {
							equal(s.Lower, c.boundary[i][0])
							equal(s.Upper, c.boundary[i][1])
							if len(s.SourceFaceIDs) != c.faces || len(s.SourceEdgeIDs) != c.edges {
								t.Fatal("span ownership", s)
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
								if len(owner.ContactFaceIDs) == 0 {
									t.Fatal("missing owner")
								}
								hits++
							}
						}
						if hits != len(c.hits) {
							t.Fatal("hit count", hits)
						}
						if out, e := query(RationalHierarchyLineOptions{MaxPlanarTests: r.PlanarTests - 1}); e == nil || len(out.Hits) != 0 {
							t.Fatal("work limit")
						}
						old, e := clipRationalHierarchyLine(ctx, m, origin, direction, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), RationalHierarchyLineOptions{}, vendor, true)
						if e != nil || old.Leaves[index].Supported || old.Leaves[index].UnsupportedReason == "" {
							t.Fatal("transverse gate changed", e)
						}
						if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE"); bin != "" && c.oracle {
							args := []string{"testdata/" + name}
							for _, v := range [][3]*big.Rat{origin, direction} {
								for _, q := range v {
									f, _ := q.Float64()
									args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
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
}
