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

const mappedChainFirstCrossingKnot = 29.0 / 256
const mappedChainSecondCrossingKnot = 369.0 / 2048
const mappedChainCrossingSlope = .125

func mappedChainTwoKnotFixture(t *testing.T) MappedSplineChainShell {
	t.Helper()
	ctx := context.Background()
	extra := [2]float64{mappedChainSecondCrossingKnot, mappedChainCrossingSlope}
	a := mappedChainCreasedSupport(t, false, mappedChainFirstCrossingKnot, mappedChainCrossingSlope, extra)
	b := mappedChainCreasedSupport(t, true, mappedChainFirstCrossingKnot, mappedChainCrossingSlope, extra)
	_, ca := mappedChainFixtureWithRawWidth(t, false, 273)
	_, cb := mappedChainFixtureWithRawWidth(t, true, 35)
	boundary, err := CertifyMappedSplineChainTwoSupportKnotsAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	closing, planar, seams := mappedChainFixtureClosures(t, mappedChainFirstCrossingKnot, mappedChainCrossingSlope, extra)
	shell, err := CertifyMappedSplineChainShell(ctx, boundary, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return shell
}

func TestMappedSplineChainTwoSupportKnotsRebuild(t *testing.T) {
	ctx := context.Background()
	shell := mappedChainTwoKnotFixture(t)
	var raw, replay bytes.Buffer
	if err := shell.WriteJSON(ctx, &raw, MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var record struct{ Boundary nativeMappedChainBoundary }
	if err := json.Unmarshal(raw.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Boundary.Version != 4 || len(record.Boundary.Owners[0].Stations) != 1 || len(record.Boundary.Owners[1].Stations) != 2 {
		t.Fatal("source-only recipe must retain the authored station inventory and v4 proof mode")
	}
	opened, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw.Bytes()), MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.WriteJSON(ctx, &replay, MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
		t.Fatal("v4 source replay must rediscover exactly both new cuts", err)
	}
	wantStation := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 16)}
	wantRaw := [2][2]int64{{41, 65}, {51, 43}}
	wantPoint := [2]geom.Vec3{
		{X: mappedChainFirstCrossingKnot, Y: 23.0 / 128, Z: 667.0 / 32768},
		{X: mappedChainSecondCrossingKnot, Y: 83.0 / 512, Z: 39395.0 / 1048576},
	}
	solid, err := opened.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	edges, vertices, faces := solid.Solid.Edges(), solid.Solid.Vertices(), solid.Solid.Faces()
	if len(edges) != 23 || len(vertices) != 18 || len(faces) != 7 || len(solid.ChainEdges) != 5 ||
		len(solid.ChainStations) != 6 || opened.boundary.knotRepair != mappedChainSynchronizedKnotsRepair {
		t.Fatal("two original support crossings must add exactly two shared native vertices")
	}
	for i, station := range wantStation {
		a, b := edges[solid.ChainEdges[i]], edges[solid.ChainEdges[i+1]]
		if solid.ChainStations[i+1].Cmp(station) != 0 || a.End != b.Start || vertices[a.End] != wantPoint[i] {
			t.Fatal("changed exact shared support crossing", i)
		}
	}
	for owner, face := range faces[:2] {
		original := shell.boundary.sources[owner].parts[0].Source()
		if !geom.SameSplineSurface(face.Surface.(*geom.BSplineSurface), shell.boundary.supports[owner].(*geom.BSplineSurface)) {
			t.Fatal("native rebuild changed the original support's control net or knot vector", owner)
		}
		for i := range 3 {
			use := face.Outer[i]
			curve := use.Curve.(MappedSplineUV)
			if use.Edge != solid.ChainEdges[i] || curve.Source().Reversed != original.Reversed ||
				!sameAuthoredBezierSpline(curve.Source().Curve, original.Curve) {
				t.Fatal("native rebuild changed the original owner basis", owner, i)
			}
			if _, err := ExactTrimModelBezier(ctx, face.Surface, curve, false, geom.SurfacePatchOptions{}); err != nil {
				t.Fatal("each restricted coedge must fit its original support cell", owner, i, err)
			}
		}
		for i, raw := range wantRaw[owner] {
			left, err := face.Outer[i].Curve.(MappedSplineUV).AuthoredParameter(big.NewRat(1, 1))
			if err != nil {
				t.Fatal(err)
			}
			right, err := face.Outer[i+1].Curve.(MappedSplineUV).AuthoredParameter(new(big.Rat))
			if err != nil || left.Cmp(big.NewRat(raw, 1)) != 0 || left.Cmp(right) != 0 {
				t.Fatal("exact raw parameter was lost on one side of the coedge split", owner, i, err)
			}
		}
	}
	proof, err := solid.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 46 {
		t.Fatal("native edge validation must cover both sides of all three restricted coedges", err)
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
		t.Fatal("closed display requires a common mesh partition through both original knots", err)
	}
	for i, station := range wantStation {
		found := false
		for _, sample := range mesh.ChainSamples {
			if sample.ModelParameter.Cmp(station) == 0 && mesh.Mesh.Vertices[sample.Vertex] == wantPoint[i] {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("shared display lost an exact crossing station", i)
		}
	}
	t.Logf("%d triangles retain both shared stations with no unintended contacts", len(mesh.Mesh.Triangles))
}

func TestMappedSplineChainTwoSupportKnotsRejectsUnsynchronizedSource(t *testing.T) {
	ctx := context.Background()
	shell := mappedChainTwoKnotFixture(t)
	r := shell.boundary
	check := func(a, b geom.Surface, ca, cb MappedSplineChain, o edgecheck.Options, want error, content string) {
		t.Helper()
		p, err := CertifyMappedSplineChainTwoSupportKnotsAgreement(ctx, a, ca, b, cb, o)
		if !errors.Is(err, want) || (content != "" && !strings.Contains(err.Error(), content)) || len(p.ModelCuts()) != 0 {
			t.Fatal("unsupported crossing arrangement must fail atomically", content, err)
		}
	}
	check(r.supports[0], r.supports[1], r.sources[0], r.sources[1], edgecheck.Options{MaxIntervals: 4}, geom.ErrConvergence, "")
	_, narrow := mappedChainFixture(t, false)
	check(r.supports[0], r.supports[1], narrow, r.sources[1], edgecheck.Options{}, geom.ErrUnrepresentable, "")
	one := mappedChainCreasedSupport(t, true, mappedChainFirstCrossingKnot, mappedChainCrossingSlope)
	check(r.supports[0], one, r.sources[0], r.sources[1], edgecheck.Options{}, geom.ErrConvergence, "requires two original")
	unsynchronized := mappedChainCreasedSupport(t, true, 107.0/2048, mappedChainCrossingSlope,
		[2]float64{mappedChainSecondCrossingKnot, mappedChainCrossingSlope})
	check(r.supports[0], unsynchronized, r.sources[0], r.sources[1], edgecheck.Options{}, geom.ErrConvergence, "disagree in model stations")
	irrationalSecond := [2]*geom.BSplineSurface{
		mappedChainCreasedSupport(t, false, mappedChainFirstCrossingKnot, mappedChainCrossingSlope, [2]float64{.125, mappedChainCrossingSlope}),
		mappedChainCreasedSupport(t, true, mappedChainFirstCrossingKnot, mappedChainCrossingSlope, [2]float64{.125, mappedChainCrossingSlope}),
	}
	check(irrationalSecond[0], irrationalSecond[1], r.sources[0], r.sources[1], edgecheck.Options{}, geom.ErrUnrepresentable, "exact rational crossing")
	separated := [2]*geom.BSplineSurface{
		mappedChainCreasedSupport(t, false, mappedChainFirstCrossingKnot, mappedChainCrossingSlope, [2]float64{99.0 / 256, mappedChainCrossingSlope}),
		mappedChainCreasedSupport(t, true, mappedChainFirstCrossingKnot, mappedChainCrossingSlope, [2]float64{99.0 / 256, mappedChainCrossingSlope}),
	}
	check(separated[0], separated[1], r.sources[0], r.sources[1], edgecheck.Options{}, geom.ErrConvergence, "span common chain cells")
}
