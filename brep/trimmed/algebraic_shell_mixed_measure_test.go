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
		Inertia       [3][3]float64
		Faces, Solids int
		Euler         int
	}
	refs := map[string]reference{
		"intersection": {"intersection", 3.992955181787411, .4843585758058463, [3]float64{.5013441310737613, .501344131991718, -.2572354549737378},
			[3][3]float64{{.20371521469941042, -.1218491924154663, .062295538924379001}, {-.1218491924154663, .2051525087128116, .062306767791234563}, {.062295538924379001, .062306767791234563, .32555985558140593}}, 8, 1, 2},
		"union": {"union", 8.487908279091027, 1.4287883967133232, [3]float64{.5010038760434279, .5010038758984265, -.456827455506804},
			[3][3]float64{{.93573132257433334, -.35894677387277751, .32627225601126225}, {-.35894677387277751, .9694698816565811, .32630695666364889}, {.32627225601126225, .32630695666364889, 1.0504004945985486}}, 17, 1, 2},
		"difference": {"difference", 8.057158869984429, .7705242365509349, [3]float64{.500805327529843, .5002112334163255, -.6494104133380204},
			[3][3]float64{{.63205965232888095, -.19312612228535442, .2501881691341945}, {-.19312612228535442, .68990275364416731, .25017872840440297}, {.2501881691341945, .25017872840440297, .57193668591631075}}, 14, 2, 4},
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
				for i, want := range baseline.Centroid {
					if !finite(v.Centroid[i]) || math.Abs(v.Centroid[i]-want) > 1e-8 {
						t.Fatal("independent OCCT centroid repeat changed", v, baseline)
					}
					for j, want := range baseline.Inertia[i] {
						if !finite(v.Inertia[i][j]) || math.Abs(v.Inertia[i][j]-want) > 1e-7 {
							t.Fatal("independent OCCT inertia repeat changed", v, baseline)
						}
					}
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
		area, err := body.BoundOriginalFaceAreas(ctx, selection)
		if err != nil || !contains(area.Area, ref.Area) || area.Area.Lower.Sign() <= 0 {
			t.Fatal("OCCT area outside native original-support interval", name, area, err)
		}
		if len(area.Faces) != 12 {
			t.Fatal("original source ownership missing from area ledger", name, area.Faces)
		}
		sum := zeroAreaInterval()
		ambiguous := 0
		for j, row := range area.Faces {
			if row.Operand != j/6 || row.OriginalFace != j%6 || row.AmbiguousCells > row.SelectedCells {
				t.Fatal("invalid source area owner or work counts", name, j, row)
			}
			ambiguous += row.AmbiguousCells
			sum.Lower.Add(sum.Lower, row.Area.Lower)
			sum.Upper.Add(sum.Upper, row.Area.Upper)
		}
		if ambiguous != area.AmbiguousCells || sum.Lower.Cmp(area.Area.Lower) != 0 || sum.Upper.Cmp(area.Area.Upper) != 0 {
			t.Fatal("per-face contributions must exhaust the aggregate", name, sum, area)
		}
		if i == 0 {
			// The original stock floor is below the tool's full z extent;
			// the tool floor contributes precisely the overlapped unit square.
			if area.Faces[0].Area.Upper.Sign() != 0 || !contains(area.Faces[6].Area, 1) || area.Faces[6].Area.Lower.Sign() <= 0 {
				t.Fatal("incorrect physical attribution of the two original floors", area.Faces[0], area.Faces[6])
			}
			if partial, err := body.BoundOriginalFaceAreas(ctx, AlgebraicShellMeasureOptions{MaxCells: 1}); !errors.Is(err, geom.ErrConvergence) || partial.Faces != nil || partial.Area.Lower != nil {
				t.Fatal("cell exhaustion must publish no original area ledger", partial, err)
			}
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
		}
		mass, err := body.BoundMassProperties(ctx, AlgebraicShellMeasureOptions{MaxDepth: 6, TubeSubdivisions: 8})
		if err != nil || !contains(mass.Volume, ref.Volume) || mass.Volume.Lower.Sign() <= 0 {
			t.Fatal("OCCT volume outside native mass interval", name, mass.Volume, err)
		}
		for j := range mass.Centroid {
			if !contains(mass.Centroid[j], ref.Centroid[j]) || !contains(mass.FirstMoment[j], ref.Volume*ref.Centroid[j]) {
				t.Fatal("OCCT centroid or first moment outside native interval", name, j, ref.Centroid[j])
			}
			for k := range mass.Inertia[j] {
				a, b := mass.Inertia[j][k], mass.Inertia[k][j]
				if !contains(a, ref.Inertia[j][k]) || a.Lower.Cmp(b.Lower) != 0 || a.Upper.Cmp(b.Upper) != 0 {
					t.Fatal("OCCT world inertia outside native symmetric interval", name, j, k, ref.Inertia[j][k], a)
				}
			}
		}
	}
	// On the exact center line the tool roof is below the stock roof.
	// z=-1,-1/2,-7/128,1/256 are the four original boundary stations.
	line := SplineRayOptions{}
	line.Line.MinParameter, line.Line.MaxParameter = -2, 2
	for i, tc := range []struct {
		station []*big.Rat
		owner   []int
	}{
		{[]*big.Rat{big.NewRat(-1, 2), big.NewRat(-7, 128)}, []int{1, 1}},
		{[]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 256)}, []int{0, 0}},
		{[]*big.Rat{big.NewRat(-1, 1), big.NewRat(-1, 2), big.NewRat(-7, 128), big.NewRat(1, 256)}, []int{0, 1, 1, 0}},
	} {
		events, err := bodies[i].IntersectMaterialLineEvents(ctx, geom.Vec3{X: .5, Y: .5}, geom.Vec3{Z: 1}, line)
		if err != nil || len(events) != len(tc.station) {
			t.Fatal("mixed Boolean line event count", i, events, err)
		}
		for j, ev := range events {
			if len(ev.Hits) != 1 || ev.Hits[0].Operand != tc.owner[j] || ev.ExactParameter == nil ||
				ev.ExactParameter.Cmp(tc.station[j]) != 0 || ev.Before != (j%2 == 1) || ev.After != (j%2 == 0) {
				t.Fatal("wrong transition or source owner", i, j, ev)
			}
		}
	}
	// A dyadic point on the roof section: 64(x-1/2)^2-(y-1/2)^2=15/4.
	// The roofs coincide at the exact z=xy/64; the difference starts
	// and ends outside material on opposite sides of this roof contact.
	for i, b := range bodies {
		events, err := b.IntersectMaterialLineEvents(ctx, geom.Vec3{X: 95.0 / 128, Y: 9.0 / 16}, geom.Vec3{Z: 1}, line)
		if err != nil {
			t.Fatal("mixed roof section line", i, err)
		}
		grouped := 0
		for _, ev := range events {
			if len(ev.Hits) != 2 {
				continue
			}
			grouped++
			if ev.ExactParameter == nil || ev.ExactParameter.Cmp(big.NewRat(855, 131072)) != 0 ||
				ev.Hits[0].Operand == ev.Hits[1].Operand || ev.Hits[0].Face != 1 || ev.Hits[1].Face != 1 ||
				ev.Before != (i != 2) || ev.After {
				t.Fatal("lost simultaneous roof ancestry or material states", i, ev)
			}
		}
		if grouped != 1 {
			t.Fatal("simultaneous original roof contact missing", i, events)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := bodies[0].BoundMassProperties(cancelled, AlgebraicShellMeasureOptions{}); !errors.Is(err, context.Canceled) || partial.Volume.Lower != nil {
		t.Fatal("cancellation may not expose partial material measure", partial, err)
	}
}
