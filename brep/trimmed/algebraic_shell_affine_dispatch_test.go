package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"strings"
	"testing"
	"time"
)

func TestCombineSplineShellsAffinePaired(t *testing.T) {
	ctx := context.Background()
	for fixture := 0; fixture < 2; fixture++ {
		a, b := pairedAffineShellFixture(t)
		if fixture == 1 {
			a, b = affineMaterialOnePairedFixture(t)
		}
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			out, h, e := CombineSplineShells(ctx, a, b, op, SplineShellCombineOptions{AffinePaired: &ConvexAffineSplineShellOptions{}})
			if fixture == 0 && op == parametric.Difference {
				if e == nil || len(out.Faces()) != 0 || h.AffinePaired != nil || h.Method != "" {
					t.Fatal("singular fallback/publication")
				}
				continue
			}
			if e != nil {
				t.Fatal(e)
			}
			if h.Method != "exact-convex-affine-paired" || h.AffinePaired == nil || len(h.AffinePaired.FaceSources()) != len(out.faces) {
				t.Fatal("lost proof history")
			}
			faces, edges := map[int]bool{}, map[int]bool{}
			for _, table := range [][][]int{h.FacesA, h.FacesB} {
				for _, ids := range table {
					for _, id := range ids {
						if faces[id] {
							t.Fatal("duplicate face owner")
						}
						faces[id] = true
					}
				}
			}
			for _, table := range [][][]int{h.EdgesA, h.EdgesB} {
				for _, ids := range table {
					for _, id := range ids {
						edges[id] = true
					}
				}
			}
			for _, ids := range [][]int{h.SectionEdges, h.GeneratedEdges} {
				for _, id := range ids {
					edges[id] = true
				}
			}
			if len(faces) != len(out.faces) || len(edges) != len(out.edges) || len(h.SectionEdges) == 0 || len(h.GeneratedEdges) == 0 {
				t.Fatal("incomplete ancestry")
			}
			direct, e := CombineConvexAffineSplineShells(ctx, a, b, op, ConvexAffineSplineShellOptions{})
			if e != nil || direct.ExactVolume().Cmp(h.AffinePaired.ExactVolume()) != 0 {
				t.Fatal("normal/direct mismatch", e)
			}
		}
	}
	a, b := pairedAffineShellFixture(t)
	for _, o := range []SplineShellCombineOptions{
		{AffinePaired: &ConvexAffineSplineShellOptions{Authorization: AffineShellPairedImprintOptions{MaxFacePairs: 35}}},
		{AffinePaired: &ConvexAffineSplineShellOptions{Material: AffineShellMaterialOptions{MaxOperations: 1}}},
		{AffinePaired: &ConvexAffineSplineShellOptions{}, Algebraic: &SplineShellAlgebraicNativeOptions{}},
		{AffinePaired: &ConvexAffineSplineShellOptions{}, DisableAffineDiscovery: true},
		{AffinePaired: &ConvexAffineSplineShellOptions{}, DiscoverVerticalQuadraticWarp: true},
	} {
		out, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, o)
		if e == nil || len(out.faces) != 0 || h.Method != "" || h.AffinePaired != nil {
			t.Fatal("non-atomic dispatch")
		}
	}
}

func TestOCCTAffinePairedMaterial(t *testing.T) {
	path := os.Getenv("PLAINSOLID_OCCT_AFFINE_PAIRED")
	if path == "" {
		t.Skip("set PLAINSOLID_OCCT_AFFINE_PAIRED to independent analytic reference")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, e := exec.CommandContext(ctx, path).Output()
	if e != nil {
		t.Fatal(e)
	}
	type report struct {
		Regular, Order, Operation, Solids int
		Valid                             bool
		Volume                            float64
		ComponentVolumes                  []float64
	}
	seen := map[[3]int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var q report
		if e = json.Unmarshal([]byte(line), &q); e != nil {
			t.Fatal(e)
		}
		key := [3]int{q.Regular, q.Order, q.Operation}
		if seen[key] || q.Regular < 0 || q.Regular > 1 || q.Order < 0 || q.Order > 1 || q.Operation < 0 || q.Operation > 2 {
			t.Fatal("bad reference row")
		}
		seen[key] = true
		a, b := pairedAffineShellFixture(t)
		if q.Regular == 1 {
			a, b = affineMaterialOnePairedFixture(t)
		}
		if q.Order == 1 {
			a, b = b, a
		}
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[q.Operation]
		result, e := CombineConvexAffineSplineShells(ctx, a, b, op, ConvexAffineSplineShellOptions{})
		common := 293. / 64
		stockA, stockB := 32., 6.
		if q.Regular == 1 {
			common = 139. / 64
			stockB = 3
		}
		want := common
		if q.Operation == 1 {
			want = stockA + stockB - common
		}
		if q.Operation == 2 {
			want = stockA - common
			if q.Order == 1 {
				want = stockB - common
			}
		}
		solids := 1
		if q.Regular == 0 && q.Order == 1 && q.Operation == 2 {
			solids = 2
		}
		if !q.Valid || q.Solids != solids || math.Abs(q.Volume-want) > 1e-10 {
			t.Fatalf("reference %+v want %.15g solids%d", q, want, solids)
		}
		if q.Regular == 0 && q.Operation == 2 {
			if e == nil || result != nil {
				t.Fatal("singular material accepted")
			}
			if q.Order == 1 {
				collection, err := CombineConvexAffineSplineShellComponents(ctx, a, b, op, ConvexAffineSplineShellCollectionOptions{})
				if err != nil {
					t.Fatal(err)
				}
				volumes := collection.ComponentVolumes()
				if len(volumes) != q.Solids || len(q.ComponentVolumes) != q.Solids {
					t.Fatal("independent component count")
				}
				for i, v := range volumes {
					x, _ := v.Float64()
					if math.Abs(x-q.ComponentVolumes[i]) > 1e-10 {
						t.Fatal("independent component volume")
					}
				}
			}
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		v, _ := result.ExactVolume().Float64()
		if math.Abs(v-q.Volume) > 1e-10 {
			t.Fatal("native/reference mismatch")
		}
	}
	if len(seen) != 12 {
		t.Fatal("incomplete independent reference")
	}
}
