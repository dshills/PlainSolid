package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaTwoCirclesGraphEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	diskVolume := 5347170325.0 * math.Pi / 2251799813685248
	for axis := range 2 {
		src := parabolaTwoCirclesStock(t, axis, twoCirclePrimaryStock(), false)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := CombineParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, opts)
					if err != nil {
						t.Fatal(err)
					}
					h, s := r.History(), r.Boundary()
					if s.Topology().EulerCharacteristic != 2 || h.EnclosingRoof != 1 || len(s.faces[h.EnclosingRoof].Holes) != 2 {
						t.Fatal("wrong original roof-hole incidence", h, s.Topology())
					}
					for _, id := range h.WallFaces {
						wall := s.faces[id]
						baseUse, b := graphWallSourceTrim(wall.Outer[0].Curve, false, false)
						roofUse, r := graphWallSourceTrim(wall.Outer[2].Curve, true, false)
						if !b || !r || baseUse.surface != src[0] || (roofUse.surface != src[1] && roofUse.surface != src[2]) {
							t.Fatal("lost original wall coedge owner", id)
						}
					}
					if axis == 0 && order == 0 {
						volume := 88351926095467.0/44220983279616 - diskVolume
						if op == parametric.Union {
							volume = 88462602167477.0/44220983279616 + diskVolume
						}
						checkParabolaTwoCirclesSignedVolume(t, ctx, s, volume)
					}
					var raw, again bytes.Buffer
					if err := r.WriteJSON(ctx, &raw); err != nil {
						t.Fatal(err)
					}
					reload, err := ReadParabolaTwoCirclesGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
					if err != nil || reload.History() != h {
						t.Fatal("envelope replay altered region ancestry", err)
					}
					checkParabolaTwoCirclesReplaySources(t, r.sources, reload.sources)
					if err := reload.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
						t.Fatal("envelope replay changed canonical bytes", err)
					}
				})
			}
		}
	}
}

func TestNativeParabolaTwoCirclesGraphCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	// The rational strip and both radial moments are integrated independently
	// from the authored power factors, in the original unit XY coordinates.
	diskVolume := 5347170325.0 * math.Pi / 2251799813685248
	volumes := [2][]float64{
		{90040463765.0 / 44220983279616, 39745125.0 * math.Pi / 17592186044416, 259794325.0 * math.Pi / 2251799813685248},
		{20635608245.0/44220983279616 + diskVolume},
	}
	for axis := range 2 {
		src := parabolaTwoCirclesStock(t, axis, twoCirclePrimaryStock(), false)
		for order, want := range volumes {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				r, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				parts, history := r.Components(), r.History()
				if len(parts) != len(want) || len(history) != len(want) {
					t.Fatal("Cut dropped a positive original region or published an extra shell", history)
				}
				regions := [2][]int{{0, 2, 3}, {1}}[order]
				for i, s := range parts {
					h := history[i]
					topo := s.Topology()
					euler := 2
					if order == 1 {
						euler = -2
						if topo.InnerWires != 4 || len(h.CircleEdges) != 8 {
							t.Fatal("two through-hole loops were not kept in both roof faces", h, topo)
						}
					}
					if h.Region != regions[i] || len(topo.Shells) != 1 || topo.EulerCharacteristic != euler {
						t.Fatal("Cut did not retain the exact source component", h, topo)
					}
					if axis == 0 {
						checkParabolaTwoCirclesSignedVolume(t, ctx, s, want[i])
					}
				}
				var raw, again bytes.Buffer
				if err := r.WriteJSON(ctx, &raw); err != nil {
					t.Fatal(err)
				}
				reload, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, opts)
				if err != nil || !reflect.DeepEqual(reload.History(), history) {
					t.Fatal("Cut replay changed a region boundary", err)
				}
				checkParabolaTwoCirclesReplaySources(t, r.sources, reload.sources)
				if err := reload.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
					t.Fatal("Cut replay changed canonical bytes", err)
				}
			})
		}
	}
}

func TestNativeLowerParabolaTwoCirclesGraphRegions(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	src := parabolaTwoCirclesStock(t, 1, [2]twoCircleStockProbe{
		twoCircleProbe(4, 2, 1, 16), twoCircleProbe(24, 4, 1, 32),
	}, false)
	r, err := CombineParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1], src[2], 1, parametric.Intersection, 1e-9, opts)
	if err != nil || r.History().EnclosingRoof != 0 {
		t.Fatal("both lower circles must trim the lower roof region", err)
	}
	for order := range 2 {
		cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1+order], src[2-order], 1, 1e-9, opts)
		if err != nil {
			t.Fatal(err)
		}
		h := cut.History()
		if order == 0 && (len(h) != 1 || h[0].Region != 0 || len(h[0].CircleEdges) != 8) {
			t.Fatal("wrong lower layer through-hole ownership", h)
		}
		if order == 1 && (len(h) != 3 || h[0].Region != 1 || h[1].Region != 2 || h[2].Region != 3) {
			t.Fatal("wrong separate upper cap and disk ownership", h)
		}
	}
}

func checkParabolaTwoCirclesReplaySources(t *testing.T, original, replay [3]*geom.BSplineSurface) {
	t.Helper()
	for i, s := range original {
		if !geom.SameSplineSurface(s, replay[i]) {
			t.Fatal("changed original source support on replay", i)
		}
	}
}

func checkParabolaTwoCirclesSignedVolume(t *testing.T, ctx context.Context, s Solid, expected float64) {
	t.Helper()
	m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || !(m.Volume > 0) || !(math.Abs(m.Volume-expected) <= 1e-11) {
		t.Fatal("signed material differs from the independent source integral", m.Volume, expected, err)
	}
	t.Logf("signed volume %.16g on %d faces", m.Volume, len(s.faces))
}

func TestParabolaTwoCirclesGraphAtomicRefusals(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	src := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	r, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	if out, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, opts); !errors.Is(err, geom.ErrConvergence) || out != nil {
		t.Fatal("short entity limit leaked partial normalized Cut", err)
	}
	var ledger parabolaCircleCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &ledger); err != nil {
		t.Fatal(err)
	}
	ledger.Ledger[1], ledger.Ledger[2] = ledger.Ledger[2], ledger.Ledger[1]
	forged, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := ReadParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); !errors.Is(err, geom.ErrConvergence) || out != nil {
		t.Fatal("reordered lens receipts passed original-owner reproof", err)
	}
	// The second loop is geometrically qualified and has irrational radius.
	// Native assembly must not publish the representable first loop alone.
	discs := twoCirclePrimaryStock()
	discs[1].radiusSquared = big.NewRat(1, 128)
	inexact := parabolaTwoCirclesStock(t, 0, discs, false)
	p := parabolaTwoCirclesPatches(t, inexact[1], inexact[2])
	if section, err := geom.DiscoverSplineGraphParabolaTwoCircles(ctx, p[0], p[1], 0, opts.Proof); err != nil || section == nil {
		t.Fatal("irrational second radius must remain exact in the geometric proof", err)
	}
	if out, err := CutParabolaTwoCirclesSplineGraphs(ctx, inexact[0], inexact[1], inexact[2], 0, 1e-9, opts); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("unrepresentable second circle leaked an incomplete native Cut", err)
	}
	if out, err := CombineParabolaTwoCirclesSplineGraphs(ctx, inexact[0], inexact[1], inexact[2], 0, parametric.Intersection, 1e-9, opts); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("unrepresentable second circle leaked an incomplete native Common", err)
	}
	opts.Proof.MaxOperations = 1
	if out, err := CutParabolaTwoCirclesSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, opts); !errors.Is(err, geom.ErrConvergence) || out != nil {
		t.Fatal("proof budget leaked partial native Cut", err)
	}
}
