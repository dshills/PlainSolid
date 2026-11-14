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
	"strconv"
	"testing"
)

func TestIndependentCircularFrustumHierarchyNormals(t *testing.T) {
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
	file := filepath.Join(t.TempDir(), "normals.step")
	if e = os.WriteFile(file, source.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name    string
		p       [3]string
		normals map[string][3]string
	}{
		{"wall", [3]string{"3/2", "0", "1/2"}, map[string][3]string{"wall": {"75/2", "0", "15/2"}}},
		{"lower-rim", [3]string{"2", "0", "-2"}, map[string][3]string{"wall": {"50", "0", "10"}, "lower-cap": {"0", "0", "-1"}}},
		{"upper-rim", [3]string{"0", "1", "3"}, map[string][3]string{"wall": {"0", "25", "5"}, "upper-cap": {"0", "0", "1"}}},
		{"inside", [3]string{"0", "0", "0"}, nil},
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
				var point [3]*big.Rat
				for i := 0; i < 3; i++ {
					point[i] = new(big.Rat).SetFloat64(u.Transform[i][3])
					for j := 0; j < 3; j++ {
						point[i].Add(point[i], new(big.Rat).Mul(new(big.Rat).SetFloat64(u.Transform[i][j]), rat(c.p[j])))
					}
				}
				r, e := BoundaryNormalsIndependentCircularFrustumHierarchy(ctx, m, point, RationalHierarchyNormalOptions{})
				if e != nil {
					t.Fatal(e)
				}
				leaf := r.Leaves[index]
				if len(leaf.Normals) != len(c.normals) {
					t.Fatal("normal count", leaf)
				}
				for _, n := range leaf.Normals {
					local, ok := c.normals[n.Role]
					if !ok || n.SourceFaceID == 0 || n.SourceSurfaceID == 0 {
						t.Fatal("owner", n)
					}
					got := n.Vector()
					norm := new(big.Rat)
					for i := 0; i < 3; i++ {
						want := new(big.Rat)
						for j := 0; j < 3; j++ {
							want.Add(want, new(big.Rat).Mul(new(big.Rat).SetFloat64(u.Transform[i][j]), rat(local[j])))
						}
						if got[i].Cmp(want) != 0 {
							t.Fatal("exact ray", got, want)
						}
						norm.Add(norm, new(big.Rat).Mul(got[i], got[i]))
					}
					if norm.Cmp(n.SquaredNorm()) != 0 {
						t.Fatal("norm")
					}
					got[0].SetInt64(999)
					if n.Vector()[0].Cmp(big.NewRat(999, 1)) == 0 {
						t.Fatal("mutable vector")
					}
				}
				if len(c.normals) > 1 {
					if q, e := BoundaryNormalsIndependentCircularFrustumHierarchy(ctx, m, point, RationalHierarchyNormalOptions{MaxNormals: 1}); e == nil || len(q.Leaves) != 0 {
						t.Fatal("quota")
					}
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_NORMALS"); bin != "" {
					args := []string{file}
					for _, x := range point {
						v, _ := x.Float64()
						args = append(args, strconv.FormatFloat(v, 'g', 17, 64))
					}
					b, e := exec.Command(bin, args...).CombinedOutput()
					if e != nil {
						t.Fatal(e, string(b))
					}
					var external [][3]float64
					if e = json.Unmarshal(b, &external); e != nil {
						t.Fatal(e, string(b))
					}
					var ours [][3]float64
					for _, l := range r.Leaves {
						for _, n := range l.Normals {
							var v [3]float64
							norm := 0.
							for i, x := range n.Vector() {
								v[i], _ = x.Float64()
								norm += v[i] * v[i]
							}
							for i := range v {
								v[i] /= math.Sqrt(norm)
							}
							ours = append(ours, v)
						}
					}
					if len(ours) != len(external) {
						t.Fatal("oracle count", ours, external)
					}
					used := map[int]bool{}
					for _, v := range ours {
						found := -1
						for j, w := range external {
							if used[j] {
								continue
							}
							dist := 0.
							for i := range v {
								dist += math.Abs(v[i] - w[i])
							}
							if dist < 1e-6 {
								found = j
								break
							}
						}
						if found < 0 {
							t.Fatal("oracle ray", v, external)
						}
						used[found] = true
					}
				}
			})
		}
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if q, e := BoundaryNormalsIndependentCircularFrustumHierarchy(cancel, m, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, RationalHierarchyNormalOptions{}); e == nil || len(q.Leaves) != 0 {
		t.Fatal("cancel")
	}
	m.Instances[0].Path[0]++
	if q, e := BoundaryNormalsIndependentCircularFrustumHierarchy(ctx, m, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, RationalHierarchyNormalOptions{}); e == nil || len(q.Leaves) != 0 {
		t.Fatal("forged owner")
	}
}
