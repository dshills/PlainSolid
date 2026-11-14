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

func TestOCCTTorusCylinderDistance(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_TORUS_DISTANCE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_TORUS_DISTANCE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "distance.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	seen := map[[2]int]bool{}
	r := big.NewRat
	queries := [3][3]*big.Rat{{r(4, 1), r(1, 1), r(3, 2)}, {r(0, 1), r(0, 1), r(0, 1)}, {r(6, 1), r(0, 1), r(2, 1)}}
	for {
		var ref struct {
			Operation, Query int
			SquaredDistance  float64
			Supports         [4]float64
		}
		e := dec.Decode(&ref)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		key := [2]int{ref.Operation, ref.Query}
		if ref.Operation < 0 || ref.Operation > 2 || ref.Query < 0 || ref.Query > 2 || seen[key] {
			t.Fatal("record")
		}
		seen[key] = true
		op := []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion}[ref.Operation]
		s, _ := geom.NewTorusCylinderMaterial(5, 2, 6, -3, 4, op)
		v, e := s.BoundBoundaryDistance(ctx, queries[ref.Query], geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 10000)})
		if e != nil {
			t.Fatal(ref, e)
		}
		check := func(x float64, lo, hi *big.Rat) {
			t.Helper()
			l, _ := lo.Float64()
			h, _ := hi.Float64()
			tol := 3e-7 * math.Max(1, x)
			if math.IsNaN(x) || x < l-tol || x > h+tol {
				t.Fatal("independent distance", ref, x, l, h)
			}
		}
		check(ref.SquaredDistance, v.Lower, v.Upper)
		for i, x := range ref.Supports {
			support := v.Supports[i]
			if x < 0 {
				if support.Owner != 0 {
					t.Fatal("absent owner")
				}
				continue
			}
			if support.Owner == 0 {
				t.Fatal("missing owner")
			}
			check(x, support.Lower, support.Upper)
		}
	}
	if len(seen) != 9 {
		t.Fatal("records", len(seen))
	}
}
