package trimmed

import (
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
	"plainsolid/geom"
)

func TestMixedOriginalSourceMassAndArea(t *testing.T) {
	ctx := context.Background()
	a, b, p, options := mixedWallFixture(t)
	s, err := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, options)
	if err != nil {
		t.Fatal(err)
	}
	o := AlgebraicShellBooleanOptions{}
	o.Imprint.Exact.MaxOperations = 128000000
	bodies, err := s.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}, o)
	if err != nil {
		t.Fatal(err)
	}
	type reference struct {
		Name          string
		Area, Volume  float64
		Centroid      [3]float64
		Faces, Solids int
		Euler         int
	}
	refs := map[string]reference{
		"intersection": {"intersection", 3.992955181787411, .4843585758058463, [3]float64{.5013441310737613, .501344131991718, -.2572354549737378}, 8, 1, 2},
		"union":        {"union", 8.487908279091027, 1.4287883967133232, [3]float64{.5010038760434279, .5010038758984265, -.456827455506804}, 17, 1, 2},
		"difference":   {"difference", 8.057158869984429, .7705242365509349, [3]float64{.500805327529843, .5002112334163255, -.6494104133380204}, 14, 2, 4},
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_CROSSING_ROOFS"); oracle != "" {
		cmd := exec.CommandContext(ctx, oracle)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stderr = os.Stderr
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(stdout)
		seen := map[string]bool{}
		for {
			var v reference
			if err = decoder.Decode(&v); errors.Is(err, io.EOF) {
				break
			}
			if err != nil || seen[v.Name] || !finite(v.Area) || !finite(v.Volume) {
				t.Fatal("invalid independent OCCT record", v, err)
			}
			seen[v.Name] = true
			if baseline, ok := refs[v.Name]; ok {
				if v.Faces != baseline.Faces || v.Solids != baseline.Solids || v.Euler != baseline.Euler ||
					math.Abs(v.Volume-baseline.Volume) > 1e-8 || math.Abs(v.Area-baseline.Area) > 1e-7 {
					t.Fatal("independent OCCT repeat changed", v, baseline)
				}
				refs[v.Name] = v
			}
		}
		if err = cmd.Wait(); err != nil || len(seen) != 5 {
			t.Fatal("incomplete independent OCCT report", seen, err)
		}
		for name := range refs {
			if !seen[name] {
				t.Fatal("missing original Boolean result", name)
			}
		}
	}

	contains := func(v geom.ExactParameterInterval, want float64) bool {
		q := new(big.Rat).SetFloat64(want)
		return v.Lower != nil && v.Upper != nil && q != nil && v.Lower.Cmp(q) <= 0 && v.Upper.Cmp(q) >= 0
	}
	for i, name := range []string{"intersection", "union", "difference"} {
		ref := refs[name]
		body := bodies[i]
		top, err := body.Topology()
		if err != nil || len(body.Faces()) != ref.Faces || top.EulerCharacteristic != ref.Euler {
			t.Fatal("original topology differs from independently sewn OCCT", name, top, err)
		}
		selection := AlgebraicShellMeasureOptions{MaxDepth: 5, TubeSubdivisions: 8}
		area, err := body.BoundSurfaceArea(ctx, selection)
		if err != nil || !contains(area.Area, ref.Area) || area.Area.Lower.Sign() <= 0 {
			t.Fatal("OCCT area outside native original-support interval", name, area, err)
		}
		if i == 0 {
			trim, err := body.BoundTrimmedGeometry(ctx, selection)
			if err != nil || trim.Empty || trim.VisitedCells == 0 {
				t.Fatal("retained trim bounds must complete", trim, err)
			}
			box, _, err := body.boundary.Bounds()
			if err != nil || !box.Contains(trim.Box.Min) || !box.Contains(trim.Box.Max) {
				t.Fatal("trim refinement may never widen the conservative operand box", box, trim, err)
			}
			for _, p := range []geom.Vec3{{X: .1, Y: .5, Z: -.25}, {X: .9, Y: .5, Z: -.25}} {
				if where, err := body.LocateMaterial(ctx, p, SplineShellBooleanOptions{}); err != nil || where != parametric.Inside || !trim.Box.Contains(p) {
					t.Fatal("known retained point omitted from original hull", p, where, trim, err)
				}
			}
			if small, err := body.BoundTrimmedGeometry(ctx, AlgebraicShellMeasureOptions{MaxCells: 1}); !errors.Is(err, geom.ErrConvergence) || small.VisitedCells != 0 {
				t.Fatal("partial trim enclosure escaped an exhausted budget", small, err)
			}
			mass, err := body.BoundMassProperties(ctx, AlgebraicShellMeasureOptions{MaxDepth: 6, TubeSubdivisions: 8})
			if err != nil || !contains(mass.Volume, ref.Volume) || mass.Volume.Lower.Sign() <= 0 {
				t.Fatal("OCCT volume outside native mass interval", name, mass, err)
			}
			for j := range mass.Centroid {
				if !contains(mass.Centroid[j], ref.Centroid[j]) {
					t.Fatal("OCCT centroid outside native original-support interval", j, ref.Centroid[j], mass.Centroid[j])
				}
			}
			if mass.Inertia[0][1].Lower.Cmp(mass.Inertia[1][0].Lower) != 0 {
				t.Fatal("native inertia bounds must be symmetric", mass.Inertia)
			}
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := bodies[0].BoundMassProperties(cancelled, AlgebraicShellMeasureOptions{}); !errors.Is(err, context.Canceled) || partial.Volume.Lower != nil {
		t.Fatal("cancellation may not expose partial material measure", partial, err)
	}
}
