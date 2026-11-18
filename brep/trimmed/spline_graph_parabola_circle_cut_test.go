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

func TestNativeParabolaCircleGraphCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	const wallCap = 335081.0 / 27525120
	disk := 4317.0 * math.Pi / 134217728
	annulus := 368009.0/27525120 + disk
	for axis := range 2 {
		src := parabolaCircleGraphStock(t, axis, big.NewRat(3, 4), big.NewRat(1, 64), false)
		for order, volumes := range [2][]float64{{wallCap, disk}, {annulus}} {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				r, err := CutParabolaCircleSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				checkNativeParabolaCircleCutComponents(t, ctx, r, [2][]int{{0, 2}, {1}}[order], volumes)
				var encoded, reencoded bytes.Buffer
				if err := r.WriteJSON(ctx, &encoded); err != nil {
					t.Fatal(err)
				}
				reload, err := ReadParabolaCircleGraphCutJSON(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{}, opts)
				if err != nil || !reflect.DeepEqual(reload.History(), r.History()) {
					t.Fatal("changed Cut component ledger on source replay", err)
				}
				for i, s := range r.sources {
					if !geom.SameSplineSurface(s, reload.sources[i]) {
						t.Fatal("changed original source control net on replay", i)
					}
				}
				if err := reload.WriteJSON(ctx, &reencoded); err != nil || !bytes.Equal(encoded.Bytes(), reencoded.Bytes()) {
					t.Fatal("changed canonical Cut recipe", err)
				}
			})
		}
	}
}

func TestNativeLowerParabolaCircleGraphCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	src := parabolaCircleGraphStock(t, 1, big.NewRat(1, 8), big.NewRat(1, 256), false)
	disk := 5763.0 * math.Pi / 8589934592
	for order, volume := range [2][]float64{{121909.0/55050240 + disk}, {798593.0 / 11010048, disk}} {
		r, err := CutParabolaCircleSplineGraphs(ctx, src[0], src[1+order], src[2-order], 1, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		checkNativeParabolaCircleCutComponents(t, ctx, r, [2][]int{{0}, {1, 2}}[order], volume)
	}
}

func checkNativeParabolaCircleCutComponents(t *testing.T, ctx context.Context, cut *ParabolaCircleGraphCut, regions []int, volumes []float64) {
	t.Helper()
	parts, history := cut.Components(), cut.History()
	if len(parts) != len(volumes) || len(history) != len(volumes) {
		t.Fatal("wrong number of individually closed source components", len(parts))
	}
	for i, s := range parts {
		h := history[i]
		if h.Region != regions[i] || len(s.Topology().Shells) != 1 ||
			s.faces[h.RoofFaces[0]].Surface != cut.sources[1] || s.faces[h.RoofFaces[0]].Reversed ||
			s.faces[h.RoofFaces[1]].Surface != cut.sources[2] || !s.faces[h.RoofFaces[1]].Reversed {
			t.Fatal("lost source region or directed roofs", h, s.Topology())
		}
		m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || !(m.Volume > 0) || !(math.Abs(m.Volume-volumes[i]) <= 1e-9) {
			t.Fatal("component is not positively oriented or differs from the independent signed source integral", m, volumes[i], err)
		}
		t.Logf("region %d has %d faces, %d edges, %d vertices, signed volume %.16g", h.Region, len(s.faces), len(s.edges), len(s.vertices), m.Volume)
	}
}

func TestParabolaCircleGraphCutRejectsForgedLayer(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	src := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 64), false)
	r, err := CutParabolaCircleSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := r.WriteJSON(ctx, &encoded); err != nil {
		t.Fatal(err)
	}
	if part, err := ReadParabolaCircleGraphCutJSON(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{MaxEntities: 1}, opts); part != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("truncated budget published a Cut component", err)
	}
	var recipe parabolaCircleCutRecipe
	if err := json.Unmarshal(encoded.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger = recipe.Ledger[:1]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if part, err := ReadParabolaCircleGraphCutJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); part != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("omitted disk published the wall component", err)
	}
}
