package trimmed

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"testing"
)

func piecewiseParabolaVolume(order int, op parametric.BooleanOp) float64 {
	// Independent integration: int f=17/48, int f²/2=49/768.
	switch op {
	case parametric.Intersection:
		return 3 + 1.0/256 - 49.0/768
	case parametric.Union:
		return 3 + 1.0/256 + 161.0/768
	default:
		if order == 0 {
			return 161.0 / 768
		}
		return 49.0 / 768
	}
}
func TestNativePiecewiseParabolaOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_PIECEWISE_PARABOLA")
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
			t.Fatal("oracle identity", r)
		}
		seen[k] = true
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-piecewiseParabolaVolume(r.Order, op)) > 2e-8 {
			t.Fatal(r)
		}
	}
}
