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

func hierarchyTestWorldLine(u RationalHierarchyInstance, f geom.Frame, o, d [3]int64) (origin, direction [3]*big.Rat) {
	axes := [3][3]*big.Rat{mappedRatVec(f.XAxis()), mappedRatVec(f.YAxis()), mappedRatVec(f.ZAxis())}
	local := mappedRatVec(f.Origin())
	var delta [3]*big.Rat
	for i := 0; i < 3; i++ {
		delta[i] = new(big.Rat)
		for j := 0; j < 3; j++ {
			local[i].Add(local[i], new(big.Rat).Mul(axes[j][i], big.NewRat(o[j], 1)))
			delta[i].Add(delta[i], new(big.Rat).Mul(axes[j][i], big.NewRat(d[j], 1)))
		}
	}
	mat := u.Matrix()
	for i := 0; i < 3; i++ {
		origin[i] = new(big.Rat).Set(mat[i][3])
		direction[i] = new(big.Rat)
		for j := 0; j < 3; j++ {
			origin[i].Add(origin[i], new(big.Rat).Mul(mat[i][j], local[j]))
			direction[i].Add(direction[i], new(big.Rat).Mul(mat[i][j], delta[j]))
		}
	}
	return
}
func TestRationalHierarchyLine(t *testing.T) {
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
			seen := map[string]bool{}
			for k, u := range m.Instances {
				p := m.Products[u.ProductID]
				var f geom.Frame
				family := ""
				if p.Sphere != nil {
					family = "sphere"
					f = p.Sphere.Frame
				}
				if p.Torus != nil {
					family = "torus"
					f = p.Torus.Frame
				}
				if family == "" || seen[family] {
					continue
				}
				seen[family] = true
				for _, tangent := range []bool{false, true} {
					local := [3]int64{}
					dir := [3]int64{2, 0, 0}
					want := []int64{-1, 1}
					spans := 1
					if family == "torus" {
						want = []int64{-2, -1, 1, 2}
						spans = 2
					}
					if tangent {
						spans = 0
						if family == "sphere" {
							local[1] = 2
							want = []int64{0}
						} else {
							local[2] = 1
							dir[0] = 1
							want = []int64{-3, 3}
						}
					}
					origin, direction := hierarchyTestWorldLine(u, f, local, dir)
					query := func(o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
						if vendor {
							return ClipOCCTRationalHierarchyAnalyticLine(ctx, OCCTRationalSphericalHierarchy(m), origin, direction, big.NewRat(-5, 1), big.NewRat(5, 1), o)
						}
						return ClipRationalHierarchyAnalyticLine(ctx, m, origin, direction, big.NewRat(-5, 1), big.NewRat(5, 1), o)
					}
					r, e := query(RationalHierarchyLineOptions{})
					if e != nil {
						t.Fatal(family, tangent, e)
					}
					if len(r.Leaves) != 7 || len(r.Leaves[k].Spans) != spans {
						t.Fatal("spans", family, tangent, r.Leaves[k])
					}
					var got []int64
					for i, h := range r.Hits {
						if i > 0 {
							order, e := r.Hits[i-1].Parameter.Compare(ctx, h.Parameter, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !order.Proven || order.Order >= 0 {
								t.Fatal("global order")
							}
						}
						for _, owner := range h.Owners {
							if owner.ProductID != u.ProductID || !sameLinePath(owner.Path, u.Path) {
								continue
							}
							c, e := h.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
							if e != nil || c.Rational == nil || !c.Rational.IsInt() {
								t.Fatal("root", c, e)
							}
							got = append(got, c.Rational.Num().Int64())
							mult := 1
							if tangent {
								mult = 2
							}
							if owner.Multiplicity != mult {
								t.Fatal("multiplicity")
							}
						}
					}
					if len(got) != len(want) {
						t.Fatal("hits", got, want)
					}
					for i := range want {
						if got[i] != want[i] {
							t.Fatal(got, want)
						}
					}

					if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LINE"); bin != "" && !tangent {
						args := []string{"testdata/" + name}
						for _, v := range [][3]*big.Rat{origin, direction} {
							for _, q := range v {
								f, _ := q.Float64()
								args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
							}
						}
						args = append(args, "-5", "5")
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
							t.Fatal("oracle hit count", native, external)
						}
						for i, v := range native {
							if math.Abs(v-external[i]) > 1e-7 {
								t.Fatal("oracle parameter", v, external[i])
							}
						}
					}

					if out, e := query(RationalHierarchyLineOptions{MaxOperations: r.Operations - 1}); e == nil || len(out.Hits) != 0 {
						t.Fatal("cumulative operation limit", r.Operations)
					}
					if out, e := query(RationalHierarchyLineOptions{MaxOperations: 1}); e == nil || len(out.Hits) != 0 {
						t.Fatal("budget")
					}
				}
			}
		})
	}
}
func sameLinePath(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRationalHierarchyLineSimultaneousAndEndpoints(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, false)
	if e != nil {
		t.Fatal(e)
	}
	var uses []RationalHierarchyInstance
	for _, u := range m.Instances {
		if m.Products[u.ProductID].Sphere != nil {
			uses = append(uses, u)
		}
	}
	if len(uses) != 2 {
		t.Fatal("fixture")
	}
	center := m.Products[uses[0].ProductID].Sphere.Frame.Origin()
	c0, _ := uses[0].ApplyExact(center)
	c1, _ := uses[1].ApplyExact(center)
	var relation ExactNAUOOccurrence
	for _, r := range m.Relationships {
		if r.NAUOID == uses[1].Path[0] {
			relation = r
		}
	}
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	fr, _ := simple(f, relation.ToPlacementID, "AXIS2_PLACEMENT_3D", 4)
	en := f.Entities[fr.Args[1].Ref]
	for i := 0; i < 3; i++ {
		q, ok := new(big.Rat).SetString(en.Records[0].Args[1].Items[i].Text)
		if !ok {
			t.Fatal("number")
		}
		q.Add(q, new(big.Rat).Sub(c0[i], c1[i]))
		s, e := extrusionDirectionDecimal(q)
		if e != nil {
			t.Fatal(e)
		}
		en.Records[0].Args[1].Items[i].Text = s
	}
	f.Entities[fr.Args[1].Ref] = en
	altered, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	m, e = readRationalSphericalHierarchy(ctx, bytes.NewReader(altered), Limits{}, 1e-9, false)
	if e != nil {
		t.Fatal(e)
	}
	dir := [3]*big.Rat{big.NewRat(2, 1), new(big.Rat), new(big.Rat)}
	r, e := ClipRationalHierarchyAnalyticLine(ctx, m, c0, dir, big.NewRat(-1, 1), big.NewRat(1, 1), RationalHierarchyLineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Hits) != 2 {
		t.Fatal("hits", len(r.Hits))
	}
	for _, h := range r.Hits {
		if len(h.Owners) != 2 || sameLinePath(h.Owners[0].Path, h.Owners[1].Path) {
			t.Fatal("simultaneous ownership")
		}
	}
	for _, l := range r.Leaves {
		if l.Owner.Family == "sphere" && (l.LowerSign != 0 || l.UpperSign != 0 || len(l.Spans) != 1) {
			t.Fatal("endpoint clipping")
		}
	}
	// Same spheres, irrational +/-sqrt(3)/2 roots and identical cross-owner roots.
	origin := copyHierarchyRats(c0)
	origin[1].Add(origin[1], big.NewRat(1, 1))
	r, e = ClipRationalHierarchyAnalyticLine(ctx, m, origin, dir, big.NewRat(-1, 1), big.NewRat(1, 1), RationalHierarchyLineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, h := range r.Hits {
		if len(h.Owners) != 2 {
			t.Fatal("irrational owners")
		}
		c, e := h.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || c.Kind != geom.ExactRootKindIrrational {
			t.Fatal(c, e)
		}
	}
}
