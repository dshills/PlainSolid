package trimmed

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"testing"
)

func threeEllipseVolume(order int, op parametric.BooleanOp) float64 {
	lens := 3 * math.Pi / 2097152
	delta := 1.0/1024 - 5.0/48
	switch op {
	case parametric.Union:
		return 3 + 1.0/256 + lens
	case parametric.Difference:
		if order == 0 {
			return lens
		}
		return lens - delta
	default:
		return 3 + 1.0/256 + delta - lens
	}
}
func TestThreeEllipseOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_THREE_ELLIPSE_ROOFS")
	if exe == "" {
		t.Skip("optional original OCCT stocks")
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
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 6 {
		t.Fatal("incomplete oracle")
	}
	seen := map[[2]int]bool{}
	for _, r := range rows {
		key := [2]int{r.Order, r.Operation}
		if seen[key] || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 2 {
			t.Fatal("oracle identity")
		}
		seen[key] = true
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
		solids := 1
		if r.Order == 0 && op == parametric.Difference {
			solids = 3
		}
		if !r.Valid || r.Solids != solids || math.Abs(r.Volume-threeEllipseVolume(r.Order, op)) > 2e-8 {
			t.Fatal(r, threeEllipseVolume(r.Order, op))
		}
	}
}
