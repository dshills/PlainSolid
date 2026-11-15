package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestMappedSplineChainSupportKnotRebuild(t *testing.T) {
	ctx := context.Background()
	const knot, slope = 29.0 / 256, .25
	a, b := mappedChainCreasedSupport(t, false, knot, slope), mappedChainCreasedSupport(t, true, knot, slope)
	_, ca := mappedChainFixtureWithRawWidth(t, false, 21)
	_, cb := mappedChainFixture(t, true)
	limits := geom.SurfacePatchOptions{}
	if _, err := ExactTrimModelBezier(ctx, a, ca.parts[0], false, limits); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("the authored trim crosses an original support knot", err)
	}
	r, err := CertifyMappedSplineChainSupportKnotAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantStation := big.NewRat(1, 8)
	wantRaw := [2]*big.Rat{big.NewRat(5, 1), big.NewRat(27, 1)}
	wantPoint := [3]*big.Rat{big.NewRat(29, 256), big.NewRat(23, 128), big.NewRat(667, 32768)}
	checkBoundary := func(p MappedSplineChainAgreement) {
		t.Helper()
		cuts, points := p.ModelCuts(), p.ModelPoints()
		if len(cuts) != 5 || cuts[1].Cmp(wantStation) != 0 || cuts[2].Cmp(big.NewRat(1, 4)) != 0 ||
			cuts[3].Cmp(big.NewRat(1, 2)) != 0 || p.splitSupportKnots != 1 {
			t.Fatal("expected one new exact station inside the authored partition", cuts)
		}
		for i, coord := range points[1] {
			if coord.Cmp(wantPoint[i]) != 0 {
				t.Fatal("wrong common knot model point", points[1])
			}
		}
		for owner, original := range [2]MappedSplineChain{ca, cb} {
			raw, err := p.AuthoredParameters(owner)
			if err != nil || raw[1].Cmp(wantRaw[owner]) != 0 ||
				!sameAuthoredBezierSpline(p.sources[owner].parts[0].Source().Curve, original.parts[0].Source().Curve) {
				t.Fatal("lost the original pcurve basis or exact raw knot parameter", owner, err)
			}
		}
	}
	checkBoundary(r)
	closing, planar, seams := mappedChainFixtureClosures(t, knot, slope)
	shell, err := CertifyMappedSplineChainShell(ctx, r, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var native, replay bytes.Buffer
	if err := shell.WriteJSON(ctx, &native, MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	opened, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(native.Bytes()), MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checkBoundary(opened.boundary)
	if err := opened.WriteJSON(ctx, &replay, MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
		t.Fatal("support-knot recipe must deterministically reprove and replay", err)
	}
	out, err := opened.BuildRectangularSolid(ctx, 1e-9, limits)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.ChainEdges) != 4 || len(out.Solid.Edges()) != 20 || len(out.Solid.Vertices()) != 15 || len(out.Solid.Faces()) != 7 {
		t.Fatal("the cross-knot split must be a single shared native vertex")
	}
	edges, vertices, faces := out.Solid.Edges(), out.Solid.Vertices(), out.Solid.Faces()
	before, after := edges[out.ChainEdges[0]], edges[out.ChainEdges[1]]
	point, err := geom.RepresentedExactPoint(wantPoint)
	if err != nil || before.End != after.Start || vertices[before.End] != point {
		t.Fatal("native shared vertex has drifted", err)
	}
	for owner := range 2 {
		if !geom.SameSplineSurface(faces[owner].Surface.(*geom.BSplineSurface), r.supports[owner].(*geom.BSplineSurface)) {
			t.Fatal("native roof chart changed", owner)
		}
		for cell := range 2 {
			use := faces[owner].Outer[cell]
			p := use.Curve.(MappedSplineUV)
			parameter, err := p.AuthoredParameter(big.NewRat(int64(1-cell), 1))
			if err != nil || parameter.Cmp(wantRaw[owner]) != 0 || use.Edge != out.ChainEdges[cell] ||
				!sameAuthoredBezierSpline(p.Source().Curve, r.sources[owner].parts[0].Source().Curve) {
				t.Fatal("paired restricted coedge changed original source", owner, cell, err)
			}
			if _, err := ExactTrimModelBezier(ctx, faces[owner].Surface, p, false, limits); err != nil {
				t.Fatal("each original restriction must have a one-cell model lift", owner, cell, err)
			}
		}
	}
	all, err := out.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !all.Complete || len(all.Uses) != 40 {
		t.Fatal("complete exact source agreement on rebuilt coedges", err)
	}
	line, err := opened.boundary.TessellateCertified(ctx, MappedChainPolylineOptions{Deflection: .005})
	if err != nil || len(line.KnotVertices) != 5 || line.Vertices[line.KnotVertices[1]].Point != point {
		t.Fatal("certified boundary ledger lost the original support crossing", err)
	}
	const tolerance = .04
	meshOpts := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: tolerance},
	}}
	mesh, err := opened.TessellateClosedCertified(ctx, nil, MappedChainClosedMeshOptions{
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{meshOpts, meshOpts},
	})
	if err != nil || !mesh.DisplayCollision.Complete || len(mesh.DisplayCollision.Contacts) != 0 {
		t.Fatal("closed display must preserve the new source cut", err)
	}
	for _, sample := range mesh.ChainSamples {
		if sample.ModelParameter.Cmp(wantStation) == 0 && mesh.Mesh.Vertices[sample.Vertex] == point {
			t.Logf("paired closed display uses exact crossing in %d triangles", len(mesh.Mesh.Triangles))
			return
		}
	}
	t.Fatal("closed display lost the exact shared model station")
}

func TestMappedSplineChainSupportKnotRejectsInexactOrNonisolatedEvents(t *testing.T) {
	ctx := context.Background()
	_, ca := mappedChainFixture(t, false)
	_, cb := mappedChainFixture(t, true)
	for _, test := range []struct {
		knot    float64
		message string
	}{
		{29.0 / 256, "exact spline coefficient is not representable"}, // exact station, raw owner parameter 17/7
		{1.0 / 8, "exact rational crossing parameter not proved"},     // cubic has no rational root
	} {
		a, b := mappedChainCreasedSupport(t, false, test.knot, .25), mappedChainCreasedSupport(t, true, test.knot, .25)
		r, err := CertifyMappedSplineChainSupportKnotAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
		if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), test.message) || len(r.ModelCuts()) != 0 {
			t.Fatal("expected atomic rejection of an inexact station", test.knot, err)
		}
	}
	// This exact cubic follows the knot over its entire source interval.
	var control [4][3]*big.Rat
	for i := range control {
		control[i] = [3]*big.Rat{big.NewRat(1, 2), big.NewRat(int64(i), 4), big.NewRat(1, 1)}
	}
	q, err := NewExactBezierUV(control[:])
	if err != nil {
		t.Fatal(err)
	}
	plane := [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}
	root, err := mappedSplineSupportKnotRoot(ctx, q, plane, geom.SurfacePatchOptions{}, geom.ExactCurvePlaneEventOptions{})
	if root != nil || !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "must be isolated") {
		t.Fatal("a knot-following coedge has no isolated crossing", root, err)
	}
	// x = 1/2 + (t - 1/2)^2 touches the knot at one interior point.
	for i, x := range []*big.Rat{big.NewRat(3, 4), big.NewRat(5, 12), big.NewRat(5, 12), big.NewRat(3, 4)} {
		control[i][0] = x
	}
	q, err = NewExactBezierUV(control[:])
	if err != nil {
		t.Fatal(err)
	}
	root, err = mappedSplineSupportKnotRoot(ctx, q, plane, geom.SurfacePatchOptions{}, geom.ExactCurvePlaneEventOptions{})
	if root != nil || !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "must be unique and transverse") {
		t.Fatal("an interior tangency is not a crossing", root, err)
	}
}
