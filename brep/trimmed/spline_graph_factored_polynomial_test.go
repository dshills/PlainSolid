package trimmed

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"testing"
)

func factoredPolynomialVolume(order int, op parametric.BooleanOp) float64 {
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 1)}
	fg, square := new(big.Rat), new(big.Rat)
	for k, z := range g {
		for i, x := range f {
			fg.Add(fg, new(big.Rat).Quo(new(big.Rat).Mul(z, x), big.NewRat(int64(i+k+1), 1)))
			for j, y := range f {
				square.Add(square, new(big.Rat).Quo(new(big.Rat).Mul(z, new(big.Rat).Mul(x, y)), big.NewRat(int64(2*(i+j+k+1)), 1)))
			}
		}
	}
	positive := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(5, 4), fg), square)
	r := new(big.Rat)
	switch op {
	case parametric.Union:
		r.Add(big.NewRat(769, 256), positive)
	case parametric.Intersection:
		r.Sub(big.NewRat(769, 256), square)
	default:
		if order == 0 {
			r.Set(positive)
		} else {
			r.Set(square)
		}
	}
	v, _ := r.Float64()
	return v
}
func TestFactoredPolynomialOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_FACTORED_POLYNOMIAL")
	if exe == "" {
		t.Skip("optional independently authored original OCCT stocks")
	}
	data, e := exec.Command(exe).Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		Volume                   float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 6 {
		t.Fatal(e)
	}
	seen := map[[2]int]bool{}
	for _, r := range rows {
		k := [2]int{r.Order, r.Operation}
		if r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 2 || seen[k] {
			t.Fatal(r)
		}
		seen[k] = true
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-factoredPolynomialVolume(r.Order, op)) > 3e-8 {
			t.Fatal(r)
		}
	}
}
