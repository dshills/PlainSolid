package trimmed

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func nativePolynomialNodeFixture(t *testing.T, axis int, rootDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// The cubic f and quartic g have g-f=(1-rootDenominator*u)(1+u^3)/64.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(17, 64), big.NewRat(24-rootDenominator, 64), big.NewRat(-3, 8), big.NewRat(9, 64), big.NewRat(-rootDenominator, 64)}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func nativePolynomialNodeVolume(op parametric.BooleanOp) float64 {
	// Integration of the original quadratic height gap in the dependent
	// coordinate gives (105/128)*|g-f|^3/6 between the branches. Split that
	// independent univariate integral at 1/2 to sign the cubic.
	if op == parametric.Union {
		return 425143100365.0 / 137438953472
	}
	return 412853673011.0 / 137438953472
}

func TestNativePolynomialNodeOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_POLYNOMIAL_NODE")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	data, e := exec.Command(exe, "node").Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		Volume                   float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 4 {
		t.Fatal(string(data), e)
	}
	var seen [2][2]bool
	for _, r := range rows {
		if r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 1 || seen[r.Order][r.Operation] {
			t.Fatal(r)
		}
		seen[r.Order][r.Operation] = true
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Union}[r.Operation]
		want := nativePolynomialNodeVolume(op)
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-want) > 1e-9 {
			t.Fatal(r, want)
		}
	}
}
