package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCylinderCellPairedCurveSense(t *testing.T) {
	for _, cells := range []int{2, 4} {
		name := "split"
		if cells == 4 {
			name = "quarter"
		}
		b, e := os.ReadFile("testdata/" + name + "-cylinder-reversed-circles.step")
		if e != nil {
			t.Fatal(e)
		}
		m, e := readCylinderCells(bytes.NewReader(b), Limits{}, cells)
		if e != nil || m.Radius != 2 || m.Height != 3 || len(m.Edges) != 3*cells {
			t.Fatal(cells, m, e)
		}
		// Changing only the curve sense, only the circle normal, or a line sense
		// destroys the declared positive physical arc/axial direction contract.
		for _, mode := range []string{"curve-only", "normal-only", "line", "reference"} {
			f, e := Parse(bytes.NewReader(b), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "curve-only":
				id := m.Edges[0]
				ent := f.Entities[id]
				ent.Records[0].Args[4].Text = "T"
				f.Entities[id] = ent
			case "normal-only":
				ent := f.Entities[200]
				ent.Records[0].Args[1].Items[2].Text = "1."
				f.Entities[200] = ent
			case "line":
				id := m.Edges[2*cells]
				ent := f.Entities[id]
				ent.Records[0].Args[4].Text = "F"
				f.Entities[id] = ent
			case "reference":
				ent := f.Entities[201]
				ent.Records[0].Args[3].Ref = 200
				f.Entities[201] = ent
			}
			if got, e := readCylinderCells(bytes.NewReader(splitProductFile(f, nil, nil)), Limits{}, cells); e == nil || got.Body != 0 {
				t.Fatal("accepted", cells, mode)
			}
		}
	}
}
func TestCylinderCellPairedCurveSenseOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	for _, cells := range []int{2, 4} {
		name := "split"
		if cells == 4 {
			name = "quarter"
		}
		p, e := filepath.Abs("testdata/" + name + "-cylinder-reversed-circles.step")
		if e != nil {
			t.Fatal(e)
		}
		args := []string{p}
		if cells == 4 {
			args = append(args, "quarter")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		raw, e := exec.CommandContext(ctx, bin, args...).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		var v struct {
			Faces        int
			Volume, Area float64
			Center       [3]float64
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e, string(raw))
		}
		if v.Faces != cells+2 || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
			t.Fatal(v)
		}
		for i, w := range []float64{0, 0, 1.5} {
			if math.Abs(v.Center[i]-w) > 1e-10 {
				t.Fatal(v)
			}
		}
	}
}
