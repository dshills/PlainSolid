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

const mappedChainMixedSharedKnot = 29.0 / 256
const mappedChainMixedOwnerKnot = 369.0 / 2048

func mappedChainMixedRoofHeight(owner int, x, y, extraKnot float64) float64 {
	z := x*y + .125*max(0, x-mappedChainMixedSharedKnot)
	if owner == 0 {
		z += .25 * (y - .25) * max(0, x-extraKnot)
	}
	return z
}

func mappedChainMixedFixture(t *testing.T) MappedSplineChainShell {
	t.Helper()
	ctx := context.Background()
	knots := [2][]float64{{mappedChainMixedSharedKnot, mappedChainMixedOwnerKnot}, {mappedChainMixedSharedKnot}}
	height := func(owner int, x, y float64) float64 {
		return mappedChainMixedRoofHeight(owner, x, y, mappedChainMixedOwnerKnot)
	}
	a := mappedChainFixtureKnotSupport(t, 0, knots[0], func(x, y float64) float64 { return height(0, x, y) })
	b := mappedChainFixtureKnotSupport(t, 1, knots[1], func(x, y float64) float64 { return height(1, x, y) })
	_, ca := mappedChainFixtureRawShape(t, false, 273, true)
	_, cb := mappedChainFixtureRawShape(t, true, 35, true)
	boundary, err := CertifyMappedSplineChainMixedSupportKnotsAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return mappedChainLevelTraceFixtureShell(t, boundary, knots, height)
}

func TestMappedSplineChainMixedSupportKnotsRebuild(t *testing.T) {
	ctx := context.Background()
	original := mappedChainMixedFixture(t)
	var data, replay bytes.Buffer
	if err := original.WriteJSON(ctx, &data, MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var record struct{ Boundary nativeMappedChainBoundary }
	if err := json.Unmarshal(data.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Boundary.Version != 6 || len(record.Boundary.Owners[0].Stations) != 1 || len(record.Boundary.Owners[1].Stations) != 2 {
		t.Fatal("recipe must store the original authored partitions and the mixed reproof mode")
	}
	opened, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(data.Bytes()), MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.WriteJSON(ctx, &replay, MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(data.Bytes(), replay.Bytes()) {
		t.Fatal("mixed original support crossings must rederive deterministically", err)
	}
	for _, version := range []string{"4", "5"} {
		older := bytes.Replace(data.Bytes(), []byte(`"Boundary":{"Version":6`), []byte(`"Boundary":{"Version":`+version), 1)
		if bytes.Equal(older, data.Bytes()) {
			t.Fatal("missing boundary format marker")
		}
		if _, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(older), MappedChainShellNativeOptions{}); err == nil {
			t.Fatal("older repair versions must retain their independent support arrangements", version)
		}
	}
	limits := geom.SurfacePatchOptions{}
	out, err := opened.BuildRectangularSolid(ctx, 1e-9, limits)
	if err != nil {
		t.Fatal(err)
	}
	faces, edges, vertices := out.Solid.Faces(), out.Solid.Edges(), out.Solid.Vertices()
	if len(faces) != 7 || len(edges) != 22 || len(vertices) != 17 || len(out.ChainEdges) != 5 ||
		len(out.ChainStations) != 6 || opened.boundary.knotRepair != mappedChainMixedKnotsRepair {
		t.Fatal("both mixed crossings must introduce exactly one shared vertex apiece")
	}
	stations := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 16)}
	points := [2]geom.Vec3{
		{X: mappedChainMixedSharedKnot, Y: .25, Z: 29.0 / 1024},
		{X: mappedChainMixedOwnerKnot, Y: .25, Z: 875.0 / 16384},
	}
	wantRaw := [2][2]int64{{41, 51}, {65, 43}}
	for i, t0 := range stations {
		a, b := edges[out.ChainEdges[i]], edges[out.ChainEdges[i+1]]
		if out.ChainStations[i+1].Cmp(t0) != 0 || a.End != b.Start || vertices[a.End] != points[i] {
			t.Fatal("both owner coedges must share the exact new topological vertex", i)
		}
	}
	for owner, f := range faces[:2] {
		source := original.boundary.sources[owner].parts[0].Source()
		if !geom.SameSplineSurface(f.Surface.(*geom.BSplineSurface), original.boundary.supports[owner].(*geom.BSplineSurface)) {
			t.Fatal("rebuild changed an original support chart", owner)
		}
		p, err := exactSplineSupportPatch(f.Surface, limits)
		if err != nil {
			t.Fatal(err)
		}
		var domains [3]geom.ParameterRectangle
		var cells [3]*geom.SurfacePatch
		for i := range domains {
			use := f.Outer[i]
			q := use.Curve.(MappedSplineUV)
			if use.Edge != out.ChainEdges[i] || q.Source().Reversed != source.Reversed ||
				!sameAuthoredBezierSpline(q.Source().Curve, source.Curve) {
				t.Fatal("native subedge lost original owner basis and orientation", owner, i)
			}
			for side := range 2 {
				cross := i + side - 1
				if cross < 0 || cross > 1 {
					continue
				}
				t0, err := q.AuthoredParameter(big.NewRat(int64(side), 1))
				if err != nil || t0.Cmp(big.NewRat(wantRaw[cross][owner], 1)) != 0 {
					t.Fatal("native subedge changed an exact shared raw station", owner, cross, err)
				}
			}
			d, err := q.curve.descriptor(ctx, limits)
			if err != nil {
				t.Fatal(err)
			}
			cell, err := trimModelSupportCell(ctx, p, d, limits)
			if err != nil {
				t.Fatal(err)
			}
			domains[i] = cell.Domain()
			cells[i] = cell
		}
		if domains[0] == domains[1] || (domains[1] != domains[2]) != (owner == 0) {
			t.Fatal("both owners must change cells only at the shared knot", owner, domains)
		}
		for crossing, point := range points {
			x, y := ratInset(point.X), big.NewRat(1, 4)
			wantJump := big.NewRat(1, 8)
			if crossing == 1 {
				y = big.NewRat(1, 2)
				wantJump = new(big.Rat)
				if owner == 0 {
					wantJump = big.NewRat(1, 16)
				}
			}
			u, v := x, y
			if owner == 1 {
				u, v = y, x
			}
			var derivative [2]*big.Rat
			for side, cell := range cells[crossing : crossing+2] {
				j, err := cell.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				derivative[side] = [2]*big.Rat{j.Du[2], j.Dv[2]}[owner]
			}
			if new(big.Rat).Sub(derivative[1], derivative[0]).Cmp(wantJump) != 0 {
				t.Fatal("each original support must retain its one-sided geometry", owner, crossing)
			}
		}
	}
	all, err := out.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !all.Complete || len(all.Uses) != 44 {
		t.Fatal("native solid requires exact proof of all owner incidences", err)
	}
	r := original.boundary
	reversed, err := CertifyMappedSplineChainMixedSupportKnotsAgreement(ctx, r.supports[1], r.sources[1], r.supports[0], r.sources[0], edgecheck.Options{})
	if err != nil || len(reversed.cuts) != len(r.cuts) || reversed.cuts[2].Cmp(stations[1]) != 0 {
		t.Fatal("owner order must not change which exact stations are generated", err)
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
		t.Fatal("closed display must prove one common partition across both mixed crossings", err)
	}
	for i, station := range stations {
		found := false
		for _, sample := range mesh.ChainSamples {
			if sample.ModelParameter.Cmp(station) == 0 && mesh.Mesh.Vertices[sample.Vertex] == points[i] {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("closed display lost the exact common support crossing", i)
		}
	}
	t.Logf("%d display triangles preserve both mixed stations and have no unintended contacts", len(mesh.Mesh.Triangles))
}

func TestMappedSplineChainMixedSupportKnotsWithEarlierSingleOwnerCrossing(t *testing.T) {
	r := mappedChainMixedFixture(t).boundary
	height := func(owner int, x, y float64) float64 {
		z := x*y + .125*max(0, x-mappedChainMixedOwnerKnot)
		if owner == 0 {
			z += .25 * (y - .25) * max(0, x-mappedChainMixedSharedKnot)
		}
		return z
	}
	a := mappedChainFixtureKnotSupport(t, 0, []float64{mappedChainMixedSharedKnot, mappedChainMixedOwnerKnot}, func(x, y float64) float64 {
		return height(0, x, y)
	})
	b := mappedChainFixtureKnotSupport(t, 1, []float64{mappedChainMixedOwnerKnot}, func(x, y float64) float64 {
		return height(1, x, y)
	})
	proof, err := CertifyMappedSplineChainMixedSupportKnotsAgreement(context.Background(), a, r.sources[0], b, r.sources[1], edgecheck.Options{})
	if err != nil || len(proof.cuts) != 6 || proof.cuts[1].Cmp(big.NewRat(1, 8)) != 0 || proof.cuts[2].Cmp(big.NewRat(3, 16)) != 0 {
		t.Fatal("the single-owner crossing can precede the shared crossing in one original cell", err)
	}
}

func TestMappedSplineChainMixedSupportKnotsRejectsOtherArrangements(t *testing.T) {
	ctx := context.Background()
	r := mappedChainMixedFixture(t).boundary
	check := func(a, b geom.Surface, ca MappedSplineChain, want error, message string) {
		t.Helper()
		bad, err := CertifyMappedSplineChainMixedSupportKnotsAgreement(ctx, a, ca, b, r.sources[1], edgecheck.Options{})
		if !errors.Is(err, want) || !strings.Contains(err.Error(), message) || len(bad.ModelCuts()) != 0 {
			t.Fatal("mixed support-knot proof failure must be atomic", message, err)
		}
	}
	twoSync := mappedChainFixtureKnotSupport(t, 1, []float64{mappedChainMixedSharedKnot, mappedChainMixedOwnerKnot}, func(x, y float64) float64 {
		return mappedChainMixedRoofHeight(1, x, y, mappedChainMixedOwnerKnot)
	})
	check(r.supports[0], twoSync, r.sources[0], geom.ErrConvergence, "one shared crossing and one single-owner")
	const later = 99.0 / 256
	crossesAnotherCell := mappedChainFixtureKnotSupport(t, 0, []float64{mappedChainMixedSharedKnot, later}, func(x, y float64) float64 {
		return mappedChainMixedRoofHeight(0, x, y, later)
	})
	check(crossesAnotherCell, r.supports[1], r.sources[0], geom.ErrConvergence, "span common chain cells")
	_, narrow := mappedChainFixtureRawShape(t, false, 21, true)
	check(r.supports[0], r.supports[1], narrow, geom.ErrUnrepresentable, "original restriction")
	changedLift := mappedChainFixtureKnotSupport(t, 1, []float64{mappedChainMixedSharedKnot}, func(x, y float64) float64 {
		return x*y + .25*max(0, x-mappedChainMixedSharedKnot)
	})
	check(r.supports[0], changedLift, r.sources[0], geom.ErrConvergence, "")
}
