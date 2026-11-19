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
	"testing"
)

func TestHierarchyClosestBoundary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-" + name
		}
		raw, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
		if e != nil {
			t.Fatal(e)
		}
		p := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1)}
		r, e := closestHierarchyBoundary(ctx, m, p, RationalHierarchyClosestOptions{}, vendor)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Owners) == 0 || r.SquaredDistance() == nil || r.Operations <= 0 {
			t.Fatal("missing complete minimum")
		}
		enclosure, e := r.SquaredDistance().Enclose(ctx, big.NewRat(1, 1000000000), geom.ExactCurvePlaneEventOptions{})
		if e != nil || !enclosure.Converged {
			t.Fatal(e)
		}
		if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE"); bin != "" {
			b, e := exec.Command(bin, "testdata/"+name, "0", "0", "0").CombinedOutput()
			if e != nil {
				t.Fatalf("%v %s", e, b)
			}
			var rows []struct{ BoundaryDistance float64 }
			if e = json.Unmarshal(b, &rows); e != nil {
				t.Fatal(e)
			}
			if len(rows) != len(m.Instances) {
				t.Fatal("oracle omitted occurrences")
			}
			want := math.Inf(1)
			for _, row := range rows {
				want = math.Min(want, row.BoundaryDistance*row.BoundaryDistance)
			}
			lo, _ := enclosure.Interval.Lower.Float64()
			hi, _ := enclosure.Interval.Upper.Float64()
			if want < lo-1e-7 || want > hi+1e-7 {
				t.Fatalf("oracle %g outside [%g,%g]", want, lo, hi)
			}
		}
		// Exact boundary of a retained planar occurrence gives a certified zero,
		// irrespective of other overlapping or disjoint retained boundaries.
		for _, u := range m.Instances {
			b := m.Products[u.ProductID]
			if b.Planar == nil {
				continue
			}
			p, e = u.ApplyExact(geom.Vec3{X: -10, Y: -4, Z: -2})
			if e != nil {
				t.Fatal(e)
			}
			z, e := closestHierarchyBoundary(ctx, m, p, RationalHierarchyClosestOptions{}, vendor)
			if e != nil {
				t.Fatal(e)
			}
			zero, _, _ := geom.NewExactRationalRoot(ctx, new(big.Rat), geom.ExactCurvePlaneEventOptions{})
			cmp, e := z.SquaredDistance().Compare(ctx, zero, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !cmp.Proven || cmp.Order != 0 {
				t.Fatal("boundary not zero", e)
			}
			found := false
			for _, w := range z.Owners {
				if w.Planar != nil && w.Planar.ProductID == u.ProductID {
					found = true
				}
			}
			if !found {
				t.Fatal("lost planar owner")
			}
			break
		}
		failed, e := closestHierarchyBoundary(ctx, m, p, RationalHierarchyClosestOptions{MaxOperations: 1}, vendor)
		if e == nil || failed.SquaredDistance() != nil || len(failed.Owners) != 0 {
			t.Fatal("partial quota result")
		}
		cancel, c := context.WithCancel(ctx)
		c()
		failed, e = closestHierarchyBoundary(cancel, m, p, RationalHierarchyClosestOptions{}, vendor)
		if e == nil || failed.SquaredDistance() != nil {
			t.Fatal("cancellation")
		}
	}
}

func TestHierarchyClosestExactCandidateTies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	o := geom.ExactCurvePlaneEventOptions{}
	rational, _, e := geom.NewExactRationalRoot(ctx, big.NewRat(1, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	iso, e := geom.IsolateExactPolynomialRoots(ctx, []*big.Rat{big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}, new(big.Rat), big.NewRat(2, 1), o)
	if e != nil || len(iso.Roots) != 1 {
		t.Fatal(e)
	}
	owners := []HierarchyClosestBoundaryOwner{{Planar: &HierarchyPlanarDistanceLeaf{ProductID: 1, Path: []int64{1}}}, {Sphere: &HierarchySphereDistanceLeaf{ProductID: 2, Path: []int64{2}}}, {Cylinder: &HierarchyCylinderDistanceLeaf{ProductID: 3, Path: []int64{3}}}, {Torus: &HierarchyTorusDistanceLeaf{ProductID: 4, Path: []int64{4}}}, {Torus: &HierarchyTorusDistanceLeaf{ProductID: 4, Path: []int64{5}}}}
	var r RationalHierarchyClosestBoundary
	for i, owner := range owners {
		root := rational
		if i%2 == 1 {
			root = iso.Roots[0].Root
		}
		if e = r.acceptClosest(ctx, root, owner, o); e != nil {
			t.Fatal(e)
		}
	}
	if len(r.Owners) != 5 {
		t.Fatal("lost equal family/repeated-use candidate")
	}
	smaller, _, _ := geom.NewExactRationalRoot(ctx, big.NewRat(1, 4), o)
	if e = r.acceptClosest(ctx, smaller, owners[0], o); e != nil || len(r.Owners) != 1 {
		t.Fatal("minimum replacement", e)
	}
	if e = r.acceptClosest(ctx, rational, owners[1], o); e != nil || len(r.Owners) != 1 {
		t.Fatal("larger candidate admitted", e)
	}
}
