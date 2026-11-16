package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"slices"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCuspidalCubicKnotOriginCommonExchange(t *testing.T) {
	ctx := context.Background()
	const name, units = "Cuspidal Common", .001
	const volume = 284613.0 / 22528 // Original Common plus 81/256 from the full stock hinge.
	so := step.CuspidalCubicSTEPOptions{}
	iopts := iges.CuspidalCubicIGESOptions{}
	for order := range 2 {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/native_cuspidal_cubic_knot_origin_common_order_%d.json", order))
			if err != nil {
				t.Fatal(err)
			}
			common, err := trimmed.ReadCuspidalCubicCommonJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.CuspidalCubicCommonOptions{})
			if err != nil {
				t.Fatal(err)
			}
			checkCuspidalKnotOriginNeutralBoundary(t, ctx, common.Boundary(), common.History().Vertices[0], common.History().Germs,
				[]geom.Vec3{{X: .75, Z: 131.0 / 64}, {X: .75, Y: 2, Z: 265.0 / 128}})
			var st, ig bytes.Buffer
			if err := step.WriteCuspidalCubicCommon(ctx, &st, common, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteCuspidalCubicCommon(ctx, &ig, common, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyCuspidalCubicCommonSTEP(ctx, bytes.NewReader(st.Bytes()), common, name, units, step.Limits{}, so); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyCuspidalCubicCommonIGES(ctx, bytes.NewReader(ig.Bytes()), common, units, iges.Limits{}, iopts); err != nil {
				t.Fatal(err)
			}
			checkCuspidalKnotReadback(t, ctx, common.Boundary(), common.History().Vertices[0],
				[3]int{9, 22, 15}, st.Bytes(), ig.Bytes(), volume,
				cuspidalKnotOriginDepartureJets(t, ctx))
			var refused bytes.Buffer
			if err := step.WriteCuspidalCubicCommon(ctx, &refused, common, name, units, step.Limits{MaxVertices: 14}, so); !errors.Is(err, geom.ErrConvergence) || refused.Len() != 0 {
				t.Fatal("limited STEP output published part of the knot-origin shell", err)
			}
		})
	}
}

func cuspidalKnotOriginDepartureJets(t *testing.T, ctx context.Context) func(string, string, []byte, geom.Vec3) {
	t.Helper()
	return func(format, ext string, data []byte, p geom.Vec3) {
		// The full outward germ has q=s/2 on [0,1].
		// Its outgoing dZ/dX is 9/64, and dZ/dY=3/256.
		second := [3]float64{4.5, 0, 81.0 / 128}
		third := [3]float64{0, 4.5, 27.0 / 512}
		checkCuspidalCubicExpectedJets(t, ctx, format, ext, data, p, second, third)
	}
}

func checkCuspidalKnotOriginNeutralBoundary(t *testing.T, ctx context.Context, s trimmed.Solid, cusp int, germs [2]int, stockPoints []geom.Vec3) {
	t.Helper()
	b, err := s.ExtractExactBezierBoundaryAtRationalKnots(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	native, edges := s.Vertices(), s.Edges()
	if len(b.Vertices) != len(native)+len(stockPoints) || !slices.Equal(b.Vertices[:len(native)], native) || len(b.Edges) != len(edges)+len(stockPoints) {
		t.Fatal("neutral extraction must keep the native cusp and insert the exact stock-side stations")
	}
	for _, want := range stockPoints {
		if !slices.Contains(b.Vertices[len(native):], want) {
			t.Fatal("an original stock-side knot joint was displaced", want)
		}
	}
	for _, id := range germs {
		nativeEdge, edge := edges[id], b.Edges[id]
		if edge.Start != nativeEdge.Start || edge.End != nativeEdge.End {
			t.Fatal("the original cusp germ lost its native endpoint incidence", id)
		}
		control := slices.Clone(edge.Model.Control)
		if edge.End == cusp {
			slices.Reverse(control)
		}
		degree := int64(len(control) - 1)
		if degree < 2 {
			t.Fatal("the cusp model lost its quadratic term", id)
		}
		for _, q := range control {
			if q[3].Cmp(control[0][3]) != 0 {
				t.Fatal("the original polynomial germ acquired a varying denominator", id)
			}
		}
		for axis, want := range [3]*big.Rat{big.NewRat(9, 2), new(big.Rat), big.NewRat(81, 128)} {
			if control[0][axis].Cmp(control[1][axis]) != 0 {
				t.Fatal("exact transfer model regularized the singular origin", id, axis)
			}
			d := new(big.Rat).Sub(control[2][axis], control[1][axis])
			d.Mul(d, big.NewRat(degree*(degree-1), 1)).Quo(d, control[0][3])
			if d.Cmp(want) != 0 {
				t.Fatal("exact model extraction used the opposite source-cell derivative", id, axis, d)
			}
		}
	}
}
