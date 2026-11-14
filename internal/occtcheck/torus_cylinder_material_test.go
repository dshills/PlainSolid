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

func TestOCCTTorusCylinderMaterial(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_TORUS_CYLINDER")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_TORUS_CYLINDER to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "torus.jsonl")
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
	for {
		var ref struct {
			Difference bool
			Sign       int
			Parameters [4]float64
		}
		e := dec.Decode(&ref)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		op := geom.TorusCylinderCommon
		k := 0
		if ref.Difference {
			op = geom.TorusCylinderDifference
			k = 1
		}
		key := [2]int{k, ref.Sign}
		if seen[key] || (ref.Sign != 1 && ref.Sign != -1) {
			t.Fatal("record")
		}
		seen[key] = true
		s, e := geom.NewTorusCylinderMaterial(5, 2, 6, -3, 4, op)
		if e != nil {
			t.Fatal(e)
		}
		line, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(int64(ref.Sign), 1), r(0, 1), r(int64(ref.Sign), 10)}, r(-8, 1), r(8, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
		if e != nil || len(line.Intervals) != 2 || len(line.Points) != 0 {
			t.Fatal(line, e)
		}
		masks := [4]geom.BoreBoundaryMask{2, 1, 1, 2}
		if ref.Difference {
			masks = [4]geom.BoreBoundaryMask{1, 2, 2, 1}
		}
		for i, v := range line.Intervals {
			if v.StartBoundary != masks[2*i] || v.EndBoundary != masks[2*i+1] {
				t.Fatal("owner", v)
			}
			for j, root := range []*geom.ExactAlgebraicRoot{v.Start, v.End} {
				b := root.Interval()
				lo, _ := b.Lower.Float64()
				hi, _ := b.Upper.Float64()
				x := ref.Parameters[2*i+j]
				if math.IsNaN(x) || x < lo-1e-7 || x > hi+1e-7 {
					t.Fatal("independent raw root", ref, lo, hi)
				}
			}
		}
	}
	if len(seen) != 4 {
		t.Fatal("records", len(seen))
	}
}
