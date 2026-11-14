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

func TestOCCTSpherePairRationalLine(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_LINE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_LINE to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "line.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		Name       string
		Parameters []float64
		MidInside  bool
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
	if len(refs) != 6 {
		t.Fatal("records", len(refs))
	}
	r := big.NewRat
	origin := [3]*big.Rat{r(1, 3), r(1, 3), r(1, 6)}
	for _, tc := range []struct {
		name   string
		op     geom.SphereSetOperation
		lo, hi float64
		lm, hm geom.BoreBoundaryMask
	}{{"union", geom.SphereSetUnion, -16.5, 22.5, 1, 2}, {"common", geom.SphereSetIntersection, -7.5, 13.5, 2, 1}, {"cut", geom.SphereSetDifference, -16.5, -7.5, 1, 2}} {
		s, _ := geom.NewSpherePairMaterial(geom.Vec3{}, 5, geom.Vec3{X: 2, Y: 2, Z: 1}, 5, tc.op)
		for _, sign := range []int64{1, -1} {
			name := tc.name + "Forward"
			lo, hi, lm, hm := tc.lo, tc.hi, tc.lm, tc.hm
			if sign < 0 {
				name = tc.name + "Reverse"
				lo, hi, lm, hm = -hi, -lo, hm, lm
			}
			ref, ok := refs[name]
			if !ok || len(ref.Parameters) != 2 || !ref.MidInside {
				t.Fatal("independent section", ref)
			}
			line, e := s.ClipRationalLine(ctx, origin, [3]*big.Rat{r(2*sign, 9), r(2*sign, 9), r(sign, 9)}, r(-30, 1), r(30, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
			if e != nil || len(line.Intervals) != 1 || len(line.Points) != 0 || line.Intervals[0].StartBoundary != lm || line.Intervals[0].EndBoundary != hm {
				t.Fatal(name, line, e)
			}
			for i, root := range []*geom.ExactAlgebraicRoot{line.Intervals[0].Start, line.Intervals[0].End} {
				expected := lo
				if i == 1 {
					expected = hi
				}
				if math.Abs(ref.Parameters[i]-expected) > 1e-7 {
					t.Fatal("independent raw t", name, ref)
				}
				bounds := root.Interval()
				a, _ := bounds.Lower.Float64()
				b, _ := bounds.Upper.Float64()
				if ref.Parameters[i] < a-1e-7 || ref.Parameters[i] > b+1e-7 {
					t.Fatal("exact endpoint vs oracle", name, a, b, ref)
				}
			}
		}
	}
}
