package trimmed

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"testing"
)

func TestConvexAffinePolygonLinkCutOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_AFFINE_POLYGON_PAIRED")
	if exe == "" {
		t.Skip("optional independent OCCT reference")
	}
	data, e := exec.Command(exe, "full").Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids, PinchVertices int
		Valid                                   bool
		Volume                                  float64
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 6 {
		t.Fatal("incomplete oracle")
	}
	found := false
	for _, row := range rows {
		if row.Order == 0 && row.Operation == 2 {
			found = true
			if !row.Valid || row.Solids != 1 || row.PinchVertices != 1 || math.Abs(row.Volume-1747.0/64) > 1e-10 {
				t.Fatal("OCCT physical set/identity", row)
			}
		}
	}
	if !found {
		t.Fatal("missing forward cut")
	}
}
