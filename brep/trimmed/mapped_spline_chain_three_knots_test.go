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

const mappedChainFirstSeparateKnot = 107.0 / 2048
const mappedChainSharedMiddleKnot = 29.0 / 256
const mappedChainSecondSeparateKnot = 369.0 / 2048

func mappedChainThreeKnotFixture(t *testing.T) MappedSplineChainShell {
	t.Helper()
	ctx := context.Background()
	knots := [2][]float64{{mappedChainFirstSeparateKnot, mappedChainSharedMiddleKnot}, {mappedChainSharedMiddleKnot, mappedChainSecondSeparateKnot}}
	height := func(owner int, x, y float64) float64 {
		separate := [2]float64{mappedChainFirstSeparateKnot, mappedChainSecondSeparateKnot}[owner]
		return x*y + .125*max(0, x-mappedChainSharedMiddleKnot) + .125*(y-.25)*max(0, x-separate)
	}
	a := mappedChainFixtureKnotSupport(t, 0, knots[0], func(x, y float64) float64 { return height(0, x, y) })
	b := mappedChainFixtureKnotSupport(t, 1, knots[1], func(x, y float64) float64 { return height(1, x, y) })
	_, ca := mappedChainFixtureRawShape(t, false, 1365, true)
	_, cb := mappedChainFixtureRawShape(t, true, 455, true)
	proof, err := CertifyMappedSplineChainThreeSupportKnotsAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return mappedChainLevelTraceFixtureShell(t, proof, knots, height)
}

func TestMappedSplineChainThreeSupportKnotsRebuild(t *testing.T) {
	ctx := context.Background()
	source := mappedChainThreeKnotFixture(t)
	var raw, replay bytes.Buffer
	if err := source.WriteJSON(ctx, &raw, MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var record struct{ Boundary nativeMappedChainBoundary }
	if err := json.Unmarshal(raw.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Boundary.Version != 7 || len(record.Boundary.Owners[0].Stations) != 1 || len(record.Boundary.Owners[1].Stations) != 2 {
		t.Fatal("the v7 recipe must retain the original source partitions and rediscover all three cuts")
	}
	shell, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw.Bytes()), MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := shell.WriteJSON(ctx, &replay, MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
		t.Fatal("v7 source reopen must rederive the same exact original knot partition", err)
	}
	for _, version := range []string{"4", "5", "6"} {
		older := bytes.Replace(raw.Bytes(), []byte(`"Boundary":{"Version":7`), []byte(`"Boundary":{"Version":`+version), 1)
		if _, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(older), MappedChainShellNativeOptions{}); err == nil {
			t.Fatal("each prior repair mode must retain its own supported arrangement", version)
		}
	}
	limits := geom.SurfacePatchOptions{}
	out, err := shell.BuildRectangularSolid(ctx, 1e-9, limits)
	if err != nil {
		t.Fatal(err)
	}
	edges, vertices, faces := out.Solid.Edges(), out.Solid.Vertices(), out.Solid.Faces()
	if len(faces) != 7 || len(edges) != 24 || len(vertices) != 19 || len(out.ChainEdges) != 6 ||
		len(out.ChainStations) != 7 || shell.boundary.knotRepair != mappedChainThreeKnotsRepair {
		t.Fatal("all three additional shared vertices must appear on the native roof wire")
	}
	stations := [3]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 8), big.NewRat(3, 16)}
	wantRaw := [3][2]int64{{93, 443}, {197, 387}, {317, 283}}
	points := [3]geom.Vec3{
		{X: mappedChainFirstSeparateKnot, Y: .25, Z: 107.0 / 8192},
		{X: mappedChainSharedMiddleKnot, Y: .25, Z: 29.0 / 1024},
		{X: mappedChainSecondSeparateKnot, Y: .25, Z: 875.0 / 16384},
	}
	for i, station := range stations {
		a, b := edges[out.ChainEdges[i]], edges[out.ChainEdges[i+1]]
		if out.ChainStations[i+1].Cmp(station) != 0 || a.End != b.Start || vertices[a.End] != points[i] {
			t.Fatal("paired original restrictions must meet at the same exact native vertex", i)
		}
	}
	for owner, f := range faces[:2] {
		p, err := exactSplineSupportPatch(f.Surface, limits)
		if err != nil {
			t.Fatal(err)
		}
		if !geom.SameSplineSurface(f.Surface.(*geom.BSplineSurface), source.boundary.supports[owner].(*geom.BSplineSurface)) {
			t.Fatal("the original independently creased support chart must be kept", owner)
		}
		basis := source.boundary.sources[owner].parts[0].Source()
		var cells [4]*geom.SurfacePatch
		for i := range cells {
			curve := f.Outer[i].Curve.(MappedSplineUV)
			if !sameAuthoredBezierSpline(curve.Source().Curve, basis.Curve) || curve.Source().Reversed != basis.Reversed {
				t.Fatal("original owner basis or direction changed", owner, i)
			}
			for side := range 2 {
				crossing := i + side - 1
				if crossing < 0 || crossing >= len(stations) {
					continue
				}
				raw, err := curve.AuthoredParameter(big.NewRat(int64(side), 1))
				if err != nil || raw.Cmp(big.NewRat(wantRaw[crossing][owner], 1)) != 0 {
					t.Fatal("exact raw parameter lost on the corresponding native subedge", owner, crossing, err)
				}
			}
			d, err := curve.curve.descriptor(ctx, limits)
			if err != nil {
				t.Fatal(err)
			}
			cells[i], err = trimModelSupportCell(ctx, p, d, limits)
			if err != nil {
				t.Fatal("every raw subedge must lie in its original support cell", owner, i, err)
			}
		}
		changes := [2][3]bool{{true, true, false}, {false, true, true}}
		for i, point := range points {
			if (cells[i].Domain() != cells[i+1].Domain()) != changes[owner][i] {
				t.Fatal("wrong owner changes one original support cell at this station", owner, i)
			}
			x, y := ratInset(point.X), big.NewRat(1, 4)
			wantJump := new(big.Rat)
			if i == 1 {
				wantJump = big.NewRat(1, 8)
			} else if owner == 0 {
				y = big.NewRat(1, 2)
				if i == 0 {
					wantJump = big.NewRat(1, 32)
				}
			} else {
				y = big.NewRat(1, 8)
				if i == 2 {
					wantJump = big.NewRat(-1, 64)
				}
			}
			u, v := x, y
			if owner == 1 {
				u, v = y, x
			}
			var dz [2]*big.Rat
			for side, cell := range cells[i : i+2] {
				j, err := cell.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				dz[side] = [2]*big.Rat{j.Du[2], j.Dv[2]}[owner]
			}
			if new(big.Rat).Sub(dz[1], dz[0]).Cmp(wantJump) != 0 {
				t.Fatal("material creases and untouched companion branches must stay exact", owner, i)
			}
		}
	}
	report, err := out.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !report.Complete || len(report.Uses) != 48 {
		t.Fatal("complete continuous equality is required at every source coedge", err)
	}
	fill := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .04},
	}}
	display, err := shell.TessellateClosedCertified(ctx, nil, MappedChainClosedMeshOptions{
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{fill, fill},
	})
	if err != nil || !display.DisplayCollision.Complete || len(display.DisplayCollision.Contacts) != 0 {
		t.Fatal("closed display must use the same three original stations on both owners", err)
	}
	for i, station := range stations {
		found := false
		for _, sample := range display.ChainSamples {
			if sample.ModelParameter.Cmp(station) == 0 && display.Mesh.Vertices[sample.Vertex] == points[i] {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("closed display lost an exact shared station", i)
		}
	}
	t.Logf("%d display triangles retain exact shared vertices with no unintended contacts", len(display.Mesh.Triangles))
}

func TestMappedSplineChainThreeSupportKnotsRejectsUnsupportedArrangements(t *testing.T) {
	ctx := context.Background()
	r := mappedChainThreeKnotFixture(t).boundary
	check := func(b geom.Surface, cb MappedSplineChain, want error, message string) {
		t.Helper()
		bad, err := CertifyMappedSplineChainThreeSupportKnotsAgreement(ctx, r.supports[0], r.sources[0], b, cb, edgecheck.Options{})
		if !errors.Is(err, want) || !strings.Contains(err.Error(), message) || len(bad.ModelCuts()) != 0 {
			t.Fatal("invalid original crossing arrangement must fail atomically", message, err)
		}
	}
	_, narrow := mappedChainFixtureRawShape(t, true, 35, true)
	check(r.supports[1], narrow, geom.ErrUnrepresentable, "roof 1")
	const later = 99.0 / 256
	otherCell := mappedChainFixtureKnotSupport(t, 1, []float64{mappedChainSharedMiddleKnot, later}, func(x, y float64) float64 {
		return x*y + .125*max(0, x-mappedChainSharedMiddleKnot) + .125*(y-.25)*max(0, x-later)
	})
	check(otherCell, r.sources[1], geom.ErrConvergence, "span common chain cells")
	oneKnot := mappedChainFixtureKnotSupport(t, 1, []float64{mappedChainSharedMiddleKnot}, func(x, y float64) float64 {
		return x*y + .125*max(0, x-mappedChainSharedMiddleKnot)
	})
	check(oneKnot, r.sources[1], geom.ErrConvergence, "requires two original support knots")
}
