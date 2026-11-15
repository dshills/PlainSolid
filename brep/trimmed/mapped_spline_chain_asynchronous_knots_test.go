package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

var mappedChainAsynchronousKnots = [2]float64{29.0 / 256, 99.0 / 256}

func mappedChainAsynchronousHeight(x, y, knot float64) float64 {
	return x*y + .25*(y-.25)*max(0, x-knot)
}

func mappedChainAsynchronousSupport(t *testing.T, owner int, knot float64) *geom.BSplineSurface {
	t.Helper()
	return mappedChainFixtureKnotSupport(t, owner, []float64{knot}, func(x, y float64) float64 {
		return mappedChainAsynchronousHeight(x, y, knot)
	})
}

func mappedChainFixtureKnotSupport(t *testing.T, owner int, knots []float64, height func(x, y float64) float64) *geom.BSplineSurface {
	t.Helper()
	u, v := append([]float64{0}, knots...), []float64{0, 1}
	u = append(u, 1)
	ku, kv := append(append([]float64{0}, u...), 1), []float64{0, 0, 1, 1}
	if owner == 1 {
		u, v, ku, kv = v, u, kv, ku
	}
	var poles []geom.Vec3
	for _, a := range u {
		for _, b := range v {
			x, y := a, b
			if owner == 1 {
				x, y = b, a
			}
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: height(x, y)})
		}
	}
	surface, err := geom.NewBSplineSurface(1, 1, len(u), len(v), poles, ku, kv, nil)
	if err != nil {
		t.Fatal(err)
	}
	return surface
}

func mappedChainAsynchronousFixture(t *testing.T) MappedSplineChainShell {
	t.Helper()
	ctx := context.Background()
	k0, k1 := mappedChainAsynchronousKnots[0], mappedChainAsynchronousKnots[1]
	a, b := mappedChainAsynchronousSupport(t, 0, k0), mappedChainAsynchronousSupport(t, 1, k1)
	_, ca := mappedChainFixtureRawShape(t, false, 105, true)
	_, cb := mappedChainFixtureRawShape(t, true, 5, true)
	boundary, err := CertifyMappedSplineChainAsynchronousSupportKnotsAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	knots := [2][]float64{{k0}, {k1}}
	height := func(owner int, x, y float64) float64 {
		return mappedChainAsynchronousHeight(x, y, knots[owner][0])
	}
	return mappedChainLevelTraceFixtureShell(t, boundary, knots, height)
}

func mappedChainLevelTraceFixtureShell(t *testing.T, boundary MappedSplineChainAgreement, knots [2][]float64, height func(int, float64, float64) float64) MappedSplineChainShell {
	t.Helper()
	line := func(a, b geom.Vec2) PCurve { return mappedChainFixtureLine(t, a, b) }
	wallHeight := func(owner int, x, y float64) float64 { return (1 + height(owner, x, y)) / 2 }
	var closing [2][]PCurve
	var profiles [2][]geom.Vec2
	for owner, y := range [2]float64{.75, 0} {
		uv := func(x, y float64) geom.Vec2 {
			if owner == 0 {
				return geom.Vec2{X: x, Y: y}
			}
			return geom.Vec2{X: y, Y: x}
		}
		xs := []float64{1}
		for i := len(knots[owner]) - 1; i >= 0; i-- {
			xs = append(xs, knots[owner][i])
		}
		xs = append(xs, 0)
		closing[owner] = append(closing[owner], line(uv(1, .25), uv(1, y)))
		profiles[owner] = []geom.Vec2{{}, {X: 1}}
		for i, x := range xs {
			profiles[owner] = append(profiles[owner], geom.Vec2{X: x, Y: wallHeight(owner, x, y)})
			if i+1 < len(xs) {
				closing[owner] = append(closing[owner], line(uv(x, y), uv(xs[i+1], y)))
			}
		}
		closing[owner] = append(closing[owner], line(uv(0, y), uv(0, .25)))
	}
	face := func(origin, u, v geom.Vec3, reverse bool, ring ...geom.Vec2) MappedChainPlanarFace {
		return mappedChainFixturePlanarFace(t, origin, u, v, reverse, ring...)
	}
	x, y, z := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 2}
	planar := []MappedChainPlanarFace{
		face(geom.Vec3{X: 1, Z: -1}, y, z, false,
			geom.Vec2{}, geom.Vec2{X: .75}, geom.Vec2{X: .75, Y: wallHeight(0, 1, .75)},
			geom.Vec2{X: .25, Y: wallHeight(0, 1, .25)}, geom.Vec2{Y: wallHeight(1, 1, 0)}),
		face(geom.Vec3{Z: -1}, y, z, true,
			geom.Vec2{}, geom.Vec2{X: .75}, geom.Vec2{X: .75, Y: .5}, geom.Vec2{X: .25, Y: .5}, geom.Vec2{Y: .5}),
		face(geom.Vec3{Y: .75, Z: -1}, x, z, true, profiles[0]...),
		face(geom.Vec3{Z: -1}, x, z, false, profiles[1]...),
		face(geom.Vec3{Z: -1}, x, y, true,
			geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: .75}, geom.Vec2{Y: .75}),
	}
	var seams [][2]MappedChainClosedUse
	for owner, trims := range closing {
		seams = append(seams, [2]MappedChainClosedUse{{owner, 0}, {2, 2 + owner}})
		last := len(trims) - 1
		for i := 1; i < last; i++ {
			seams = append(seams, [2]MappedChainClosedUse{{owner, i}, {4 + owner, i + 1}})
		}
		seams = append(seams, [2]MappedChainClosedUse{{owner, last}, {3, 2 + owner}})
	}
	seams = append(seams, [][2]MappedChainClosedUse{
		{{2, 1}, {4, 1}}, {{2, 4}, {5, 1}}, {{3, 1}, {4, len(profiles[0]) - 1}}, {{3, 4}, {5, len(profiles[1]) - 1}},
		{{2, 0}, {6, 1}}, {{3, 0}, {6, 3}}, {{4, 0}, {6, 2}}, {{5, 0}, {6, 0}},
	}...)
	shell, err := CertifyMappedSplineChainShell(context.Background(), boundary, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return shell
}

func TestMappedSplineChainAsynchronousSupportKnotsRebuild(t *testing.T) {
	ctx := context.Background()
	source := mappedChainAsynchronousFixture(t)
	var raw, replay bytes.Buffer
	if err := source.WriteJSON(ctx, &raw, MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var record struct{ Boundary nativeMappedChainBoundary }
	if err := json.Unmarshal(raw.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Boundary.Version != 5 || len(record.Boundary.Owners[0].Stations) != 1 || len(record.Boundary.Owners[1].Stations) != 2 {
		t.Fatal("asynchronous native recipe stores only original source intervals and the repair mode")
	}
	opened, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw.Bytes()), MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.WriteJSON(ctx, &replay, MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
		t.Fatal("asynchronous native repair must regenerate the same source recipe", err)
	}
	out, err := opened.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	edges, faces, vertices := out.Solid.Edges(), out.Solid.Faces(), out.Solid.Vertices()
	if len(edges) != 21 || len(vertices) != 16 || len(faces) != 7 || len(out.ChainEdges) != 5 ||
		len(out.ChainStations) != 6 || opened.boundary.knotRepair != mappedChainAsynchronousKnotsRepair {
		t.Fatal("each original cross-knot cell must supply one new common native vertex")
	}
	neighborCells := [2]int{0, 2}
	stations := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 8)}
	wantedRaw := [2][2]int64{{17, 27}, {65, 21}}
	limits := geom.SurfacePatchOptions{}
	for cross, cell := range neighborCells {
		knot := mappedChainAsynchronousKnots[cross]
		a, b := edges[out.ChainEdges[cell]], edges[out.ChainEdges[cell+1]]
		if out.ChainStations[cell+1].Cmp(stations[cross]) != 0 || a.End != b.Start ||
			vertices[a.End] != (geom.Vec3{X: knot, Y: .25, Z: .25 * knot}) {
			t.Fatal("native neighboring edges must share one exact physical station", cross)
		}
		for owner, face := range faces[:2] {
			patch := face.Surface.(*geom.BSplineSurface)
			if !geom.SameSplineSurface(patch, source.boundary.supports[owner].(*geom.BSplineSurface)) {
				t.Fatal("original owner support's knot and control net changed", owner)
			}
			p, err := exactSplineSupportPatch(patch, limits)
			if err != nil {
				t.Fatal(err)
			}
			var domains [2]geom.ParameterRectangle
			for offset := range 2 {
				curve := face.Outer[cell+offset].Curve.(MappedSplineUV)
				raw, err := curve.AuthoredParameter(big.NewRat(int64(1-offset), 1))
				prior := source.boundary.sources[owner].parts[0].Source()
				if err != nil || raw.Cmp(big.NewRat(wantedRaw[cross][owner], 1)) != 0 ||
					curve.Source().Reversed != prior.Reversed || !sameAuthoredBezierSpline(prior.Curve, curve.Source().Curve) {
					t.Fatal("paired raw original restriction changed", cross, owner, err)
				}
				d, err := curve.curve.descriptor(ctx, limits)
				if err != nil {
					t.Fatal(err)
				}
				cell, err := trimModelSupportCell(ctx, p, d, limits)
				if err != nil {
					t.Fatal("both sides must have an exact single-cell lift in each original chart", cross, owner, err)
				}
				domains[offset] = cell.Domain()
			}
			if (domains[0] != domains[1]) != (owner == cross) {
				t.Fatal("only the owner of this station should change original knot cells", owner, cross, domains)
			}
		}
	}
	proof, err := out.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 42 {
		t.Fatal("native asynchronous shell must prove every complete coedge", err)
	}
	for owner, f := range faces[:2] {
		patch, err := exactSplineSupportPatch(f.Surface, limits)
		if err != nil {
			t.Fatal(err)
		}
		knot := ratInset(mappedChainAsynchronousKnots[owner])
		u, v := knot, big.NewRat(1, 2)
		split := patch.SplitU
		if owner == 1 {
			u, v, split = big.NewRat(1, 8), knot, patch.SplitV
		}
		left, right, err := split(mappedChainAsynchronousKnots[owner])
		if err != nil {
			t.Fatal(err)
		}
		var slope [2]*big.Rat
		for side, p := range [2]*geom.SurfacePatch{left, right} {
			j, err := p.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			slope[side] = [2]*big.Rat{j.Du[2], j.Dv[2]}[owner]
		}
		wantJump := [2]*big.Rat{big.NewRat(1, 16), big.NewRat(-1, 32)}[owner]
		if new(big.Rat).Sub(slope[1], slope[0]).Cmp(wantJump) != 0 {
			t.Fatal("each roof must retain its actual one-sided crease away from the shared edge", owner)
		}
	}
	r := source.boundary
	reordered, err := CertifyMappedSplineChainAsynchronousSupportKnotsAgreement(ctx, r.supports[1], r.sources[1], r.supports[0], r.sources[0], edgecheck.Options{})
	if err != nil || len(reordered.cuts) != len(r.cuts) {
		t.Fatal("discovering the larger crossing first must preserve deterministic station order", err)
	}
	for i, cut := range reordered.cuts {
		if cut.Cmp(r.cuts[i]) != 0 {
			t.Fatal("source owner order changed the common station sequence", i)
		}
	}
	fill := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .04},
	}}
	mesh, err := opened.TessellateClosedCertified(ctx, nil, MappedChainClosedMeshOptions{
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{fill, fill},
	})
	if err != nil || !mesh.DisplayCollision.Complete || len(mesh.DisplayCollision.Contacts) != 0 {
		t.Fatal("closed display must use the same certified boundary partition at both asynchronous stations", err)
	}
	for i, station := range stations {
		x := mappedChainAsynchronousKnots[i]
		point := geom.Vec3{X: x, Y: .25, Z: .25 * x}
		found := false
		for _, sample := range mesh.ChainSamples {
			if sample.ModelParameter.Cmp(station) == 0 && mesh.Mesh.Vertices[sample.Vertex] == point {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("closed display lost the exact paired crossing", i)
		}
	}
	t.Logf("%d display triangles preserve both asynchronous stations and have no unintended contacts", len(mesh.Mesh.Triangles))
}

func TestMappedSplineChainAsynchronousSupportKnotsRejectsUnsupportedArrangements(t *testing.T) {
	ctx := context.Background()
	r := mappedChainAsynchronousFixture(t).boundary
	check := func(other geom.Surface, ca MappedSplineChain, want error, partial string) {
		t.Helper()
		p, err := CertifyMappedSplineChainAsynchronousSupportKnotsAgreement(ctx, r.supports[0], ca, other, r.sources[1], edgecheck.Options{})
		if !errors.Is(err, want) || !strings.Contains(err.Error(), partial) || len(p.ModelCuts()) != 0 {
			t.Fatal("bounded asynchronous proof must reject atomically", partial, err)
		}
	}
	sameKnot := mappedChainAsynchronousSupport(t, 1, mappedChainAsynchronousKnots[0])
	check(sameKnot, r.sources[0], geom.ErrConvergence, "distinct common chain cells")
	sameCell := mappedChainAsynchronousSupport(t, 1, 369.0/2048)
	check(sameCell, r.sources[0], geom.ErrConvergence, "distinct common chain cells")
	_, unrepresentableCompanion := mappedChainFixtureRawShape(t, false, 21, true)
	check(r.supports[1], unrepresentableCompanion, geom.ErrUnrepresentable, "roof 0")
}
