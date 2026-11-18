package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeCuspidalCubicFuseAndCutAtLongitudinalKnot(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	src := cuspidalCubicStockWithLongitudinalKnot(t, .25)
	// The common hinge adds 81/256 to the upper envelope. Its
	// contribution cancels from the signed height gap in each Cut.
	const fuseVolume = 286551.0 / 22528
	cutVolumes := [2]float64{1695.0 / 22528, 243.0 / 22528}
	o := CuspidalCubicCommonOptions{}
	for order := range 2 {
		input := [3]*geom.BSplineSurface{src[0], src[1+order], src[2-order]}
		t.Run(fmt.Sprintf("Fuse/%d", order), func(t *testing.T) {
			fuse, err := FuseCuspidalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, o)
			if err != nil {
				t.Fatal(err)
			}
			checkCuspidalCubicKnotOriginEnvelope(t, ctx, fuse.Boundary(), fuse.History(), fuse.section, input, order, order)
			checkKnotOriginSourceVolume(t, ctx, fuse.Boundary(), fuseVolume)

			var raw, echo bytes.Buffer
			if err := fuse.WriteJSON(ctx, &raw); err != nil {
				t.Fatal(err)
			}
			reloaded, err := ReadCuspidalCubicFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
			if err != nil || reloaded.History() != fuse.History() {
				t.Fatal("native Fuse replay changed the cusp ledger", err)
			}
			checkKnotOriginReplaySupports(t, input, reloaded.sources)
			checkCuspidalCubicKnotOriginEnvelope(t, ctx, reloaded.Boundary(), reloaded.History(), reloaded.section, reloaded.sources, order, order)
			if err := reloaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
				t.Fatal("native Fuse source replay changed canonical bytes", err)
			}
		})
		t.Run(fmt.Sprintf("Cut/%d", order), func(t *testing.T) {
			cut, err := CutCuspidalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, o)
			if err != nil {
				t.Fatal(err)
			}
			checkCuspidalCubicKnotOriginCut(t, ctx, cut, input, order)
			checkKnotOriginSourceVolume(t, ctx, cut.Boundary(), cutVolumes[order])

			var raw, echo bytes.Buffer
			if err := cut.WriteJSON(ctx, &raw); err != nil {
				t.Fatal(err)
			}
			reloaded, err := ReadCuspidalCubicCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
			if err != nil {
				t.Fatal("native Cut replay failed", err)
			}
			checkKnotOriginReplaySupports(t, input, reloaded.sources)
			checkCuspidalCubicKnotOriginCut(t, ctx, reloaded, reloaded.sources, order)
			if err := reloaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
				t.Fatal("native Cut source replay changed canonical bytes", err)
			}
		})
	}
}

func checkKnotOriginSourceVolume(t *testing.T, ctx context.Context, s Solid, want float64) {
	t.Helper()
	m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil || !(math.Abs(m.Volume-want) <= 1e-10) || !(m.Area > 0) {
		t.Fatal("native mass differs from the independently integrated original gap", m, want, err)
	}
	t.Logf("signed volume %.16g area %.16g", m.Volume, m.Area)
}

func checkKnotOriginReplaySupports(t *testing.T, a, b [3]*geom.BSplineSurface) {
	t.Helper()
	for i, original := range a {
		if !geom.SameSplineSurface(original, b[i]) {
			t.Fatal("source replay changed a knot, control or chart", i)
		}
	}
}

func checkCuspidalCubicKnotOriginCut(t *testing.T, ctx context.Context, cut *CuspidalCubicCut, sources [3]*geom.BSplineSurface, order int) {
	t.Helper()
	s, h := cut.Boundary(), cut.History()
	topo := s.Topology()
	counts := [2][3]int{{7, 16, 11}, {4, 7, 5}}[order]
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 || h.Region != order ||
		([3]int{len(s.faces), len(s.edges), len(s.vertices)}) != counts {
		t.Fatal("the source knot changed the signed single-component Cut topology", h, topo)
	}
	for owner, face := range h.RoofFaces {
		if s.faces[face].Surface != sources[1+owner] {
			t.Fatal("Cut substituted an original knotted roof", owner)
		}
	}
	for _, id := range h.WallFaces {
		var seen [2]bool
		for _, u := range s.faces[id].Outer {
			for owner := range seen {
				trim, ok := graphWallSourceTrim(u.Curve, owner == 0, true)
				if ok && trim.surface == sources[1+owner] {
					seen[owner] = true
				}
			}
		}
		if seen != ([2]bool{true, true}) {
			t.Fatal("Cut wall lost one of the original roof coedges", id)
		}
	}
	checkCuspidalCubicKnotOriginGerms(t, ctx, s, cut.section, h.Vertices, h.Germs, h.RoofFaces, order)
}
