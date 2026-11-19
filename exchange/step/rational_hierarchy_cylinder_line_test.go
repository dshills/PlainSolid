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

func TestRationalHierarchyCylinderLine(t *testing.T) {
	t.Parallel()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy.step"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			raw, _ := os.ReadFile("testdata/" + name)
			m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
			if e != nil {
				t.Fatal(e)
			}
			index := -1
			for k, u := range m.Instances {
				if m.Products[u.ProductID].Cylinder != nil {
					index = k
					break
				}
			}
			u := m.Instances[index]
			f := m.Products[u.ProductID].Cylinder.Frame
			cases := []struct {
				name            string
				o, d            [3]int64
				lo, hi          int64
				hits            []int64
				spans, boundary int
				roles           []string
			}{
				{"transverse", [3]int64{0, 0, 1}, [3]int64{2, 0, 0}, -5, 5, []int64{-1, 1}, 1, 0, nil},
				{"axial", [3]int64{}, [3]int64{0, 0, 1}, -5, 5, []int64{0, 3}, 1, 0, nil},
				{"reverse-axial", [3]int64{}, [3]int64{0, 0, -1}, -5, 5, []int64{-3, 0}, 1, 0, nil},
				{"rim", [3]int64{0, 0, 1}, [3]int64{2, 0, -1}, -5, 5, []int64{-1, 1}, 1, 0, nil},
				{"lower-cap", [3]int64{}, [3]int64{2, 0, 0}, -5, 5, []int64{-1, 1}, 0, 1, []string{"lower-cap"}},
				{"upper-cap", [3]int64{0, 0, 3}, [3]int64{2, 0, 0}, -5, 5, []int64{-1, 1}, 0, 1, []string{"upper-cap"}},
				{"wall", [3]int64{2, 0, 0}, [3]int64{0, 0, 1}, -5, 5, []int64{0, 3}, 0, 1, []string{"wall"}},
				{"wall-clipped", [3]int64{2, 0, 0}, [3]int64{0, 0, 1}, 1, 2, nil, 0, 1, []string{"wall"}},
				{"outside", [3]int64{3, 0, 0}, [3]int64{0, 0, 1}, -5, 5, nil, 0, 0, nil},
				{"tangent", [3]int64{2, 0, 1}, [3]int64{0, 1, 0}, -5, 5, []int64{0}, 0, 0, nil},
				{"singleton-cap", [3]int64{}, [3]int64{0, 0, 1}, 3, 4, []int64{3}, 0, 0, nil},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					origin, direction := hierarchyTestWorldLine(u, f, c.o, c.d)
					query := func(options RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
						if vendor {
							return ClipOCCTRationalHierarchyAnalyticLine(ctx, OCCTRationalSphericalHierarchy(m), origin, direction, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), options)
						}
						return ClipRationalHierarchyAnalyticLine(ctx, m, origin, direction, big.NewRat(c.lo, 1), big.NewRat(c.hi, 1), options)
					}
					r, e := query(RationalHierarchyLineOptions{})
					if e != nil {
						t.Fatal(e)
					}
					leaf := r.Leaves[index]
					if !leaf.Supported || len(leaf.Spans) != c.spans || len(leaf.BoundarySpans) != c.boundary {
						t.Fatal("span counts", leaf)
					}
					for _, s := range leaf.BoundarySpans {
						if len(s.Roles) != len(c.roles) || s.Roles[0] != c.roles[0] {
							t.Fatal("boundary roles", s.Roles)
						}
					}

					if c.spans+c.boundary > 0 {
						ends := [2]*geom.ExactAlgebraicRoot{}
						if c.spans > 0 {
							ends = [2]*geom.ExactAlgebraicRoot{leaf.Spans[0].Lower, leaf.Spans[0].Upper}
						} else {
							ends = [2]*geom.ExactAlgebraicRoot{leaf.BoundarySpans[0].Lower, leaf.BoundarySpans[0].Upper}
						}
						want := [2]int64{c.lo, c.hi}
						if len(c.hits) > 1 {
							want = [2]int64{c.hits[0], c.hits[len(c.hits)-1]}
						}
						for i, q := range ends {
							r, _, e := geom.NewExactRationalRoot(ctx, big.NewRat(want[i], 1), geom.ExactCurvePlaneEventOptions{})
							if e != nil {
								t.Fatal(e)
							}
							order, e := q.Compare(ctx, r, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !order.Proven || order.Order != 0 {
								t.Fatal("span endpoint", want[i], e)
							}
						}
					}
					var got []int64
					for _, hit := range r.Hits {
						for _, owner := range hit.Owners {
							if !sameLinePath(owner.Path, u.Path) {
								continue
							}
							if len(got) >= len(c.hits) {
								t.Fatal("unexpected hit")
							}
							parameter := c.hits[len(got)]
							expected, _, e := geom.NewExactRationalRoot(ctx, big.NewRat(parameter, 1), geom.ExactCurvePlaneEventOptions{})
							if e != nil {
								t.Fatal(e)
							}
							order, e := hit.Parameter.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !order.Proven || order.Order != 0 {
								t.Fatal("exact parameter", parameter, e)
							}
							got = append(got, parameter)
							if c.name == "rim" && parameter == 1 {
								if len(owner.Supports) != 2 || owner.Multiplicity != 0 {
									t.Fatal("rim simultaneous", owner)
								}
							}
							if c.name == "wall" || c.name == "lower-cap" || c.name == "upper-cap" {
								if len(owner.Supports) != 2 {
									t.Fatal("coincident supports", owner)
								}
							}
							if c.name == "tangent" && owner.Multiplicity != 2 {
								t.Fatal("tangent")
							}
						}
					}
					if len(got) != len(c.hits) {
						t.Fatal("hits", got, c.hits)
					}
					for i, v := range got {
						if v != c.hits[i] {
							t.Fatal(got, c.hits)
						}
					}
					if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE"); bin != "" && (c.name == "transverse" || c.name == "axial" || c.name == "rim") {
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
							mid := new(big.Rat).Quo(new(big.Rat).Add(enc.Interval.Lower, enc.Interval.Upper), big.NewRat(2, 1))
							v, _ := mid.Float64()
							for range h.Owners {
								native = append(native, v)
							}
						}
						if len(native) != len(external) {
							t.Fatal("oracle count", native, external)
						}
						for i, v := range native {
							if math.Abs(v-external[i]) > 1e-7 {
								t.Fatal("oracle parameter", v, external[i])
							}
						}
					}
					if out, e := query(RationalHierarchyLineOptions{MaxOperations: r.Operations - 1}); e == nil || len(out.Hits) != 0 {
						t.Fatal("cumulative limit")
					}
				})
			}
		})
	}
}
