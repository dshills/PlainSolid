package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCoincidentRoofBoxMaterialMoments(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{2, 2, 2}, 0)
	share, _ := makeOpenSplineBox(t, [3]float64{1, 0, 0}, [3]float64{3, 2, 2}, 0)
	adjacent, _ := makeOpenSplineBox(t, [3]float64{2, 1, 1}, [3]float64{4, 3, 3}, 0)
	live := false
	var refs [3][3]struct {
		Volume   float64
		Centroid [3]float64
		Inertia  [3][3]float64
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_COINCIDENT_ROOF_AREA"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "mass").Output()
		if err != nil || json.Unmarshal(data, &refs) != nil {
			t.Fatalf("independent coincident mass: %v %s", err, data)
		}
		live = true
	}
	r := big.NewRat
	want := [3][3]int64{{4, 12, 4}, {0, 16, 8}, {8, 8, 0}}
	exact := func(v geom.ExactParameterInterval, q *big.Rat) bool {
		return v.Lower != nil && v.Lower.Cmp(q) == 0 && v.Upper.Cmp(q) == 0
	}
	for i, b := range []Solid{share, adjacent, a} {
		for j, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			v, err := BoundRoofBoxVolume(ctx, a, b, op, MixedRoofQueryOptions{}, MixedRoofVolumeOptions{MaxCells: 16})
			if err != nil || !v.Converged || !exact(v.Volume, r(want[i][j], 1)) {
				t.Fatal("direct exact shared-boundary volume", i, op, v, err)
			}
			m, err := BoundRoofBoxMass(ctx, a, b, op, MixedRoofQueryOptions{}, MixedRoofMassOptions{MaxCells: 16})
			if err != nil || m.EnclosedCells != 0 || !exact(m.Volume, r(want[i][j], 1)) {
				t.Fatal("direct exact shared-boundary material moments", i, op, m, err)
			}
			if want[i][j] == 0 {
				for k := range 3 {
					if m.Centroid[k].Lower != nil || !exact(m.FirstMoment[k], r(0, 1)) || !exact(m.Inertia[k][k], r(0, 1)) {
						t.Fatal("zero volume has no centroid or mass moment", m)
					}
				}
			}
			if i == 0 && j == 0 {
				if !exact(m.Centroid[0], r(3, 2)) || !exact(m.Inertia[0][0], r(32, 3)) || !exact(m.Inertia[0][1], r(-6, 1)) {
					t.Fatal("analytic shared-face overlap tensor", m)
				}
			}
			if live && want[i][j] > 0 {
				ref := refs[i][j]
				if math.Abs(ref.Volume-float64(want[i][j])) > 1e-9 {
					t.Fatal("OCCT volume differs", ref)
				}
				for k := range 3 {
					c, _ := m.Centroid[k].Lower.Float64()
					if math.Abs(c-ref.Centroid[k]) > 1e-9 {
						t.Fatal("OCCT centroid differs", i, j, k, c, ref)
					}
					for l := range 3 {
						h, _ := m.Inertia[k][l].Lower.Float64()
						if math.Abs(h-ref.Inertia[k][l]) > 1e-8 {
							t.Fatal("OCCT world inertia differs", i, j, k, l, h, ref)
						}
					}
				}
			}
		}
	}
	if m, err := BoundRoofBoxMass(ctx, a, share, parametric.Union, MixedRoofQueryOptions{MaxPreparationOperations: 1}, MixedRoofMassOptions{}); !errors.Is(err, geom.ErrConvergence) || m.Volume.Lower != nil {
		t.Fatal("source preparation exhaustion leaked a result", m, err)
	}
}
