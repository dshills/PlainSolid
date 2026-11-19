package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This source qualification intentionally has no prospective native constructor
// dependency. It compares independently constructed OCCT cuts to closed-form
// quarter-circle section measures, and retains strict legacy import rejection.
func TestUnequalCenterSideIndependentFixtures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"side", "through"} {
		raw, e := os.ReadFile("testdata/occt_unequal_center_dual_disk_" + mode + ".step")
		if e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil || len(f.Entities) != 582 {
			t.Fatal(e)
		}
		counts := map[string]int{}
		for _, en := range f.Entities {
			for _, r := range en.Records {
				counts[r.Type]++
			}
		}
		if counts["CYLINDRICAL_SURFACE"] != 2 || counts["ADVANCED_FACE"] != 9 {
			t.Fatal(counts)
		}
		if _, e = ReadOffsetSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("offset domain widened")
		}
		if _, e = ReadExactWallSideOpenDualDiskProduct(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
			t.Fatal("side-centered domain widened")
		}
	}
}
func TestOCCTUnequalCenterSideIndependentReference(t *testing.T) {
	t.Parallel()
	oracle := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if oracle == "" {
		t.Skip("optional independent OCCT helper")
	}
	for _, through := range []bool{false, true} {
		mode := "unequal-center-side"
		depth := 3.
		if through {
			mode = "unequal-center-through"
			depth = 6
		}
		path := filepath.Join(t.TempDir(), "reference.step")
		for _, m := range []string{mode, "read"} {
			raw, e := exec.Command(oracle, m, path).CombinedOutput()
			if e != nil {
				t.Fatalf("%v %s", e, raw)
			}
			idx := bytes.LastIndex(raw, []byte("{\"Faces\""))
			if idx < 0 {
				t.Fatal(string(raw))
			}
			var got struct {
				Faces, Planes, Cylinders int
				Volume, Area             float64
				Center                   [3]float64
			}
			if e = json.Unmarshal(bytes.TrimSpace(raw[idx:]), &got); e != nil {
				t.Fatal(e)
			}
			area := 216.5*math.Pi - 409
			wantV := 3600 - depth*area
			wantA := 2040 + depth*(17*math.Pi-48)
			if through {
				wantA -= 2 * area
			}
			if got.Faces != 9 || got.Planes != 7 || got.Cylinders != 2 || math.Abs(got.Volume-wantV) > 1e-7 || math.Abs(got.Area-wantA) > 1e-7 {
				t.Fatal(through, got, wantV, wantA)
			}
			t.Logf("%s V=%.15g A=%.15g center=%v", m, got.Volume, got.Area, got.Center)
		}
	}
}
