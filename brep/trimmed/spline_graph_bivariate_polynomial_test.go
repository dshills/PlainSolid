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

func bivariatePolynomialVolume(order int, op parametric.BooleanOp) float64 {
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	neg, total := new(big.Rat), big.NewRat(13, 12)
	for i, x := range f {
		total.Sub(total, new(big.Rat).Mul(x, new(big.Rat).Add(big.NewRat(3, int64(2*(i+1))), big.NewRat(1, int64(i+2)))))
		for j, y := range f {
			xy := new(big.Rat).Mul(x, y)
			neg.Add(neg, new(big.Rat).Mul(xy, new(big.Rat).Add(big.NewRat(1, int64(2*(i+j+1))), big.NewRat(1, int64(2*(i+j+2))))))
			for k, z := range f {
				neg.Add(neg, new(big.Rat).Quo(new(big.Rat).Mul(xy, z), big.NewRat(int64(6*(i+j+k+1)), 1)))
			}
		}
	}
	positive := new(big.Rat).Add(total, neg)
	r := new(big.Rat)
	switch op {
	case parametric.Union:
		r.Add(big.NewRat(769, 256), positive)
	case parametric.Intersection:
		r.Sub(big.NewRat(769, 256), neg)
	default:
		if order == 0 {
			r.Set(positive)
		} else {
			r.Set(neg)
		}
	}
	v, _ := r.Float64()
	return v
}
func TestBivariatePolynomialOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_BIVARIATE_POLYNOMIAL")
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
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-bivariatePolynomialVolume(r.Order, op)) > 3e-8 {
			t.Fatal(r)
		}
	}
}
