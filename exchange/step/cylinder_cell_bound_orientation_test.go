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

func TestCylinderCellBoundOrientation(t *testing.T) {
	t.Parallel()
	for _, cells := range []int{2, 4} {
		prefix := "split"
		if cells == 4 {
			prefix = "quarter"
		}
		b, e := os.ReadFile("testdata/" + prefix + "-cylinder-reversed-bounds.step")
		if e != nil {
			t.Fatal(e)
		}
		m, e := readCylinderCells(bytes.NewReader(b), Limits{}, cells)
		if e != nil || len(m.Faces) != cells+2 {
			t.Fatal(cells, m, e)
		}
		for _, mode := range []string{"bound-only", "line-sense-only", "line-direction-only", "one-coedge"} {
			f, e := Parse(bytes.NewReader(b), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			face := f.Entities[m.Faces[0]].Records[0]
			boundID := face.Args[1].Items[0].Ref
			bound := f.Entities[boundID]
			switch mode {
			case "bound-only":
				bound.Records[0].Args[2].Text = "T"
				f.Entities[boundID] = bound
			case "line-sense-only":
				id := m.Edges[2*cells]
				ent := f.Entities[id]
				ent.Records[0].Args[4].Text = "T"
				f.Entities[id] = ent
			case "line-direction-only":
				ent := f.Entities[200]
				ent.Records[0].Args[1].Items[2].Text = "1."
				f.Entities[200] = ent
			case "one-coedge":
				loopID := bound.Records[0].Args[1].Ref
				useID := f.Entities[loopID].Records[0].Args[1].Items[0].Ref
				ent := f.Entities[useID]
				if ent.Records[0].Args[4].Text == "T" {
					ent.Records[0].Args[4].Text = "F"
				} else {
					ent.Records[0].Args[4].Text = "T"
				}
				f.Entities[useID] = ent
			}
			if got, e := readCylinderCells(bytes.NewReader(splitProductFile(f, nil, nil)), Limits{}, cells); e == nil || got.Body != 0 {
				t.Fatal("accepted", cells, mode)
			}
		}
	}
}
func TestCylinderCellBoundOrientationOCCT(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	for _, cells := range []int{2, 4} {
		name := "split"
		if cells == 4 {
			name = "quarter"
		}
		p, e := filepath.Abs("testdata/" + name + "-cylinder-reversed-bounds.step")
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
