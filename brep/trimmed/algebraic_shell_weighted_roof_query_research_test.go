//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestOriginalWeightedRoofMaterialQueries(t *testing.T) {
	ctx := context.Background()
	a, b := weightedIdentityRoofStock(t, false, 2), weightedIdentityRoofStock(t, true, 2)
	// Test the complete original section and regularized result; a caller
	// cannot construct a query by asserting its own solid envelopes.
	sec, err := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if err != nil || !sec.Complete {
		t.Fatal("weighted original source section", sec.Coverage, err)
	}
	m, err := sec.CombineAlgebraic(ctx, parametric.Difference, AlgebraicShellBooleanOptions{})
	if err != nil {
		t.Fatal("weighted cavity", err)
	}
	q, err := m.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal("prepare weighted roof", err)
	}
	if q.box[0].minWeight == nil {
		t.Fatal("nonconstant original weight was discarded")
	}
	r := big.NewRat
	delta := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	p := [3]*big.Rat{r(1, 2), r(1, 2), new(big.Rat).Add(r(3, 2), delta)}
	if loc, err := q.LocateRational(ctx, p, geom.SurfacePatchEvaluationOptions{}); err != nil || loc.Location != parametric.Outside {
		t.Fatal("sub-binary weighted roof membership", loc, err)
	}
	// z=3/2+X/4 gives 3X²-7X+2=0, retaining X=1/3 inside
	// the footprint. The denominator remains strictly positive.
	line, err := q.IntersectLineRational(ctx, [3]*big.Rat{r(0, 1), r(1, 2), r(3, 2)}, [3]*big.Rat{r(1, 1), r(0, 1), r(1, 4)}, r(1, 8), r(7, 8), MixedRoofLineOptions{})
	if err != nil || len(line.Events) != 1 || len(line.Spans) != 1 || len(line.Events[0].Owners) != 1 || line.Events[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) {
		t.Fatal("weighted roof line", line, err)
	}
	k, err := line.Events[0].Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if err != nil || k.Kind != geom.ExactRootKindRational || k.Rational.Cmp(r(1, 3)) != 0 {
		t.Fatal(k, err)
	}
	if program := os.Getenv("PLAINSOLID_OCCT_WEIGHTED_ROOF"); program != "" {
		out, err := exec.CommandContext(ctx, program, "weighted-line").Output()
		var ref struct{ Parameter float64 }
		if err != nil {
			t.Fatal("independent weighted roof oracle", err)
		}
		want, _ := k.Rational.Float64()
		if err := json.Unmarshal(out, &ref); err != nil || !finite(ref.Parameter) || math.Abs(ref.Parameter-want) > 1e-10 {
			t.Fatal("weighted line disagrees with OCCT's original sewn surfaces", ref, want, err)
		}
	}
	p[2].SetFrac64(3, 2)
	n, err := q.BoundaryNormalsRational(ctx, p, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(n.Normals) != 1 || n.Normals[0].Direction[0].Cmp(r(1, 3)) != 0 {
		t.Fatal("weighted quotient derivative lost", n, err)
	}
	dist, err := q.BoundBoundaryDistanceRational(ctx, [3]*big.Rat{r(1, 2), r(1, 2), r(-1, 8)}, MixedRoofDistanceOptions{})
	if err != nil || !dist.Converged || dist.Squared.Lower.Cmp(r(1, 64)) != 0 || dist.Squared.Upper.Cmp(r(1, 64)) != 0 ||
		len(dist.Owners) != 1 || dist.Owners[0] != (MixedRoofFaceOwner{0, 0}) {
		t.Fatal("weighted stock distance lost the original floor witness", dist, err)
	}
	volume, err := q.BoundVolume(ctx, MixedRoofVolumeOptions{AbsoluteGap: r(1, 100)})
	// Integrating (2+x²)/(1+x) gives 3 ln(2)-1/2. The completely
	// enclosed sloped tool has area 9/16 and average thickness 3/4.
	want := new(big.Rat).SetFloat64(3*math.Log(2) - .5 - 27.0/64)
	if err != nil || !volume.Converged || volume.Volume.Lower.Cmp(want) > 0 || volume.Volume.Upper.Cmp(want) < 0 {
		t.Fatal("weighted cavity analytic volume", volume, want, err)
	}
	common, err := sec.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if err != nil {
		t.Fatal("weighted contained Common", err)
	}
	contained, err := common.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	proven, err := contained.BoundVolume(ctx, MixedRoofVolumeOptions{AbsoluteGap: r(1, 1000000000000), MaxCells: 3})
	if err != nil || !proven.Converged || proven.Volume.Lower.Cmp(r(27, 64)) != 0 || proven.Volume.Upper.Cmp(r(27, 64)) != 0 {
		t.Fatal("buried rational roof must leave an exact polynomial integral", proven, err)
	}
	checkWeightedRoofColumnMass(t, ctx, q, contained)
	checkWeightedRoofStrictSurfaceArea(t, ctx, q, contained)
	if program := os.Getenv("PLAINSOLID_OCCT_WEIGHTED_ROOF_MASS"); program != "" {
		out, err := exec.CommandContext(ctx, program, "weighted-mass").Output()
		if err != nil {
			t.Fatal("independent weighted mass oracle", err)
		}
		rows := json.NewDecoder(bytes.NewReader(out))
		for _, name := range []string{"intersection", "union", "difference"} {
			var ref struct {
				Name   string
				Volume float64
			}
			if err := rows.Decode(&ref); err != nil || ref.Name != name || !finite(ref.Volume) {
				t.Fatal("invalid weighted mass oracle result", ref, err)
			}
			bound := volume.Volume
			switch name {
			case "intersection":
				bound = proven.Volume
			case "union":
				fused, err := sec.CombineAlgebraic(ctx, parametric.Union, AlgebraicShellBooleanOptions{})
				if err != nil {
					t.Fatal(err)
				}
				fusedQuery, err := fused.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
				if err != nil {
					t.Fatal(err)
				}
				fusedVolume, err := fusedQuery.BoundVolume(ctx, MixedRoofVolumeOptions{})
				if err != nil || !fusedVolume.Converged {
					t.Fatal("weighted Fuse enclosure", fusedVolume, err)
				}
				bound = fusedVolume.Volume
			}
			value := new(big.Rat).SetFloat64(ref.Volume)
			if bound.Lower.Cmp(value) > 0 || bound.Upper.Cmp(value) < 0 {
				t.Fatal("weighted volume from separately sewn OCCT originals is outside bounds", name, value, bound)
			}
		}
		if err := rows.Decode(new(any)); !errors.Is(err, io.EOF) {
			t.Fatal("trailing weighted mass result", err)
		}
	}
	pose, err := placement.NewRationalRigid([3][4]*big.Rat{
		{r(0, 1), r(0, 1), r(-1, 1), r(7, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(8, 1)}, {r(1, 1), r(0, 1), r(0, 1), r(9, 1)},
	}, 256)
	if err != nil {
		t.Fatal(err)
	}
	placed, err := q.Place(pose)
	if err != nil {
		t.Fatal(err)
	}
	worldVolume, err := placed.BoundVolume(ctx, MixedRoofVolumeOptions{AbsoluteGap: r(1, 100)})
	if err != nil || !worldVolume.Converged || worldVolume.Volume.Lower.Cmp(volume.Volume.Lower) != 0 || worldVolume.Volume.Upper.Cmp(volume.Volume.Upper) != 0 {
		t.Fatal("proper rigid placement changed volume", worldVolume, volume, err)
	}
	coarse, err := q.BoundVolume(ctx, MixedRoofVolumeOptions{AbsoluteGap: r(1, 100000), MaxCells: 10})
	if !errors.Is(err, geom.ErrConvergence) || coarse.Volume.Lower == nil ||
		coarse.Volume.Lower.Cmp(volume.Volume.Lower) > 0 || coarse.Volume.Upper.Cmp(volume.Volume.Upper) < 0 {
		t.Fatal("limited volume must preserve a conservative enclosure", coarse, volume, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := q.BoundVolume(cancelled, MixedRoofVolumeOptions{}); !errors.Is(err, context.Canceled) || partial.Volume.Lower != nil {
		t.Fatal("volume cancellation exposed partial data", partial, err)
	}
	if _, err := proveMixedRoofBox(ctx, a, 1, 1000000); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("polynomial-only mesh proof accepted rational height", err)
	}
}
