package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strconv"
	"testing"
)

func TestIndependentCircularFrustumHierarchyLine(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	var source bytes.Buffer
	if e := WriteIndependentFrustumHierarchy(ctx, &source, IndependentFrustumHierarchyModel(frustumHierarchyModel(t)), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(source.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentFrustumHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadIndependentFrustumHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "line.step")
	if e = os.WriteFile(file, source.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name             string
		o, d             [3]string
		lo, hi           string
		hits             []string
		inside, boundary int
		live             bool
	}{
		{"transverse", [3]string{"0", "0", "1/2"}, [3]string{"3/2", "0", "0"}, "-4", "4", []string{"-1", "1"}, 1, 0, true},
		{"axial", [3]string{"0", "0", "0"}, [3]string{"0", "0", "1"}, "-4", "4", []string{"-2", "3"}, 1, 0, true},
		{"reverse", [3]string{"0", "0", "0"}, [3]string{"0", "0", "-1"}, "-4", "4", []string{"-3", "2"}, 1, 0, true},
		{"lower-cap", [3]string{"0", "0", "-2"}, [3]string{"2", "0", "0"}, "-4", "4", []string{"-1", "1"}, 0, 1, false},
		{"upper-cap", [3]string{"0", "0", "3"}, [3]string{"1", "0", "0"}, "-4", "4", []string{"-1", "1"}, 0, 1, false},
		{"seam-generator", [3]string{"2", "0", "-2"}, [3]string{"-1", "0", "5"}, "-1", "2", []string{"0", "1"}, 0, 1, false},
		{"other-generator", [3]string{"0", "2", "-2"}, [3]string{"0", "-1", "5"}, "-1", "2", []string{"0", "1"}, 0, 1, false},
		{"clipped-generator", [3]string{"2", "0", "-2"}, [3]string{"-1", "0", "5"}, "1/4", "3/4", nil, 0, 1, false},
		{"tangent", [3]string{"3/2", "0", "1/2"}, [3]string{"0", "1", "0"}, "-1", "1", []string{"0"}, 0, 0, false},
		{"negative-leading", [3]string{"3/2", "0", "0"}, [3]string{"0", "0", "1"}, "-4", "4", []string{"-2", "1/2"}, 1, 0, true},
		{"linear", [3]string{"0", "0", "-2"}, [3]string{"-1", "0", "5"}, "-1", "2", []string{"0", "1"}, 1, 0, true},
		{"irrational", [3]string{"0", "1", "1/2"}, [3]string{"1", "0", "0"}, "-4", "4", []string{"", ""}, 1, 0, true},
		{"endpoint", [3]string{"0", "0", "0"}, [3]string{"0", "0", "1"}, "3", "4", []string{"3"}, 0, 0, false},
	}
	rat := func(s string) *big.Rat {
		q, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return q
	}
	for index, u := range m.Instances {
		if m.Products[u.ProductID].Frustum == nil {
			continue
		}
		for _, c := range cases {
			t.Run(strconv.Itoa(index)+"/"+c.name, func(t *testing.T) {
				var origin, direction [3]*big.Rat
				for i := 0; i < 3; i++ {
					origin[i] = new(big.Rat).SetFloat64(u.Transform[i][3])
					direction[i] = new(big.Rat)
					for j := 0; j < 3; j++ {
						factor := new(big.Rat).SetFloat64(u.Transform[i][j])
						origin[i].Add(origin[i], new(big.Rat).Mul(factor, rat(c.o[j])))
						direction[i].Add(direction[i], new(big.Rat).Mul(factor, rat(c.d[j])))
					}
				}
				query := func(o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
					return ClipIndependentCircularFrustumHierarchyLine(ctx, m, origin, direction, rat(c.lo), rat(c.hi), o)
				}
				r, e := query(RationalHierarchyLineOptions{})
				if e != nil {
					t.Fatal(e)
				}
				leaf := r.Leaves[index]
				if !leaf.Supported || len(leaf.Spans) != c.inside || len(leaf.BoundarySpans) != c.boundary {
					t.Fatal("spans", leaf)
				}

				if c.inside+c.boundary > 0 && c.name != "irrational" {
					wantLo, wantHi := c.lo, c.hi
					if len(c.hits) > 1 {
						wantLo, wantHi = c.hits[0], c.hits[len(c.hits)-1]
					}
					var endpoints [2]*geom.ExactAlgebraicRoot
					if c.inside > 0 {
						endpoints = [2]*geom.ExactAlgebraicRoot{leaf.Spans[0].Lower, leaf.Spans[0].Upper}
					} else {
						endpoints = [2]*geom.ExactAlgebraicRoot{leaf.BoundarySpans[0].Lower, leaf.BoundarySpans[0].Upper}
					}
					for j, want := range []string{wantLo, wantHi} {
						q, _, _ := geom.NewExactRationalRoot(ctx, rat(want), geom.ExactCurvePlaneEventOptions{})
						cmp, e := endpoints[j].Compare(ctx, q, geom.ExactCurvePlaneEventOptions{})
						if e != nil || !cmp.Proven || cmp.Order != 0 {
							t.Fatal("exact span clipping", j, want, e)
						}
					}
				}
				n := 0
				for _, hit := range r.Hits {
					for _, own := range hit.Owners {
						if !sameLinePath(own.Path, u.Path) {
							continue
						}
						if n >= len(c.hits) {
							t.Fatal("extra hit")
						}
						if c.hits[n] != "" {
							expected, _, _ := geom.NewExactRationalRoot(ctx, rat(c.hits[n]), geom.ExactCurvePlaneEventOptions{})
							cmp, e := hit.Parameter.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !cmp.Proven || cmp.Order != 0 {
								t.Fatal("parameter", n, e)
							}
						} else {
							sign, e := hit.Parameter.SignPolynomial(ctx, []*big.Rat{big.NewRat(-5, 1), new(big.Rat), big.NewRat(4, 1)}, geom.ExactCurvePlaneEventOptions{})
							if e != nil || sign.Sign != 0 {
								t.Fatal("irrational polynomial", e)
							}
						}
						n++
						if c.name == "tangent" && own.Multiplicity != 2 {
							t.Fatal("tangent multiplicity", own)
						}
						if len(own.ContactFaceIDs) == 0 {
							t.Fatal("missing actual face")
						}
						for _, support := range own.Supports {
							if support.SourceFaceID == 0 {
								t.Fatal("unowned support")
							}
						}
						if c.name == "seam-generator" {
							if len(own.ContactFaceIDs) != 2 || len(own.ContactEdgeIDs) != 2 || len(own.ContactVertexIDs) != 1 {
								t.Fatal("seam endpoint incidence", own)
							}
						}
					}
				}
				if n != len(c.hits) {
					t.Fatal("missing hits", n, c.hits)
				}
				for _, span := range leaf.BoundarySpans {
					if len(span.SourceFaceIDs) != 1 {
						t.Fatal("span face")
					}
					if c.name == "seam-generator" || c.name == "clipped-generator" {
						if len(span.SourceEdgeIDs) != 1 {
							t.Fatal("seam span")
						}
					}
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE"); bin != "" && c.live {
					args := []string{file}
					for _, q := range append(origin[:], direction[:]...) {
						v, _ := q.Float64()
						args = append(args, strconv.FormatFloat(v, 'g', 17, 64))
					}
					args = append(args, c.lo, c.hi)
					b, e := exec.Command(bin, args...).CombinedOutput()
					if e != nil {
						t.Fatal(e, string(b))
					}
					var external []float64
					if e = json.Unmarshal(b, &external); e != nil {
						t.Fatal(e, string(b))
					}
					var ours []float64
					for _, hit := range r.Hits {
						en, e := hit.Parameter.Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
						if e != nil || !en.Converged {
							t.Fatal(e)
						}
						v, _ := en.Interval.Lower.Float64()
						for range hit.Owners {
							ours = append(ours, v)
						}
					}
					if len(ours) != len(external) {
						t.Fatal("live count", ours, external)
					}
					for i, x := range ours {
						if math.Abs(x-external[i]) > 1e-7 {
							t.Fatal("live parameter", ours, external)
						}
					}
				}
				if c.name == "transverse" {
					if q, e := query(RationalHierarchyLineOptions{MaxOperations: r.Operations - 1}); e == nil || len(q.Hits) != 0 {
						t.Fatal("cumulative work")
					}
				}
			})
		}
	}
}

func TestIndependentCircularFrustumLineFailures(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	model := frustumHierarchyModel(t)
	var b bytes.Buffer
	if e := WriteIndependentFrustumHierarchy(ctx, &b, IndependentFrustumHierarchyModel(model), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	direction := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}
	query := func(ctx context.Context, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
		return ClipIndependentCircularFrustumHierarchyLine(ctx, m, origin, direction, big.NewRat(-4, 1), big.NewRat(4, 1), o)
	}
	for _, o := range []RationalHierarchyLineOptions{{MaxOperations: 1}, {MaxPlanarTests: 1}, {Source: Limits{MaxSolids: 4}}} {
		r, e := query(ctx, o)
		if e == nil || len(r.Hits) != 0 || len(r.Leaves) != 0 {
			t.Fatal("partial work")
		}
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if r, e := query(cancel, RationalHierarchyLineOptions{}); e == nil || len(r.Leaves) != 0 {
		t.Fatal("cancel")
	}
	m.Instances[0].Path[0]++
	if r, e := query(ctx, RationalHierarchyLineOptions{}); e == nil || len(r.Leaves) != 0 {
		t.Fatal("forged path")
	}
	raw, e := os.ReadFile("testdata/wrapped-frustum.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range model.Products {
		if model.Products[i].ID == "frustum" {
			model.Products[i].Source = f
		}
	}
	b.Reset()
	if e = WriteIndependentFrustumHierarchy(ctx, &b, IndependentFrustumHierarchyModel(model), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e = ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := query(ctx, RationalHierarchyLineOptions{}); e == nil || len(r.Leaves) != 0 {
		t.Fatal("noncircular promotion")
	}
}
