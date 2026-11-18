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

func piecewisePolynomialVolume(order int, op parametric.BooleanOp) float64 {
	fs := [][]*big.Rat{{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}, {big.NewRat(3, 8), big.NewRat(1, 8), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}}
	I, Q := new(big.Rat), new(big.Rat)
	for _, f := range fs {
		for i, x := range f {
			I.Add(I, new(big.Rat).Quo(x, big.NewRat(int64(2*(i+1)), 1)))
			for j, y := range f {
				Q.Add(Q, new(big.Rat).Quo(new(big.Rat).Mul(x, y), big.NewRat(int64(4*(i+j+1)), 1)))
			}
		}
	}
	P := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 2), I), Q)
	r := new(big.Rat)
	switch op {
	case parametric.Union:
		r.Add(big.NewRat(769, 256), P)
	case parametric.Intersection:
		r.Sub(big.NewRat(769, 256), Q)
	default:
		if order == 0 {
			r.Set(P)
		} else {
			r.Set(Q)
		}
	}
	v, _ := r.Float64()
	return v
}
func TestPiecewisePolynomialOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_PIECEWISE_POLYNOMIAL")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
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
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-piecewisePolynomialVolume(r.Order, op)) > 3e-8 {
			t.Fatal(r)
		}
	}
}
