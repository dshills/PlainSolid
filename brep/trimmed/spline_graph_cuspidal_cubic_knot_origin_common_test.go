package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"slices"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeCuspidalCubicCommonAtLongitudinalKnot(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicStockWithLongitudinalKnot(t, .25)
	// The common longitudinal hinge adds the integral of
	// (3/16)*max(0,u-1/4) over the original 3 by 2 footprint.
	const added = 81.0 / 256
	const volume = 277485.0/22528 + added
	o := CuspidalCubicCommonOptions{}
	for order := range 2 {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			input := [3]*geom.BSplineSurface{src[0], src[1+order], src[2-order]}
			r, err := CommonCuspidalCubicSplineGraphs(ctx, input[0], input[1], input[2], order, 1e-9, o)
			if err != nil {
				t.Fatal(err)
			}
			checkCuspidalCubicKnotOriginShell(t, ctx, r, input, order)
			m, err := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil || !(math.Abs(m.Volume-volume) <= 1e-9) || m.Area <= 0 {
				t.Fatal("native mass differs from the independently integrated hinge", m, volume, err)
			}
			t.Logf("Common signed volume %.16g area %.16g", m.Volume, m.Area)

			var raw, echo bytes.Buffer
			if err := r.WriteJSON(ctx, &raw); err != nil {
				t.Fatal(err)
			}
			reloaded, err := ReadCuspidalCubicCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
			if err != nil || reloaded.History() != r.History() {
				t.Fatal("source replay changed the cusp ledger", err)
			}
			for i, source := range input {
				if !geom.SameSplineSurface(source, reloaded.sources[i]) {
					t.Fatal("source replay changed an original knot, control or chart", i)
				}
			}
			checkCuspidalCubicKnotOriginShell(t, ctx, reloaded, reloaded.sources, order)
			if err := reloaded.WriteJSON(ctx, &echo); err != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
				t.Fatal("native source replay changed canonical bytes", err)
			}
		})
	}
}

func checkCuspidalCubicKnotOriginShell(t *testing.T, ctx context.Context, r *CuspidalCubicCommon, sources [3]*geom.BSplineSurface, order int) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
		len(s.faces) != 9 || len(s.edges) != 20 || len(s.vertices) != 13 ||
		h.RoofOwners != ([2]int{1 - order, order}) || s.faces[h.Base].Surface != sources[0] {
		t.Fatal("the crease origin changed native signed topology", h, topo)
	}
	for i, face := range h.RoofFaces {
		if s.faces[face].Surface != sources[1+h.RoofOwners[i]] {
			t.Fatal("native shell substituted an original knotted roof", i)
		}
	}
	want := [3]geom.Vec3{
		{X: .75, Y: 1, Z: 527.0 / 256},
		{X: 3, Y: .25, Z: 599.0 / 256},
		{X: 3, Y: 1.75, Z: 617.0 / 256},
	}
	for i, id := range h.Vertices {
		if s.vertices[id] != want[i] {
			t.Fatal("native origin or exit moved", i, s.vertices[id], want[i])
		}
	}
	cusp := r.section.Vertices[0]
	if cusp.UV[order][0].RatString() != "1/4" || cusp.UV[1-order][1].Sign() != 0 {
		t.Fatal("the model cusp does not lie on both original source knots", cusp.UV)
	}
	for i, id := range h.Germs {
		e := s.edges[id]
		at := e.Range.Min
		if e.End == h.Vertices[0] {
			at = e.Range.Max
		} else if e.Start != h.Vertices[0] {
			t.Fatal("germ lost its original common cusp vertex", i)
		}
		d, err := e.Curve.Derivative(at)
		if err != nil || d != (geom.Vec3{}) || !slices.Equal(nodalCubicIncidentFaces(s, id), h.RoofFaces[:]) {
			t.Fatal("germ lost its zero first derivative or either original roof use", i, d, err)
		}
		g := r.section.Germs[i]
		third := [2][3]string{{"0", "-36", "-27/64"}, {"0", "36", "27/64"}}[i]
		for j, second := range [3]string{"18", "0", "81/32"} {
			if g.ModelSecond[j].RatString() != second || g.ModelThird[j].RatString() != third[j] {
				t.Fatal("native germ lost its original outgoing cell jet", i, g)
			}
		}
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete {
		t.Fatal("native original owner coedges no longer agree", err)
	}
	for _, use := range proof.Uses {
		if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("native coedge agreement requires an error allowance", use)
		}
	}
}
