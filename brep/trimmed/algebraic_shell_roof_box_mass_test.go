package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func checkWeightedRoofColumnMass(t *testing.T, ctx context.Context, cut, common *MixedRoofMaterialQuery) {
	t.Helper()
	r := big.NewRat
	m, err := common.BoundMass(ctx, MixedRoofMassOptions{MaxCells: 3})
	if err != nil || m.VisitedCells != 1 || m.EnclosedCells != 0 || m.Volume.Lower.Cmp(r(27, 64)) != 0 || m.Volume.Upper.Cmp(r(27, 64)) != 0 {
		t.Fatal("exact weighted Common material column", m, err)
	}
	// For the tool, x,y range from 1/8 to 7/8; z ranges from 1/4
	// to 7/8+x/4. These moments follow by direct polynomial integration.
	centroid := [3]*big.Rat{r(33, 64), r(1, 2), r(321, 512)}
	for i, want := range centroid {
		if m.Centroid[i].Lower.Cmp(want) != 0 || m.Centroid[i].Upper.Cmp(want) != 0 {
			t.Fatal("weighted exact Common centroid", i, m.Centroid[i], want)
		}
	}
	for _, a := range []struct {
		i, j int
		want *big.Rat
	}{{0, 0, r(5103, 16384)}, {1, 1, r(5211, 16384)}, {2, 2, r(1053, 4096)},
		{0, 1, r(-891, 8192)}, {0, 2, r(-9099, 65536)}, {1, 2, r(-8667, 65536)}} {
		if v := m.Inertia[a.i][a.j]; v.Lower.Cmp(a.want) != 0 || v.Upper.Cmp(a.want) != 0 ||
			m.Inertia[a.j][a.i].Lower.Cmp(a.want) != 0 || m.Inertia[a.j][a.i].Upper.Cmp(a.want) != 0 {
			t.Fatal("weighted exact origin tensor", a.i, a.j, v, a.want)
		}
	}
	pose, err := placement.NewRationalRigid([3][4]*big.Rat{
		{r(0, 1), r(0, 1), r(-1, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(1, 1), r(0, 1), r(0, 1), r(3, 4)},
	}, 256)
	if err != nil {
		t.Fatal(err)
	}
	placed, err := common.Place(pose)
	if err != nil {
		t.Fatal(err)
	}
	world, err := placed.BoundMass(ctx, MixedRoofMassOptions{MaxCells: 3})
	if err != nil {
		t.Fatal("weighted Common exact occurrence mass", err)
	}
	for i, want := range [3]*big.Rat{r(-321, 512), r(1, 2), r(81, 64)} {
		if v := world.Centroid[i]; v.Lower.Cmp(want) != 0 || v.Upper.Cmp(want) != 0 {
			t.Fatal("placed exact centroid", i, v, want)
		}
	}
	// The moved Z=h+x moment is Qxx+2h*Mx+h²V. The XZ product is
	// -(Qxz+h*Mz), and inertia negates the off-diagonal product.
	for _, a := range []struct {
		v    geom.ExactParameterInterval
		want *big.Rat
	}{{world.SecondMoment[2][2], r(5697, 8192)}, {world.Inertia[0][2], r(44199, 131072)}} {
		if a.v.Lower.Cmp(a.want) != 0 || a.v.Upper.Cmp(a.want) != 0 {
			t.Fatal("placed exact world tensor", a.v, a.want)
		}
	}
	contains := func(a geom.ExactParameterInterval, x float64) bool {
		q := new(big.Rat).SetFloat64(x)
		return q != nil && a.Lower.Cmp(q) <= 0 && a.Upper.Cmp(q) >= 0
	}
	coarse, err := cut.BoundMass(ctx, MixedRoofMassOptions{MaxDepth: 4})
	if err != nil || coarse.EnclosedCells == 0 {
		t.Fatal("weighted cut needs conservative cells", coarse, err)
	}
	fine, err := cut.BoundMass(ctx, MixedRoofMassOptions{MaxDepth: 8})
	if err != nil || fine.EnclosedCells == 0 {
		t.Fatal("weighted cut refinement", fine, err)
	}
	width := func(v geom.ExactParameterInterval) *big.Rat { return new(big.Rat).Sub(v.Upper, v.Lower) }
	if width(fine.Volume).Cmp(width(coarse.Volume)) >= 0 || width(fine.SecondMoment[2][2]).Cmp(width(coarse.SecondMoment[2][2])) >= 0 {
		t.Fatal("refinement failed to tighten the weighted cavity", coarse.Volume, fine.Volume)
	}
	if program := os.Getenv("PLAINSOLID_OCCT_WEIGHTED_ROOF_MASS"); program != "" {
		data, err := exec.CommandContext(ctx, program, "weighted-mass").Output()
		if err != nil {
			t.Fatal(err)
		}
		var rows [3]struct {
			Name     string
			Volume   float64
			Centroid [3]float64
			Inertia  [3][3]float64
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		for i, name := range []string{"intersection", "union", "difference"} {
			if err := dec.Decode(&rows[i]); err != nil || rows[i].Name != name {
				t.Fatal("independent column mass values", rows[i], err)
			}
		}
		if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
			t.Fatal("trailing weighted mass data", err)
		}
		if !contains(fine.Volume, rows[2].Volume) {
			t.Fatal("OCCT weighted cavity volume misses bound", fine.Volume, rows[2].Volume)
		}
		for i := range 3 {
			if !contains(fine.Centroid[i], rows[2].Centroid[i]) {
				t.Fatal("OCCT weighted cavity centroid misses bound", i, fine.Centroid[i], rows[2].Centroid[i])
			}
			for j := range 3 {
				if !contains(fine.Inertia[i][j], rows[2].Inertia[i][j]) {
					t.Fatal("OCCT weighted cavity inertia misses bound", i, j, fine.Inertia[i][j], rows[2].Inertia[i][j])
				}
			}
		}
	}
	if partial, err := cut.BoundMass(ctx, MixedRoofMassOptions{MaxCells: 1}); !errors.Is(err, geom.ErrConvergence) || partial.Volume.Lower != nil {
		t.Fatal("mass cell budget published a partial result", partial, err)
	}
	if partial, err := common.BoundMass(ctx, MixedRoofMassOptions{MaxCells: 3,
		SurfacePatchEvaluationOptions: geom.SurfacePatchEvaluationOptions{MaxOperations: m.ExactOperations - 1}}); !errors.Is(err, geom.ErrConvergence) || partial.Volume.Lower != nil {
		t.Fatal("mass arithmetic budget published a partial result", partial, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := cut.BoundMass(cancelled, MixedRoofMassOptions{}); !errors.Is(err, context.Canceled) || partial.Volume.Lower != nil {
		t.Fatal("mass cancellation published a partial result", partial, err)
	}
}
