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
	var wallRefs [3][7]parametric.Location
	hasWallOracle := false
	if oracle := os.Getenv("PLAINSOLID_OCCT_CROSSING_ROOF_WALLS"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "vertical-wall").Output()
		if err != nil {
			t.Fatal("OCCT vertical wall reference", err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		for i, name := range []string{"intersection", "union", "difference"} {
			var row struct {
				Name      string
				Locations []parametric.Location
			}
			if err := decoder.Decode(&row); err != nil || row.Name != name || len(row.Locations) != 7 {
				t.Fatal("invalid OCCT wall reference", row, err)
			}
			copy(wallRefs[i][:], row.Locations)
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			t.Fatal("trailing OCCT wall reference", err)
		}
		hasWallOracle = true
	}
	var lineRefs [3][]float64
	hasLineOracle := false
	if oracle := os.Getenv("PLAINSOLID_OCCT_CROSSING_ROOF_LINES"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "oblique-line").Output()
		if err != nil {
			t.Fatal("OCCT oblique line reference", err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		for i, name := range []string{"intersection", "union", "difference"} {
			var row struct {
				Name     string
				Stations []float64
			}
			if err := decoder.Decode(&row); err != nil || row.Name != name || len(row.Stations) != [3]int{2, 0, 2}[i] {
				t.Fatal("invalid OCCT oblique line reference", row, err)
			}
			lineRefs[i] = row.Stations
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			t.Fatal("trailing OCCT line reference", err)
		}
		hasLineOracle = true
	}
	var placedRefs [3]reference
	hasPlacedOracle := false
	if oracle := os.Getenv("PLAINSOLID_OCCT_CROSSING_ROOF_PLACED"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "placed-mass").Output()
		if err != nil {
			t.Fatal("OCCT placed mass reference", err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		for i, name := range []string{"intersection", "union", "difference"} {
			if err := decoder.Decode(&placedRefs[i]); err != nil || placedRefs[i].Name != name {
				t.Fatal("invalid OCCT placed mass reference", placedRefs[i], err)
			}
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			t.Fatal("trailing OCCT placed mass reference", err)
		}
		hasPlacedOracle = true
	}
	var distanceRefs [3][]float64
	if oracle := os.Getenv("PLAINSOLID_OCCT_CROSSING_ROOF_DISTANCE"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "boundary-distance").Output()
		if err != nil {
			t.Fatal("OCCT boundary distance reference", err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		for i, name := range []string{"intersection", "union", "difference"} {
			var row struct {
				Name    string
				Squared []float64
			}
			if err := decoder.Decode(&row); err != nil || row.Name != name || len(row.Squared) != 4 {
				t.Fatal("invalid OCCT boundary distance reference", row, err)
			}
			distanceRefs[i] = row.Squared
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			t.Fatal("trailing OCCT distance reference", err)
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
		history, err := body.History()
		if err != nil || len(history.Faces) != 12 || len(history.Edges) != len(body.Edges()) {
			t.Fatal("incomplete original ancestry for retained material", name, history, err)
		}
		seenFaces := make(map[int]bool)
		for _, row := range history.Faces {
			for _, id := range row.ResultFaces {
				f := body.boundary.faces[id]
				if seenFaces[id] || row.Operand != f.Operand || row.OriginalFace != f.Original {
					t.Fatal("fragment ownership duplicates or changes source", name, row, f)
				}
				seenFaces[id] = true
			}
		}
		if len(seenFaces) != ref.Faces || name == "intersection" && (len(history.Faces[0].ResultFaces) != 0 || len(history.Faces[6].ResultFaces) != 1) {
			t.Fatal("wrong retained or deleted floor ancestry", name, history.Faces)
		}
		generated, split := 0, 0
		for _, row := range history.Edges {
			if row.Operand < 0 {
				p, err := body.SectionPiece(row.ResultEdge)
				if err != nil || p.Faces != row.Faces || p.Index != row.Piece || row.Branch < 0 {
					t.Fatal("generated edge lost one of its chart owners", name, row, err)
				}
				generated++
			} else if row.Lower != nil || row.Upper != nil {
				if row.OriginalEdge < 0 || row.Branch != -1 {
					t.Fatal("original split remapped its ancestry", name, row)
				}
				split++
			}
		}
		if generated == 0 || split == 0 {
			t.Fatal("expected both generated and split edges", name, generated, split)
		}
		roof, err := body.PrepareRoofMaterialQuery(ctx, MixedRoofQueryOptions{})
		if err != nil || roof.PreparationOperations() <= 0 {
			t.Fatal("original roof query preparation", name, err)
		}
		delta := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
		height := big.NewRat(-7, 128) // tool roof at x=y=1/2
		for j, want := range [3][3]parametric.Location{
			{parametric.Inside, parametric.Boundary, parametric.Outside},
			{parametric.Inside, parametric.Inside, parametric.Inside},
			{parametric.Outside, parametric.Boundary, parametric.Inside},
		}[i] {
			z := new(big.Rat).Add(height, new(big.Rat).Mul(big.NewRat(int64(j-1), 1), delta))
			got, err := roof.LocateRational(ctx, [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), z}, geom.SurfacePatchEvaluationOptions{})
			if err != nil || got.Location != want || got.Operands[0] != parametric.Inside || got.Operands[1] != [3]parametric.Location{parametric.Inside, parametric.Boundary, parametric.Outside}[j] {
				t.Fatal("sub-binary roof classification", name, j, got, want, err)
			}
		}
		checkMixedOriginalVerticalSection(t, ctx, roof, i, delta)
		checkMixedOriginalAlgebraicLines(t, ctx, roof, i)
		checkPlacedOriginalRoofQueries(t, ctx, roof, i, delta)
		checkOriginalRoofDistance(t, ctx, roof, i)
		if len(distanceRefs[i]) != 0 {
			r := big.NewRat
			points := [4][3]*big.Rat{{r(1, 2), r(1, 2), r(-5, 8)}, {r(1, 2), r(1, 2), r(-9, 8)},
				{r(1, 2), r(1, 2), r(1, 4)}, {r(1, 40), r(1, 2), r(1, 40)}}
			for j, p := range points {
				rangeProof, err := roof.BoundBoundaryDistanceRational(ctx, p, MixedRoofDistanceOptions{
					SquaredGap: r(1, 2000), SurfacePatchEvaluationOptions: geom.SurfacePatchEvaluationOptions{MaxOperations: 4000000},
				})
				if err != nil || !rangeProof.Converged {
					t.Fatal("missing curved boundary distance proof", name, j, rangeProof, err)
				}
				lo, _ := rangeProof.Squared.Lower.Float64()
				hi, _ := rangeProof.Squared.Upper.Float64()
				ref := distanceRefs[i][j]
				if !finite(ref) || ref < lo-1e-8 || ref > hi+1e-8 {
					t.Fatal("OCCT curved boundary distance outside exact interval", name, j, lo, hi, ref)
				}
			}
		}
		if hasLineOracle {
			origin := [3]*big.Rat{new(big.Rat), big.NewRat(1, 4), new(big.Rat)}
			direction := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 4), new(big.Rat)}
			result, err := roof.IntersectLineRational(ctx, origin, direction, big.NewRat(1, 16), big.NewRat(15, 16), MixedRoofLineOptions{})
			if err != nil || len(result.Events) != len(lineRefs[i]) {
				t.Fatal("exact oblique hit count disagrees with OCCT", name, result, lineRefs[i], err)
			}
			for j, event := range result.Events {
				interval, err := event.Parameter.Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
				if err != nil {
					t.Fatal("failed to refine exact oblique station", name, j, err)
				}
				low, _ := interval.Interval.Lower.Float64()
				high, _ := interval.Interval.Upper.Float64()
				ref := lineRefs[i][j]
				if !finite(ref) || ref < low-1e-8 || ref > high+1e-8 {
					t.Fatal("exact oblique station disagrees with OCCT", name, j, low, high, ref)
				}
			}
		}
		if hasWallOracle {
			for j, p := range [7][2]int64{{2, -12}, {2, -4}, {2, 1}, {2, 4}, {0, -12}, {0, -4}, {0, 1}} {
				tmid := big.NewRat(p[1], 16)
				got, err := roof.IntersectVerticalRational(ctx, [2]*big.Rat{new(big.Rat), big.NewRat(p[0], 4)}, new(big.Rat), big.NewRat(1, 1), tmid, tmid, geom.SurfacePatchEvaluationOptions{})
				if err != nil || got.Start != wallRefs[i][j] || got.End != wallRefs[i][j] {
					t.Fatal("exact wall state disagrees with OCCT open sectors", name, j, got, wallRefs[i][j], err)
				}
			}
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
		if hasPlacedOracle {
			r := big.NewRat
			matrix := [3][4]*big.Rat{
				{r(3, 5), r(0, 1), r(4, 5), r(7, 3)},
				{r(0, 1), r(1, 1), r(0, 1), r(2, 7)},
				{r(-4, 5), r(0, 1), r(3, 5), r(-1, 3)},
			}
			pose, err := placement.NewRationalRigid(matrix, 4096)
			if err != nil {
				t.Fatal(err)
			}
			placed, err := body.BoundPlacedMassProperties(ctx, pose, AlgebraicShellMeasureOptions{MaxDepth: 6, TubeSubdivisions: 8})
			ref := placedRefs[i]
			if err != nil || !contains(placed.Volume, ref.Volume) {
				t.Fatal("OCCT placed volume outside native interval", name, placed.Volume, ref.Volume, err)
			}
			for j := range placed.Centroid {
				if !contains(placed.Centroid[j], ref.Centroid[j]) || !contains(placed.FirstMoment[j], ref.Centroid[j]*ref.Volume) {
					t.Fatal("OCCT placed first moment outside native interval", name, j, ref.Centroid[j])
				}
				for k := range placed.Inertia[j] {
					a, b := placed.Inertia[j][k], placed.Inertia[k][j]
					if !contains(a, ref.Inertia[j][k]) || a.Lower.Cmp(b.Lower) != 0 || a.Upper.Cmp(b.Upper) != 0 {
						t.Fatal("OCCT placed origin inertia outside native symmetric interval", name, j, k, ref.Inertia[j][k], a)
					}
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
	// With a distant origin, the positive roof hit is less than one binary64
	// step beyond T=2^47. It must stay outside a closed range ending exactly
	// there, even though its outward presentation interval starts at 2^47.
	line.Line.MinParameter, line.Line.MaxParameter = 0, 1<<47
	ev, err := bodies[1].IntersectMaterialLineEvents(ctx, geom.Vec3{X: .5, Y: .5, Z: -(1 << 47)}, geom.Vec3{Z: 1}, line)
	if err != nil || len(ev) != 1 || ev[0].ExactParameter == nil || ev[0].ExactParameter.Cmp(big.NewRat(1<<47-1, 1)) != 0 || !ev[0].After {
		t.Fatal("clamped exact material event order at distant origin", ev, err)
	}
	ev, err = bodies[1].IntersectMaterialLineEvents(ctx, geom.Vec3{X: .5, Y: .5, Z: 1 << 47}, geom.Vec3{Z: -2}, line)
	if err != nil || len(ev) != 1 || ev[0].ExactParameter == nil || ev[0].ExactParameter.Cmp(new(big.Rat).Sub(big.NewRat(1<<47, 1), big.NewRat(1, 256))) != 0 || !ev[0].After {
		t.Fatal("recentered sweep must preserve negative directions and exact hit units", ev, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := bodies[0].BoundMassProperties(cancelled, AlgebraicShellMeasureOptions{}); !errors.Is(err, context.Canceled) || partial.Volume.Lower != nil {
		t.Fatal("cancellation may not expose partial material measure", partial, err)
	}
}

func checkOriginalRoofDistance(t *testing.T, ctx context.Context, roof *MixedRoofMaterialQuery, op int) {
	t.Helper()
	r := big.NewRat
	// At this XY location the intersection's tool floor and the two
	// operations' stock floor are exact nearest points. In the difference
	// the tool floor above the query is a retained cavity boundary.
	z := [3]*big.Rat{r(-5, 8), r(-9, 8), r(-5, 8)}[op]
	p := [3]*big.Rat{r(1, 2), r(1, 2), z}
	near, err := roof.BoundBoundaryDistanceRational(ctx, p, MixedRoofDistanceOptions{SquaredGap: r(1, 2000)})
	want := r(1, 64)
	owner := [3]MixedRoofFaceOwner{{1, 0}, {0, 0}, {1, 0}}[op]
	if err != nil || !near.Converged || near.Squared.Lower.Cmp(want) > 0 || near.Squared.Upper.Cmp(want) != 0 ||
		len(near.Owners) != 1 || near.Owners[0] != owner {
		t.Fatal("wrong exact material boundary distance or source owner", op, near, err)
	}
	if got, err := roof.LocateRational(ctx, near.Witness, geom.SurfacePatchEvaluationOptions{}); err != nil || got.Location != parametric.Boundary {
		t.Fatal("distance upper bound is not an exposed witness", op, near, got, err)
	}
	// The stock roof touching the tool side is a source seam, not the
	// distance to a hidden infinite support. All three operations expose it.
	p = [3]*big.Rat{r(0, 1), r(1, 2), r(0, 1)}
	seam, err := roof.BoundBoundaryDistanceRational(ctx, p, MixedRoofDistanceOptions{SquaredGap: r(1, 10000)})
	if err != nil || !seam.Converged || seam.Squared.Lower.Sign() != 0 || seam.Squared.Upper.Sign() != 0 ||
		len(seam.Owners) != 2 || seam.Owners[0] != (MixedRoofFaceOwner{0, 1}) || seam.Owners[1] != (MixedRoofFaceOwner{1, 2}) ||
		seam.Witness[0] == p[0] {
		t.Fatal("did not enclose original seam contact", op, seam, err)
	}
	limited, err := roof.BoundBoundaryDistanceRational(ctx, [3]*big.Rat{r(1, 3), r(1, 7), r(1, 20)},
		MixedRoofDistanceOptions{MaxCells: 1, SquaredGap: r(1, 1000000)})
	if !errors.Is(err, geom.ErrConvergence) || limited.Converged || limited.Squared.Lower == nil || limited.VisitedCells != 1 {
		t.Fatal("exhaustion lost the valid global lower bound", op, limited, err)
	}
}

func checkPlacedOriginalRoofQueries(t *testing.T, ctx context.Context, roof *MixedRoofMaterialQuery, op int, delta *big.Rat) {
	t.Helper()
	r := big.NewRat
	distant := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 120))
	pose, err := placement.NewRationalRigid([3][4]*big.Rat{
		{r(3, 5), r(0, 1), r(4, 5), distant},
		{r(0, 1), r(1, 1), r(0, 1), r(2, 7)},
		{r(-4, 5), r(0, 1), r(3, 5), r(-1, 3)},
	}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	q, err := roof.Place(pose)
	if err != nil {
		t.Fatal(err)
	}
	o := geom.SurfacePatchEvaluationOptions{}
	p := [3]*big.Rat{r(1, 2), r(1, 2), new(big.Rat).Add(r(-7, 128), delta)}
	world, err := pose.TransformPoint(ctx, p, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	hit, err := q.LocateRational(ctx, world, o)
	if err != nil || hit.Location != [3]parametric.Location{parametric.Outside, parametric.Inside, parametric.Inside}[op] ||
		hit.Operands != [2]parametric.Location{parametric.Inside, parametric.Outside} {
		t.Fatal("placed point lost exact offset above the original roof", op, hit, err)
	}
	orig, dir := [3]*big.Rat{r(0, 1), r(1, 4), r(0, 1)}, [3]*big.Rat{r(1, 1), r(1, 4), r(0, 1)}
	world, err = pose.TransformPoint(ctx, orig, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	worldDirection, err := pose.TransformVector(ctx, dir, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	rangeStart, rangeEnd := r(1, 16), r(15, 16)
	moved, err := q.IntersectLineRational(ctx, world, worldDirection, rangeStart, rangeEnd, MixedRoofLineOptions{})
	if err != nil {
		t.Fatal("placed exact line", op, err)
	}
	local, err := roof.IntersectLineRational(ctx, orig, dir, rangeStart, rangeEnd, MixedRoofLineOptions{})
	if err != nil || len(local.Events) != len(moved.Events) || len(local.Spans) != len(moved.Spans) ||
		local.Start != moved.Start || local.End != moved.End || moved.ExactOperations-local.ExactOperations != roofPosePointOperations+roofPoseVectorOperations {
		t.Fatal("placement changed exact line states or budget", op, moved, local, err)
	}
	for i, event := range moved.Events {
		ref := local.Events[i]
		order, err := event.Parameter.Compare(ctx, ref.Parameter, geom.ExactCurvePlaneEventOptions{})
		if err != nil || !order.Proven || order.Order != 0 || event.Before != ref.Before || event.After != ref.After ||
			len(event.Owners) != 1 || event.Owners[0] != ref.Owners[0] {
			t.Fatal("placement changed original station or face", op, i, event, order, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, err := q.IntersectLineRational(cancelled, world, worldDirection, rangeStart, rangeEnd, MixedRoofLineOptions{}); !errors.Is(err, context.Canceled) || partial.Events != nil || partial.Spans != nil {
		t.Fatal("placed line cancellation leaked results", partial, err)
	}
	if _, err := roof.Place(placement.RationalRigid{}); err == nil {
		t.Fatal("unproved zero pose")
	}
	z := [3]*big.Rat{r(-5, 8), r(-9, 8), r(-5, 8)}[op]
	original := [3]*big.Rat{r(1, 2), r(1, 2), z}
	world, err = pose.TransformPoint(ctx, original, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	distance, err := q.BoundBoundaryDistanceRational(ctx, world, MixedRoofDistanceOptions{SquaredGap: r(1, 2000)})
	if err != nil || !distance.Converged || distance.Squared.Upper.Cmp(r(1, 64)) != 0 || distance.Squared.Lower.Cmp(r(1, 64)) > 0 ||
		len(distance.Owners) != 1 || distance.Owners[0] != [3]MixedRoofFaceOwner{{1, 0}, {0, 0}, {1, 0}}[op] {
		t.Fatal("placed distance lost exact length or original owner", op, distance, err)
	}
	where, err := q.LocateRational(ctx, distance.Witness, o)
	if err != nil || where.Location != parametric.Boundary {
		t.Fatal("placed distance witness is not on the world boundary", op, where, err)
	}
}

func checkMixedOriginalAlgebraicLines(t *testing.T, ctx context.Context, roof *MixedRoofMaterialQuery, op int) {
	t.Helper()
	r := big.NewRat
	v := func(x, y, z *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, z} }
	opts := MixedRoofLineOptions{}
	lo, hi := r(1, 16), r(15, 16)
	// Z=0 at Y=1/2. The original tool roof along X=t is
	// t²-127t/128+49/256, whose two roots are irrational.
	p := v(r(0, 1), r(1, 2), r(0, 1))
	d := v(r(1, 1), r(0, 1), r(0, 1))
	section, err := roof.IntersectLineRational(ctx, p, d, lo, hi, opts)
	if err != nil || len(section.Events) != [3]int{2, 0, 2}[op] || len(section.Spans) != [3]int{2, 1, 1}[op] {
		t.Fatal("wrong algebraic roof sweep", op, section, err)
	}
	points := []*big.Rat{r(49, 256), r(-127, 128), r(1, 1)}
	before := [3]parametric.Location{parametric.Inside, parametric.Inside, parametric.Outside}[op]
	after := [3]parametric.Location{parametric.Outside, parametric.Inside, parametric.Inside}[op]
	for i, e := range section.Events {
		zero, err := e.Parameter.SignPolynomial(ctx, points, geom.ExactCurvePlaneEventOptions{})
		if err != nil || zero.Sign != 0 || len(e.Owners) != 1 || e.Owners[0] != (MixedRoofFaceOwner{1, 1}) ||
			e.Before != [2]parametric.Location{before, after}[i] || e.After != [2]parametric.Location{after, before}[i] {
			t.Fatal("irrational roof station order, sign or ownership", op, i, e, err)
		}
		k, err := e.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if err != nil || k.Kind != geom.ExactRootKindIrrational {
			t.Fatal("tool roof station is not proven irrational", k, err)
		}
	}
	reversed, err := roof.IntersectLineRational(ctx, v(r(1, 1), r(1, 2), r(0, 1)), v(r(-1, 1), r(0, 1), r(0, 1)), lo, hi, opts)
	if err != nil || len(reversed.Events) != len(section.Events) {
		t.Fatal("line reversal changed exact topology", op, reversed, err)
	}
	for i, e := range reversed.Events {
		orig := section.Events[len(section.Events)-1-i]
		inverse, _, err := e.Parameter.AffineMap(ctx, r(1, 1), r(-1, 1), geom.ExactCurvePlaneEventOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ord, err := inverse.Compare(ctx, orig.Parameter, geom.ExactCurvePlaneEventOptions{})
		if err != nil || !ord.Proven || ord.Order != 0 || e.Before != orig.After || e.After != orig.Before {
			t.Fatal("reversal must preserve exact identity and swap incident states", op, e, ord, err)
		}
	}
	// Put the line exactly at the minimum of the quadratic. The repeated
	// root touches the exposed tool roof without flipping either sign.
	p[2] = r(-3585, 65536)
	tan, err := roof.IntersectLineRational(ctx, p, d, lo, hi, opts)
	if err != nil || len(tan.Events) != [3]int{1, 0, 1}[op] || len(tan.Spans) != [3]int{2, 1, 0}[op] {
		t.Fatal("wrong repeated-root contact", op, tan, err)
	}
	if len(tan.Events) != 0 {
		e := tan.Events[0]
		id, err := e.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if err != nil || id.Kind != geom.ExactRootKindRational || id.Rational.Cmp(r(127, 256)) != 0 || e.Before != before || e.After != before {
			t.Fatal("wrong repeated-root witness or regularized states", op, e, id, err)
		}
	}
	// An entire line segment on the original floor is exposed on two
	// operations and interior to the union. Clip ends must not be hits.
	p[2] = r(-1, 2)
	floor, err := roof.IntersectLineRational(ctx, p, d, lo, hi, opts)
	if err != nil || len(floor.Events) != 0 || len(floor.Spans) != 1 || (floor.Start == parametric.Boundary) != (op != 1) ||
		floor.Start != floor.End || floor.Start != floor.Spans[0].Location || len(floor.Spans[0].Owners) != [3]int{1, 0, 1}[op] {
		t.Fatal("coincident floor was lost or made a discrete contact", op, floor, err)
	}
	// Two distinct original roofs give one exact event at the same point.
	tie := r(855, 131072)
	group, err := roof.IntersectLineRational(ctx, v(r(95, 128), r(9, 16), r(0, 1)), v(r(0, 1), r(0, 1), r(1, 1)), tie, tie, opts)
	if err != nil || group.Start != parametric.Boundary || group.End != parametric.Boundary ||
		len(group.Events) != 1 || len(group.Spans) != 0 || len(group.Events[0].Owners) != 2 ||
		group.Events[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) || group.Events[0].Owners[1] != (MixedRoofFaceOwner{1, 1}) {
		t.Fatal("shared exact clip station lost a roof witness", op, group, err)
	}
	if partial, err := roof.IntersectLineRational(ctx, p, d, lo, hi, MixedRoofLineOptions{SurfacePatchEvaluationOptions: geom.SurfacePatchEvaluationOptions{MaxOperations: 1}}); !errors.Is(err, geom.ErrConvergence) || partial.Events != nil || partial.Spans != nil {
		t.Fatal("unproved algebraic line result escaped budget", op, partial, err)
	}
}

func checkMixedOriginalVerticalSection(t *testing.T, ctx context.Context, roof *MixedRoofMaterialQuery, op int, delta *big.Rat) {
	t.Helper()
	r := big.NewRat
	center := [2]*big.Rat{r(1, 2), r(1, 2)}
	o := geom.SurfacePatchEvaluationOptions{}
	all, err := roof.IntersectVerticalRational(ctx, center, r(0, 1), r(1, 1), r(-2, 1), r(2, 1), o)
	full := []struct {
		z []*big.Rat
		f []MixedRoofFaceOwner
	}{
		{[]*big.Rat{r(-1, 2), r(-7, 128)}, []MixedRoofFaceOwner{{1, 0}, {1, 1}}},
		{[]*big.Rat{r(-1, 1), r(1, 256)}, []MixedRoofFaceOwner{{0, 0}, {0, 1}}},
		{[]*big.Rat{r(-1, 1), r(-1, 2), r(-7, 128), r(1, 256)}, []MixedRoofFaceOwner{{0, 0}, {1, 0}, {1, 1}, {0, 1}}},
	}[op]
	if err != nil || all.Start != parametric.Outside || all.End != parametric.Outside || len(all.Events) != len(full.z) || len(all.Spans)*2 != len(full.z) {
		t.Fatal("incorrect exact vertical material topology", op, all, err)
	}
	for i, e := range all.Events {
		span := all.Spans[i/2]
		endpoint := [2]*big.Rat{span.Lower, span.Upper}[i%2]
		if e.Parameter.Cmp(full.z[i]) != 0 || len(e.Owners) != 1 || e.Owners[0] != full.f[i] ||
			e.Before != (i%2 == 1) || e.After != (i%2 == 0) || endpoint.Cmp(e.Parameter) != 0 || endpoint == e.Parameter {
			t.Fatal("wrong exact vertical span or original face", op, i, e, span)
		}
	}
	// The two original roofs meet transversely at this XY station. Move by
	// 2^-100 to the side where tool is below stock; the narrow difference
	// band must stay distinct, despite a much larger negative line origin.
	xy := [2]*big.Rat{new(big.Rat).Sub(r(95, 128), delta), r(9, 16)}
	stock := new(big.Rat).Quo(new(big.Rat).Mul(xy[0], xy[1]), r(64, 1))
	dx := new(big.Rat).Sub(xy[0], r(1, 2))
	tool := new(big.Rat).Add(stock, new(big.Rat).Sub(new(big.Rat).Mul(dx, dx), r(1, 16)))
	tool.Add(tool, new(big.Rat).Quo(new(big.Rat).Mul(xy[1], new(big.Rat).Sub(r(1, 1), xy[1])), r(64, 1)))
	if tool.Cmp(stock) >= 0 {
		t.Fatal("invalid test formula for narrow stock band", tool, stock)
	}
	offset := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 300))
	for _, sign := range []int64{1, -1} {
		direction := r(7*sign, 3)
		origin := new(big.Rat).Neg(new(big.Rat).Mul(offset, direction))
		param := func(h *big.Rat) *big.Rat {
			return new(big.Rat).Quo(new(big.Rat).Sub(h, origin), direction)
		}
		want := []*big.Rat{param(tool)}
		owners := []int{1}
		if op == 1 {
			want, owners = []*big.Rat{param(stock)}, []int{0}
		}
		if op == 2 {
			want, owners = []*big.Rat{param(tool), param(stock)}, []int{1, 0}
			if sign < 0 {
				want[0], want[1], owners[0], owners[1] = want[1], want[0], owners[1], owners[0]
			}
		}
		a, b := param(tool), param(stock)
		if sign < 0 {
			a, b = b, a
		}
		narrow, err := roof.IntersectVerticalRational(ctx, xy, origin, direction, a, b, o)
		if err != nil || len(narrow.Events) != len(want) {
			t.Fatal("lost unrounded vertical events", op, sign, narrow, err)
		}
		for i, e := range narrow.Events {
			if e.Parameter.Cmp(want[i]) != 0 || len(e.Owners) != 1 || e.Owners[0] != (MixedRoofFaceOwner{owners[i], 1}) {
				t.Fatal("incorrect rational station or original owner", op, sign, e, want[i])
			}
		}
		if op == 2 && (len(narrow.Spans) != 1 || narrow.Spans[0].Lower.Cmp(a) != 0 || narrow.Spans[0].Upper.Cmp(b) != 0 || narrow.Start != parametric.Boundary || narrow.End != parametric.Boundary) {
			t.Fatal("sub-binary retained span changed", sign, narrow)
		}
	}
	xy[0] = r(95, 128)
	tie := r(855, 131072)
	at, err := roof.IntersectVerticalRational(ctx, xy, r(0, 1), r(1, 1), tie, tie, o)
	if err != nil || len(at.Events) != 1 || len(at.Spans) != 0 || at.Start != parametric.Boundary || at.End != parametric.Boundary ||
		len(at.Events[0].Owners) != 2 || at.Events[0].Owners[0] != (MixedRoofFaceOwner{0, 1}) || at.Events[0].Owners[1] != (MixedRoofFaceOwner{1, 1}) ||
		at.Events[0].Before != (op != 2) || at.Events[0].After {
		t.Fatal("coincident line stations must retain both faces and exact contact", op, at, err)
	}
	inside, err := roof.IntersectVerticalRational(ctx, center, r(0, 1), r(1, 1), r(-5, 16), r(-1, 8), o)
	if err != nil || len(inside.Events) != 0 || (len(inside.Spans) == 1) != (op != 2) ||
		(inside.Start == parametric.Inside) != (op != 2) || inside.Start != inside.End {
		t.Fatal("clip endpoints inside a span must not create events", op, inside, err)
	}
	outside, err := roof.IntersectVerticalRational(ctx, [2]*big.Rat{r(2, 1), center[1]}, r(0, 1), r(1, 1), r(-2, 1), r(2, 1), o)
	if err != nil || outside.Start != parametric.Outside || outside.End != parametric.Outside || len(outside.Events)+len(outside.Spans) != 0 {
		t.Fatal("line outside both footprints published material", op, outside, err)
	}
	// A line on the tool's original X-min side is a boundary only where
	// the Boolean exposes it. The stock roof crosses it at exactly Z=0.
	wallXY := [2]*big.Rat{r(0, 1), center[1]}
	wall, err := roof.IntersectVerticalRational(ctx, wallXY, r(0, 1), r(1, 1), r(-2, 1), r(2, 1), o)
	wallBounds := [3][2]*big.Rat{{r(-1, 2), r(0, 1)}, {r(0, 1), r(49, 256)}, {r(-1, 2), r(0, 1)}}[op]
	if err != nil || len(wall.BoundarySpans) != 1 || len(wall.Spans) != [3]int{0, 1, 1}[op] {
		t.Fatal("wrong exposed wall section", op, wall, err)
	}
	side := MixedRoofFaceOwner{1, 2}
	coincident := wall.BoundarySpans[0]
	if len(coincident.Owners) != 1 || coincident.Owners[0] != side || coincident.Parameter.Lower.Cmp(wallBounds[0]) != 0 || coincident.Parameter.Upper.Cmp(wallBounds[1]) != 0 {
		t.Fatal("wrong clipped wall or source side owner", op, coincident, wallBounds)
	}
	hit := wall.Events[[3]int{1, 1, 2}[op]]
	if hit.Parameter.Sign() != 0 || len(hit.Owners) != 2 || hit.Owners[0] != (MixedRoofFaceOwner{0, 1}) || hit.Owners[1] != side || hit.Before != (op == 1) || hit.After {
		t.Fatal("wrong original roof and wall contact", op, hit)
	}
	a := new(big.Rat).Quo(new(big.Rat).Add(wallBounds[0], wallBounds[1]), r(2, 1))
	clipped, err := roof.IntersectVerticalRational(ctx, wallXY, r(0, 1), r(1, 1), a, wallBounds[1], o)
	if err != nil || len(clipped.BoundarySpans) != 1 || len(clipped.Events) != 1 || len(clipped.Spans) != 0 ||
		clipped.Start != parametric.Boundary || clipped.End != parametric.Boundary || clipped.BoundarySpans[0].Parameter.Lower.Cmp(a) != 0 {
		t.Fatal("artificial clip endpoint altered exposed side", op, clipped, err)
	}
	corner, err := roof.IntersectVerticalRational(ctx, [2]*big.Rat{r(0, 1), r(0, 1)}, r(0, 1), r(1, 1), r(-2, 1), r(2, 1), o)
	if err != nil || len(corner.Spans) != 0 || len(corner.BoundarySpans) != [3]int{1, 3, 2}[op] {
		t.Fatal("incorrect side section at perpendicular original faces", op, corner, err)
	}
	both := corner.BoundarySpans[[3]int{0, 1, 1}[op]]
	if len(both.Owners) != 2 || both.Owners[0] != (MixedRoofFaceOwner{0, 4}) || both.Owners[1] != side ||
		both.Parameter.Lower.Cmp(r(-1, 2)) != 0 || both.Parameter.Upper.Sign() != 0 {
		t.Fatal("side intersection lost its two owners", op, both)
	}
	for _, tc := range []struct {
		xy  [2]*big.Rat
		o   geom.SurfacePatchEvaluationOptions
		err error
	}{
		{center, geom.SurfacePatchEvaluationOptions{MaxOperations: all.ExactOperations - 1}, geom.ErrConvergence},
	} {
		partial, err := roof.IntersectVerticalRational(ctx, tc.xy, r(0, 1), r(1, 1), r(-2, 1), r(2, 1), tc.o)
		if !errors.Is(err, tc.err) || partial.Events != nil || partial.Spans != nil || partial.ExactOperations != 0 {
			t.Fatal("unproved vertical result must be atomic", op, partial, err)
		}
	}
}
