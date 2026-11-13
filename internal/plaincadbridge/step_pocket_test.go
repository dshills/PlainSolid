package plaincadbridge

import (
	"math"
	"os"
	"testing"
)

func TestSTEPImportedBorePocketRoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/plaincad-placed-bore.step")
	if err != nil {
		t.Fatal(err)
	}
	b := New()
	stock, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	near(t, stock.GeometryAssertions.Volume, 2000-40*math.Pi)
	tool := box(t, b, vector{27, -15, 15}, vector{29, -13, 19})
	cut := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	near(t, cut.GeometryAssertions.Volume, 2000-40*math.Pi-16)
	count := len(b.shapes)
	output, err := b.exportSTEP([]string{cut.Handle})
	if err != nil {
		t.Fatal(err)
	}
	if len(output.After) != 1 || len(b.shapes) != count {
		t.Fatal("export ownership changed")
	}
	near(t, output.After[0].Volume, 2000-40*math.Pi-16)
	near(t, output.After[0].SurfaceArea, cut.GeometryAssertions.SurfaceArea)
	for i := 0; i < 3; i++ {
		near(t, output.After[0].Bounds.Min[i], cut.Bounds.Min[i])
		near(t, output.After[0].Bounds.Max[i], cut.Bounds.Max[i])
	}

}
