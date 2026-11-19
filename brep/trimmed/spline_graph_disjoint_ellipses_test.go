package trimmed

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"testing"
)

func TestDisjointEllipseOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_DISJOINT_ELLIPSE_ROOFS")
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
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 6 {
		t.Fatal("incomplete oracle")
	}
	lens := math.Pi / 4096
	vb := 3 + 1.0/256
	delta := 1.0/64 - 5.0/48
	seen := map[[2]int]bool{}
	for _, row := range rows {
		id := [2]int{row.Order, row.Operation}
		if seen[id] || row.Order < 0 || row.Order > 1 || row.Operation < 0 || row.Operation > 2 {
			t.Fatal("oracle identity")
		}
		seen[id] = true
		want := vb + delta - lens
		if row.Operation == 1 {
			want = vb + lens
		}
		if row.Operation == 2 {
			want = lens
			if row.Order == 1 {
				want = lens - delta
			}
		}
		solids := 1
		if row.Order == 0 && row.Operation == 2 {
			solids = 2
		}
		if !row.Valid || row.Solids != solids || math.Abs(row.Volume-want) > 2e-8 {
			t.Fatal("OCCT original Boolean", row, want)
		}
	}
}
