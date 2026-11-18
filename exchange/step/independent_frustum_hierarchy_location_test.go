package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"strconv"
	"testing"
)

func TestIndependentCircularFrustumHierarchyPoint(t *testing.T) {
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
	file := filepath.Join(t.TempDir(), "location.step")
	if e = os.WriteFile(file, source.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name                   string
		p                      [3]string
		location               AnalyticPointLocation
		faces, edges, vertices int
	}{
		{"inside", [3]string{"0", "0", "0"}, AnalyticPointInside, 0, 0, 0},
		{"outside", [3]string{"3", "0", "0"}, AnalyticPointOutside, 0, 0, 0},
		{"wall", [3]string{"0", "3/2", "1/2"}, AnalyticPointBoundary, 1, 0, 0},
		{"seam", [3]string{"3/2", "0", "1/2"}, AnalyticPointBoundary, 1, 1, 0},
		{"nonbinary-seam", [3]string{"7/5", "0", "1"}, AnalyticPointBoundary, 1, 1, 0},
		{"lower", [3]string{"0", "0", "-2"}, AnalyticPointBoundary, 1, 0, 0},
		{"upper", [3]string{"0", "0", "3"}, AnalyticPointBoundary, 1, 0, 0},
		{"lower-rim", [3]string{"0", "2", "-2"}, AnalyticPointBoundary, 2, 1, 0},
		{"upper-rim", [3]string{"0", "1", "3"}, AnalyticPointBoundary, 2, 1, 0},
		{"lower-vertex", [3]string{"2", "0", "-2"}, AnalyticPointBoundary, 2, 2, 1},
		{"upper-vertex", [3]string{"1", "0", "3"}, AnalyticPointBoundary, 2, 2, 1},
		{"above", [3]string{"0", "0", "4"}, AnalyticPointOutside, 0, 0, 0},
	}

	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, side := range []int{-1, 1} {
		x := new(big.Rat).Add(big.NewRat(3, 2), new(big.Rat).Mul(big.NewRat(int64(side), 1), epsilon))
		name := "exact-near-inside"
		location := AnalyticPointInside
		if side > 0 {
			name = "exact-near-outside"
			location = AnalyticPointOutside
		}
		cases = append(cases, struct {
			name                   string
			p                      [3]string
			location               AnalyticPointLocation
			faces, edges, vertices int
		}{name, [3]string{x.RatString(), "0", "1/2"}, location, 0, 0, 0})
	}
	for i, u := range m.Instances {
		p := m.Products[u.ProductID]
		if p.Frustum == nil {
			continue
		}
		for _, c := range cases {
			t.Run(strconv.Itoa(i)+"/"+c.name, func(t *testing.T) {
				var local, world [3]*big.Rat
				for j, s := range c.p {
					local[j], _ = new(big.Rat).SetString(s)
				}
				for j := 0; j < 3; j++ {
					world[j] = new(big.Rat).SetFloat64(u.Transform[j][3])
					for k := 0; k < 3; k++ {
						world[j].Add(world[j], new(big.Rat).Mul(new(big.Rat).SetFloat64(u.Transform[j][k]), local[k]))
					}
				}
				r, e := LocateIndependentCircularFrustumHierarchyPoint(ctx, m, world, RationalHierarchyPointOptions{})
				if e != nil {
					t.Fatal(e)
				}
				leaf := r.Leaves[i]
				if leaf.Location != c.location || len(leaf.ContactFaceIDs) != c.faces || len(leaf.ContactEdgeIDs) != c.edges || len(leaf.ContactVertexIDs) != c.vertices || leaf.ProductID != u.ProductID {
					t.Fatalf("contacts %+v", leaf)
				}
				for _, id := range leaf.ContactEdgeIDs {
					found := false
					for _, want := range p.Frustum.Face.SideEdges {
						found = found || id == want
					}
					if !found {
						t.Fatal("foreign edge")
					}
				}
				for _, id := range leaf.ContactVertexIDs {
					found := false
					for _, want := range p.Frustum.Face.CornerVertices {
						found = found || id == want
					}
					if !found {
						t.Fatal("foreign vertex")
					}
				}
				got := leaf.ProductCoordinates()
				for j := 0; j < 3; j++ {
					if got[j].Cmp(local[j]) != 0 {
						t.Fatal("source coordinates")
					}
				}
				got[0].SetInt64(999)
				if leaf.ProductCoordinates()[0].Cmp(big.NewRat(999, 1)) == 0 {
					t.Fatal("mutable observation")
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LOCATION"); bin != "" && c.name != "exact-near-inside" && c.name != "exact-near-outside" {
					args := []string{file}
					for _, x := range world {
						v, _ := x.Float64()
						args = append(args, strconv.FormatFloat(v, 'g', 17, 64))
					}
					b, e := exec.Command(bin, args...).CombinedOutput()
					if e != nil {
						t.Fatal(e, string(b))
					}
					at := bytes.LastIndex(b, []byte("{"))
					var oracle struct{ Inside, Boundary, Outside int }
					if at < 0 || json.Unmarshal(b[at:], &oracle) != nil {
						t.Fatal(string(b))
					}
					got := oracle
					got.Inside = 0
					got.Boundary = 0
					got.Outside = 0
					for _, x := range r.Leaves {
						switch x.Location {
						case AnalyticPointInside:
							got.Inside++
						case AnalyticPointOutside:
							got.Outside++
						case AnalyticPointBoundary:
							got.Boundary++
						default:
							t.Fatal("unsupported")
						}
					}
					if got != oracle {
						t.Fatal("independent states", got, oracle)
					}
				}
			})
		}
	}
	point := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for _, o := range []RationalHierarchyPointOptions{{MaxPlanarTests: 1}, {Source: Limits{MaxSolids: 4}}} {
		if r, e := LocateIndependentCircularFrustumHierarchyPoint(ctx, m, point, o); e == nil || len(r.Leaves) != 0 {
			t.Fatal("partial budget")
		}
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if r, e := LocateIndependentCircularFrustumHierarchyPoint(cancel, m, point, RationalHierarchyPointOptions{}); e == nil || len(r.Leaves) != 0 {
		t.Fatal("cancel")
	}
	m.Instances[0].Path[0]++
	if r, e := LocateIndependentCircularFrustumHierarchyPoint(ctx, m, point, RationalHierarchyPointOptions{}); e == nil || len(r.Leaves) != 0 {
		t.Fatal("forged path")
	}
}
func TestIndependentCircularFrustumPointNoncircular(t *testing.T) {
	ctx := context.Background()
	model := frustumHierarchyModel(t)
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
	var b bytes.Buffer
	if e = WriteIndependentFrustumHierarchy(ctx, &b, IndependentFrustumHierarchyModel(model), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := LocateIndependentCircularFrustumHierarchyPoint(ctx, m, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, RationalHierarchyPointOptions{}); e == nil || len(r.Leaves) != 0 {
		t.Fatal("noncircular promotion")
	}
}
