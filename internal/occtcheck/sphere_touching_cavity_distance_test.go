package occtcheck

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTSphereTouchingCavityNearest(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_TOUCHING_CAVITY")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_TOUCHING_CAVITY to pinned support oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "touching.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		Name            string
		SquaredDistance float64
		Point           [3]float64
		Solutions       int
	}
	refs := map[string]row{}
	dec := json.NewDecoder(f)
	for {
		var v row
		e := dec.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[v.Name]; ok {
			t.Fatal("duplicate")
		}
		refs[v.Name] = v
	}
	if len(refs) != 8 {
		t.Fatal("records", len(refs))
	}
	r := big.NewRat
	for _, tc := range []struct {
		name            string
		p               [3]*big.Rat
		distance        *big.Rat
		owners          geom.BoreBoundaryMask
		shared, contact bool
	}{
		{"sharedInside", [3]*big.Rat{r(9, 2), r(0, 1), r(0, 1)}, r(1, 4), 3, true, true},
		{"contact", [3]*big.Rat{r(5, 1), r(0, 1), r(0, 1)}, r(0, 1), 3, true, true},
		{"sharedOutside", [3]*big.Rat{r(6, 1), r(0, 1), r(0, 1)}, r(1, 1), 3, true, true},
		{"innerCenter", [3]*big.Rat{r(4, 1), r(0, 1), r(0, 1)}, r(1, 1), 3, false, true},
		{"distinctTie", [3]*big.Rat{r(20, 7), r(15, 7), r(0, 1)}, r(100, 49), 3, false, false},
		{"innerFoot", [3]*big.Rat{r(4, 1), r(1, 1), r(0, 1)}, r(0, 1), 2, false, false},
		{"outerCenter", [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, r(9, 1), 2, false, false},
		{"obliqueContact", [3]*big.Rat{r(53, 10), r(17, 5), r(3, 1)}, r(1, 4), 3, true, true},
	} {
		a, b, ra := geom.Vec3{}, geom.Vec3{X: 4}, 5.
		if tc.name == "obliqueContact" {
			a = geom.Vec3{X: 2, Y: -1, Z: 3}
			b = geom.Vec3{X: 5, Y: 3, Z: 3}
			ra = 6
		}
		s, e := geom.NewSpherePairMaterial(a, ra, b, 1, geom.SphereSetDifference)
		if e != nil {
			t.Fatal(e)
		}
		g, e := s.NearestBoundaryRational(ctx, tc.p, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Kind != geom.SphereNearestTouchingCavity || g.Owners != tc.owners || g.Touching.ContactNearest != tc.contact || g.Touching.CoincidentSingletons != tc.shared || g.Lower.Cmp(tc.distance) != 0 || g.Upper.Cmp(tc.distance) != 0 {
			t.Fatal(tc.name, g, e)
		}
		expected, _ := tc.distance.Float64()
		ref, ok := refs[tc.name]
		tol := 3e-7 * math.Max(1, expected)
		if !ok || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-expected) > tol {
			t.Fatal("independent support distance", tc.name, ref)
		}
		d := 0.
		for j, v := range ref.Point {
			q, _ := tc.p[j].Float64()
			d += math.Pow(v-q, 2)
		}
		if math.Abs(d-ref.SquaredDistance) > tol {
			t.Fatal("independent foot")
		}
		if tc.shared {
			for j, v := range ref.Point {
				exact, _ := g.Touching.Contact[j].Float64()
				if math.Abs(v-exact) > 1e-6 {
					t.Fatal("shared physical point", ref)
				}
			}
		}
		// The inner center has infinitely many minimizers: no OCCT count claim.
		if tc.name == "innerCenter" && !g.Touching.Boundary.Candidates[1].WholeSphere {
			t.Fatal("lost center family")
		}
	}
}
