package exchange_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestParabolaOffsetCirclesEnvelopeExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const name, unit = "Parabola and offset circles envelope", .001
	proof := edgecheck.Options{}
	diskVolume := 905961714175.0 * math.Pi / 1729382256910270464
	for _, c := range []struct {
		name        string
		axis, order int
		op          parametric.BooleanOp
		want        float64
	}{
		{"common", 0, 0, parametric.Intersection, 192847408364983.0/96482145337344 - diskVolume},
		{"fuse", 1, 1, parametric.Union, 193030413089753.0/96482145337344 + diskVolume},
	} {
		t.Run(c.name, func(t *testing.T) {
			stock := offsetCirclesExchangeStock(t, ctx, c.axis)
			r, err := trimmed.CombineParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1+c.order], stock[2-c.order], c.axis, c.op, 1e-9, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			h := r.History()
			checkOffsetCirclesExchangeConics(t, ctx, r.Boundary(), c.axis, h.ParabolaEdge, [2][]int{h.CircleEdges[0][:], h.CircleEdges[1][:]})
			var st, ig bytes.Buffer
			if err := step.WriteParabolaTwoCirclesGraphEnvelope(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaTwoCirclesGraphEnvelope(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaTwoCirclesGraphEnvelopeSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaTwoCirclesGraphEnvelopeIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			checkTwoCirclesExchangeReadback(t, ctx, []trimmed.Solid{r.Boundary()}, []float64{c.want}, st.Bytes(), ig.Bytes())
		})
	}
}

func TestParabolaOffsetCirclesCutExchange(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const name, unit = "Parabola and offset circles Cut", .001
	proof := edgecheck.Options{}
	disks := [2]float64{73873625.0 * math.Pi / 211106232532992, 100262992725.0 * math.Pi / 576460752303423488}
	for order, want := range [2][]float64{
		{116882309705.0 / 96482145337344, disks[0], disks[1]},
		{66122415065.0/96482145337344 + disks[0] + disks[1]},
	} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			axis := order
			stock := offsetCirclesExchangeStock(t, ctx, axis)
			r, err := trimmed.CutParabolaTwoCirclesSplineGraphs(ctx, stock[0], stock[1+order], stock[2-order], axis, 1e-9, trimmed.ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
			parts, owners := r.Components(), r.History()
			for i, h := range owners {
				var rings [2][]int
				if h.Region >= 2 {
					rings[h.Region-2] = h.CircleEdges
				} else if len(h.CircleEdges) == 8 {
					rings = [2][]int{h.CircleEdges[:4], h.CircleEdges[4:]}
				}
				checkOffsetCirclesExchangeConics(t, ctx, parts[i], axis, h.ParabolaEdge, rings)
			}
			var st, ig bytes.Buffer
			if err := step.WriteParabolaTwoCirclesGraphCut(ctx, &st, r, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.WriteParabolaTwoCirclesGraphCut(ctx, &ig, r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := step.VerifyParabolaTwoCirclesGraphCutSTEP(ctx, bytes.NewReader(st.Bytes()), r, name, unit, step.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyParabolaTwoCirclesGraphCutIGES(ctx, bytes.NewReader(ig.Bytes()), r, unit, iges.Limits{}, proof); err != nil {
				t.Fatal(err)
			}
			checkTwoCirclesExchangeReadback(t, ctx, parts, want, st.Bytes(), ig.Bytes())
		})
	}
}

func offsetCirclesExchangeStock(t *testing.T, ctx context.Context, axis int) [3]*geom.BSplineSurface {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("testdata/native_parabola_offset_circles_axis_%d.json", axis))
	if err != nil {
		t.Fatal(err)
	}
	r, err := trimmed.ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(raw), trimmed.JSONLimits{}, trimmed.ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h := r.History()
	faces := r.Boundary().Faces()
	stock := [3]*geom.BSplineSurface{faces[h.Base].Surface.(*geom.BSplineSurface)}
	for region, owner := range h.RoofOwners {
		stock[1+owner] = faces[h.RoofFaces[region]].Surface.(*geom.BSplineSurface)
	}
	return stock
}

func checkOffsetCirclesExchangeConics(t *testing.T, ctx context.Context, body trimmed.Solid, axis, parabola int, rings [2][]int) {
	t.Helper()
	boundary, err := body.ExtractExactBezierBoundary(ctx, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	contact := make(map[int]int)
	if parabola >= 0 {
		contact[parabola] = -1
	}
	for circle, edges := range rings {
		for _, id := range edges {
			contact[id] = circle
		}
	}
	for id, circle := range contact {
		checkOffsetExchangeConicLaw(t, boundary.Edges[id].Model, axis, circle, 2)
		owners := 0
		for _, f := range boundary.Faces {
			for _, loop := range f.Loops {
				for _, use := range loop {
					if use.Edge == id {
						checkOffsetExchangeConicLaw(t, use.UV, axis, circle, 0)
						owners++
					}
				}
			}
		}
		if owners != 2 {
			t.Fatal("expected both exact original pcurves", id, owners)
		}
	}
}

func checkOffsetExchangeConicLaw(t *testing.T, span geom.ExactBezierSpan, axis, circle int, z int64) {
	t.Helper()
	if len(span.Control) != 3 || span.Start != 0 || span.End != 1 {
		t.Fatal("lost exact quadratic and normalized edge clock", span)
	}
	circles := [2][3]*big.Rat{
		{big.NewRat(3, 8), big.NewRat(9, 16), big.NewRat(1, 64)},
		{big.NewRat(17, 32), big.NewRat(25, 32), big.NewRat(9, 1024)},
	}
	// The substituted homogeneous conic has degree at most four.
	// Five distinct rational parameters certify the identity everywhere.
	for i := range int64(5) {
		basis := [3]int64{(4 - i) * (4 - i), 2 * i * (4 - i), i * i}
		var h [4]big.Rat
		for coordinate := range h {
			for j, b := range basis {
				h[coordinate].Add(&h[coordinate], new(big.Rat).Mul(span.Control[j][coordinate], big.NewRat(b, 1)))
			}
		}
		x := new(big.Rat).Quo(&h[axis], &h[3])
		y := new(big.Rat).Quo(&h[1-axis], &h[3])
		if new(big.Rat).Quo(&h[2], &h[3]).Cmp(big.NewRat(z, 1)) != 0 {
			t.Fatal("model curve or pcurve left its exact plane", i)
		}
		if circle < 0 {
			x.Mul(x, x).Quo(x, big.NewRat(8, 1)).Add(x, big.NewRat(1, 4))
			if x.Cmp(y) != 0 {
				t.Fatal("open parabola changed its original polynomial", i)
			}
		} else {
			c := circles[circle]
			x.Sub(x, c[0])
			y.Sub(y, c[1])
			x.Mul(x, x).Add(x, y.Mul(y, y))
			if x.Cmp(c[2]) != 0 {
				t.Fatal("offset arc changed its original implicit circle", circle, i)
			}
		}
	}
}
