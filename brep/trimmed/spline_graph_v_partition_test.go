package trimmed

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"testing"
)

// Independently author vertical loops: transpose the tensor basis AND exchange
// represented physical X/Y poles, preserving the identity XY source chart.
// Runtime discovery never changes these authored supports or owner coordinates.
func verticalGraphVolume(n, order int, op parametric.BooleanOp) float64 {
	if n == 3 {
		return threeEllipseVolume(order, op)
	}
	lens := math.Pi / 4096
	delta := 1.0/64 - 5.0/48
	if op == parametric.Union {
		return 3 + 1.0/256 + lens
	}
	if op == parametric.Difference {
		if order == 0 {
			return lens
		}
		return lens - delta
	}
	return 3 + 1.0/256 + delta - lens
}
func TestOriginalVPartitionOCCT(t *testing.T) {
	for _, n := range []int{2, 3} {
		key := "PLAINSOLID_OCCT_DISJOINT_ELLIPSE_ROOFS"
		if n == 3 {
			key = "PLAINSOLID_OCCT_THREE_ELLIPSE_ROOFS"
		}
		exe := os.Getenv(key)
		if exe == "" {
			t.Skip("optional independently authored vertical OCCT stocks")
		}
		data, e := exec.Command(exe, "vertical").Output()
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Order, Operation, Solids int
			Valid, Vertical          bool
			Volume                   float64
		}
		if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 6 {
			t.Fatal(e)
		}
		seen := map[[2]int]bool{}
		for _, r := range rows {
			k := [2]int{r.Order, r.Operation}
			if seen[k] || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 2 {
				t.Fatal("oracle identity")
			}
			seen[k] = true
			op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
			solids := 1
			if op == parametric.Difference && r.Order == 0 {
				solids = n
			}
			if !r.Valid || !r.Vertical || r.Solids != solids || math.Abs(r.Volume-verticalGraphVolume(n, r.Order, op)) > 2e-8 {
				t.Fatal(n, r)
			}
		}
	}
}
