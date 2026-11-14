package trimmed

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
)

func TestCoincidentRoofBoxSurfaceArea(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{2, 2, 2}, 0)
	sameSide, _ := makeOpenSplineBox(t, [3]float64{1, 0, 0}, [3]float64{3, 2, 2}, 0)
	opposite, _ := makeOpenSplineBox(t, [3]float64{2, 1, 1}, [3]float64{4, 3, 3}, 0)
	identical := a
	want := [3][3]int64{{16, 32, 16}, {0, 46, 24}, {24, 24, 0}}
	if oracle := os.Getenv("PLAINSOLID_OCCT_COINCIDENT_ROOF_AREA"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle).Output()
		var sum [3][3]float64
		if err != nil || json.Unmarshal(data, &sum) != nil {
			t.Fatalf("independent box area: %v %s", err, data)
		}
		for i, row := range sum {
			for j, got := range row {
				delta := got - float64(want[i][j])
				if delta < -1e-8 || delta > 1e-8 {
					t.Fatal("independent coplanar regularization changed", i, j, got)
				}
			}
		}
	}
	for i, b := range []Solid{sameSide, opposite, identical} {
		for j, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			out, err := BoundRoofBoxSurfaceArea(ctx, a, b, op, MixedRoofQueryOptions{}, MixedRoofAreaOptions{AbsoluteGap: 1e-7, MaxCells: 96})
			if err != nil || out.Area.Lower.Cmp(big.NewRat(want[i][j], 1)) != 0 || out.Area.Upper.Cmp(out.Area.Lower) != 0 {
				t.Fatal("coincident face policy must give an exact physical area", i, op, out, err)
			}
			// The first cube has source floor 4 and roof 5. The same-side
			// difference must expose a reversed internal cutter wall.
			if i == 0 && j == 2 {
				if out.Faces[4].Area.Lower.Cmp(big.NewRat(2, 1)) != 0 || out.Faces[5].Area.Lower.Cmp(big.NewRat(2, 1)) != 0 ||
					!out.Faces[6].Reversed || out.Faces[6].Area.Lower.Cmp(big.NewRat(4, 1)) != 0 {
					t.Fatal("wrong source face ownership or cutter reversal", out.Faces)
				}
			}
			if i == 2 || i == 1 && j == 2 {
				for _, f := range out.Faces[6:] {
					if f.Retained {
						t.Fatal("canceled/shared face assigned to the second body", f)
					}
				}
			}
		}
	}
	weighted := weightedIdentityRoofStock(t, false, 2)
	out, err := BoundRoofBoxSurfaceArea(ctx, weighted, weighted, parametric.Union, MixedRoofQueryOptions{}, MixedRoofAreaOptions{AbsoluteGap: .15})
	// Independent OCCT stock area is the weighted cavity area minus the
	// entirely enclosed tool's area. The union must count the roof once.
	ref := new(big.Rat).SetFloat64(12.284286477470193 - 3.3923117286024835)
	if err != nil || out.Area.Lower.Cmp(ref) > 0 || out.Area.Upper.Cmp(ref) < 0 || !out.Faces[1].Retained || out.Faces[7].Retained {
		t.Fatal("coincident weighted roof should contribute once", out, err)
	}
}
